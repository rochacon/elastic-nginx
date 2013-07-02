package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/ec2"
	"io/ioutil"
	"net/http"
	"os"
)

var AWSRegion = ""
var Region = aws.Region{}
var TopicArn = ""
var UpstreamName = ""
var UpstreamFile = ""
var UpstreamsPath = ""

type Message struct {
	Event string
	InstanceId string `json:"EC2InstanceId"`
}

type JSONResponse struct {
	TopicArn string
	Message string
}

func getUpstreamFilenameForInstance(i *ec2.Instance) string {
	return UpstreamsPath + "/" + i.InstanceId + ".upstream"
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
	auth, err := aws.EnvAuth()
	if err != nil {
		return ec2.Instance{}, err
	}

	ec2conn := ec2.New(auth, Region)

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

func readMessage(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

	response := JSONResponse{}
	if err := json.Unmarshal(input, &response); err != nil {
		http.Error(w, "Invalid JSON.", http.StatusBadRequest)
		return
	}

	// // Check TopicArn
	if response.TopicArn != TopicArn { // Use configurable ARN
		http.Error(w, fmt.Sprintf("No handler for the specified ARN (\"%s\") found.", TopicArn), http.StatusNotFound)
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

	switch message.Event {
		case "autoscaling:EC2_INSTANCE_LAUNCH":
			err := addInstance(&instance)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, `Added instance "%s".`, instance.InstanceId)

		case "autoscaling:EC2_INSTANCE_TERMINATE":
			err := rmInstance(&instance)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, `Removed instance "%s".`, instance.InstanceId)
	}
}

func main() {
	flag.StringVar(&AWSRegion, "aws-region", "us-east-1", "AWS Region of choice.")
	Region = aws.Region{
		Name: AWSRegion,
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
	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
