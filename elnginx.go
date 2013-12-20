package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/rochacon/elastic-nginx/config"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/ec2"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const VERSION = "0.4"

var AWSAuth = aws.Auth{}
var AWSRegion = ""
var ConfigPath = ""
var Config = &config.Config{}
var Region = aws.Region{}

type Message struct {
	Event               string
	InstanceId          string `json:"EC2InstanceId"`
	AutoScalingGroupARN string
}

type JSONInput struct {
	Type         string
	TopicArn     string
	Message      string
	SubscribeURL string
}

func getUpstreamFilenameForInstance(u config.Upstream, i *ec2.Instance) string {
	return filepath.Join(u.ContainerFolder, i.InstanceId+".upstream")
}

func addInstance(u config.Upstream, i *ec2.Instance) error {
	filename := getUpstreamFilenameForInstance(u, i)

	u.Lock()
	defer u.Unlock()

	upstream := fmt.Sprintf("server %s:80 max_fails=3 fail_timeout=60s;\n", i.PrivateDNSName)
	buf := []byte(upstream)

	if err := ioutil.WriteFile(filename, buf, 0640); err != nil {
		return err
	}

	return nil
}

func rmInstance(u config.Upstream, i *ec2.Instance) error {
	filename := getUpstreamFilenameForInstance(u, i)

	err := os.Remove(filename)

	if err != nil && os.IsNotExist(err) {
		return fmt.Errorf("Instance \"%s\" not found in config.", i.InstanceId)
	}

	if err != nil {
		return err
	}

	return nil
}

func getInstance(id string) (ec2.Instance, error) {
	ec2conn := ec2.New(AWSAuth, Region)

	ec2resp, err := ec2conn.Instances([]string{id}, nil)
	if err != nil {
		return ec2.Instance{}, err
	}

	for _, r := range ec2resp.Reservations {
		for _, i := range r.Instances {
			return i, nil
		}
	}
	return ec2.Instance{}, fmt.Errorf("WTFBBQ?!")
}

func reconfigure(u config.Upstream) error {
	u.Lock()
	defer u.Unlock()

	upstream, err := os.Create(u.File)
	if err != nil {
		return err
	}
	defer upstream.Close()

	upstream.WriteString(fmt.Sprintf("upstream %s {\n", u.Name))

	upstream_filenames, err := filepath.Glob(filepath.Join(u.ContainerFolder, "*.upstream"))
	if err != nil {
		return err
	}

	for _, upstream_filename := range upstream_filenames {
		content, err := ioutil.ReadFile(upstream_filename)
		if err != nil {
			return err
		}
		upstream.WriteString(fmt.Sprintf("  %s", string(content)))
	}

	upstream.WriteString("}\n")
	return nil
}

func reload() ([]byte, error) {
	return exec.Command("sudo", "service", "nginx", "reload").CombinedOutput()
}

func readMessage(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	log.Println(fmt.Sprintf("Received Payload: %s", body))

	data := JSONInput{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Println("Invalid JSON.")
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	if data.TopicArn != Config.TopicArn {
		output := fmt.Sprintf("No handler for the specified ARN (\"%s\") found.", data.TopicArn)
		log.Println(output)
		http.Error(w, output, http.StatusNotFound)
		return
	}

	if data.Type == "SubscriptionConfirmation" {
		if Config.AutoSubscribe {
			go http.Get(data.SubscribeURL)
			w.WriteHeader(http.StatusAccepted)
			output := fmt.Sprintf(`Subscribed to "%s".`, data.TopicArn)
			log.Println(output)
			fmt.Fprintf(w, output)
		}
		return
	}

	// Load message
	message := Message{}
	if err := json.Unmarshal([]byte(data.Message), &message); err != nil {
		log.Println("Invalid Message field JSON.")
		http.Error(w, "Invalid Message field JSON.", http.StatusBadRequest)
		return
	}

	for _, u := range Config.Upstreams {
		if message.AutoScalingGroupARN == u.AutoScalingGroupARN {
			switch message.Event {
			case "autoscaling:EC2_INSTANCE_LAUNCH":
				launch(w, u, message.InstanceId)
			case "autoscaling:EC2_INSTANCE_TERMINATE":
				terminate(w, u, message.InstanceId)
			default:
				log.Println("Invaild Event.")
				http.Error(w, "Invalid Event.", http.StatusBadRequest)
				return
			}
			return
		}
	}

	output := fmt.Sprintf(`Invalid Auto Scaling Group ARN "%s".`, message.AutoScalingGroupARN)
	log.Println(output)
	http.Error(w, output, http.StatusBadRequest)
	return
}

func launch(w http.ResponseWriter, u config.Upstream, instanceId string) {
	// Get EC2 Instance
	instance, err := getInstance(instanceId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = addInstance(u, &instance)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := reconfigure(u); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := reload(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := fmt.Sprintf(`Added instance "%s".`, instance.InstanceId)
	log.Println(output)
	fmt.Fprintf(w, output)
}

func terminate(w http.ResponseWriter, u config.Upstream, instanceId string) {
	// Get EC2 Instance
	instance, err := getInstance(instanceId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = rmInstance(u, &instance)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := reconfigure(u); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := reload(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := fmt.Sprintf(`Removed instance "%s".`, instance.InstanceId)
	log.Println(output)
	fmt.Fprintf(w, output)
}

func init() {
	flag.StringVar(&AWSRegion, "aws-region", "us-east-1", "AWS Region of choice.")
	flag.StringVar(&ConfigPath, "config", "/etc/elastic-nginx.json", "Elastic NGINX config file.")
}

func main() {
	listen := flag.String("listen", "127.0.0.1:5000", "Address to listen to.")
	show_version := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	Region = aws.Region{
		Name:        AWSRegion,
		EC2Endpoint: fmt.Sprintf("https://ec2.%s.amazonaws.com", AWSRegion),
		SNSEndpoint: fmt.Sprintf("https://sns.%s.amazonaws.com", AWSRegion),
	}

	if *show_version {
		fmt.Println("elastic-nginx version", VERSION)
		return
	}

	var err error
	AWSAuth, err = aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}

	Config, err = config.ReadFile(ConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", readMessage)

	log.Println("Listening on", *listen)
	log.Println("Monitoring events on topic:", Config.TopicArn)
	log.Println("AWS Region:", AWSRegion)
	for i, u := range Config.Upstreams {
		log.Printf("Upstream %d: %s", i, u.Name)
		log.Println("  ContainerFolder:", u.ContainerFolder)
		log.Println("  File:", u.File)
	}
	err = http.ListenAndServe(*listen, nil)
	if err != nil {
		log.Fatal(err)
	}
}
