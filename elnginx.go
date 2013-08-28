package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/ec2"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"os/exec"
)

var AWSAuth = aws.Auth{}
var AWSRegion = ""
var Region = aws.Region{}
var TopicArn = ""
var UpstreamName = ""
var UpstreamFile = ""
var UpstreamsPath = ""

type Message struct {
	Event      string
	InstanceId string `json:"EC2InstanceId"`
}

type JSONResponse struct {
	TopicArn string
	Message  string
}

func getUpstreamFilenameForInstance(i *ec2.Instance) string {
	return filepath.Join(UpstreamsPath, i.InstanceId+".upstream")
}

func addInstance(i *ec2.Instance) error {
	filename := getUpstreamFilenameForInstance(i)

	upstream := fmt.Sprintf("server %s:80 max_fails=3 fail_timeout=60s;\n", i.PrivateDNSName)
	buf := []byte(upstream)

	if err := ioutil.WriteFile(filename, buf, 0640); err != nil {
		return err
	}

	return nil
}

func rmInstance(i *ec2.Instance) error {
	filename := getUpstreamFilenameForInstance(i)

	if _, err := os.Open(filename); os.IsNotExist(err) {
		return fmt.Errorf("Instance \"%s\" not found in config.", i.InstanceId)
	}

	if err := os.Remove(filename); err != nil {
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

func reconfigure() error {
	upstream, err := os.Create(UpstreamFile)
	if err != nil {
		return err
	}
	defer upstream.Close()

	upstream.WriteString(fmt.Sprintf("upstream %s {\n", UpstreamName))

	upstream_filenames, err := filepath.Glob(filepath.Join(UpstreamsPath, "*.upstream"))
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
	input, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	log.Println(fmt.Sprintf("Received Payload: %s", input))

	response := JSONResponse{}
	if err := json.Unmarshal(input, &response); err != nil {
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	// Check TopicArn
	if response.TopicArn != TopicArn {
		http.Error(w, fmt.Sprintf("No handler for the specified ARN (\"%s\") found.", response.TopicArn), http.StatusNotFound)
		return
	}

	// Load message
	message := Message{}
	if err := json.Unmarshal([]byte(response.Message), &message); err != nil {
		http.Error(w, "Invalid Message field JSON.", http.StatusBadRequest)
		return
	}

	// Get EC2 Instance
	instance, err := getInstance(message.InstanceId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response_content := ""
	switch message.Event {
	case "autoscaling:EC2_INSTANCE_LAUNCH":
		err := addInstance(&instance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response_content = fmt.Sprintf(`Added instance "%s".`, instance.InstanceId)

	case "autoscaling:EC2_INSTANCE_TERMINATE":
		err := rmInstance(&instance)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		response_content = fmt.Sprintf(`Removed instance "%s".`, instance.InstanceId)

	default:
		http.Error(w, "Invalid Event.", http.StatusBadRequest)
		return
	}

	if err := reconfigure(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := reload(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println(response_content)
	fmt.Fprintf(w, response_content)
}

func main() {
	var err error
	AWSAuth, err = aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&AWSRegion, "aws-region", "us-east-1", "AWS Region of choice.")
	Region = aws.Region{
		Name:        AWSRegion,
		EC2Endpoint: fmt.Sprintf("https://ec2.%s.amazonaws.com", AWSRegion),
		SNSEndpoint: fmt.Sprintf("https://sns.%s.amazonaws.com", AWSRegion),
	}

	flag.StringVar(&TopicArn, "topic-arn", "", "Topic ARN to be monitored.")

	flag.StringVar(&UpstreamName, "upstream", "backends", "Upstream name to be generated.")

	flag.StringVar(&UpstreamFile, "upstream-file", "/etc/nginx/conf.d/upstreams/backends.upstreams",
		"Name of the file that holds the upstream block.")

	flag.StringVar(&UpstreamsPath, "upstreams-path", "/etc/nginx/conf.d/upstreams/backends",
		"Folder where will be generated servers confs.")

	flag.Parse()

	if TopicArn == "" {
		log.Fatal("No Topic ARN found.")
	}

	http.HandleFunc("/", readMessage)

	log.Println("Listening on :5000")
	log.Println("Monitoring events on:", TopicArn)
	log.Println("Upstream:", UpstreamName)
	log.Println("  File:", UpstreamFile)
	log.Println("  Path:", UpstreamsPath)
	err = http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
