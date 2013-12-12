package main

import (
	"bytes"
	"fmt"
	"github.com/globocom/commandmocker"
	"github.com/rochacon/elastic-nginx/config"
	"io"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/ec2"
	"launchpad.net/goamz/ec2/ec2test"
	"launchpad.net/gocheck"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

type S struct {
	ec2          *ec2.EC2
	logOutput    *bytes.Buffer
	instance_ids []string
	testPath     string
	testServer   *ec2test.Server
}

var _ = gocheck.Suite(&S{})

func (s *S) SetUpSuite(c *gocheck.C) {
	var err error

	s.testServer, err = ec2test.NewServer()
	if err != nil {
		panic(err)
	}

	s.instance_ids = s.testServer.NewInstances(1, "t1.micro", "ami-00000", ec2test.Running, nil)

	s.testPath = c.MkDir()

	AWSRegion = "test"
	Region = aws.Region{EC2Endpoint: s.testServer.URL()}
	Config = &config.Config{
		TopicArn:      "arn:test",
		AutoSubscribe: false,
		Upstreams: []config.Upstream{
			config.Upstream{
				AutoScalingGroupARN: "arn:asg-test",
				ContainerFolder:     path.Join(s.testPath, "testupstreamcontainer"),
				File:                path.Join(s.testPath, "testupstreamfile"),
				Name:                "test",
			},
		},
	}
}

func (s *S) SetUpTest(c *gocheck.C) {
	s.logOutput = &bytes.Buffer{}
	log.SetOutput(s.logOutput)
	os.MkdirAll(Config.Upstreams[0].ContainerFolder, 0755)
}

func (s *S) TearDownTest(c *gocheck.C) {
	os.RemoveAll(s.testPath)
}

func (s *S) TearDownSuite(c *gocheck.C) {
	s.testServer.Quit()
}

func newRequest(method, url string, b io.Reader, c *gocheck.C) (*httptest.ResponseRecorder, *http.Request) {
	request, err := http.NewRequest(method, url, b)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	return recorder, request
}

func readBody(b io.Reader, c *gocheck.C) string {
	body, err := ioutil.ReadAll(b)
	c.Assert(err, gocheck.IsNil)
	return string(body)
}

func (s *S) TestReadMessageWithLaunchJSON(c *gocheck.C) {
	cmd, err := commandmocker.Add("sudo", "service nginx reload")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(cmd)

	payload := `{"TopicArn":"arn:test","Message":` +
		`"{\"AutoScalingGroupARN\":\"arn:asg-test\",\"Event\":\"autoscaling:EC2_INSTANCE_LAUNCH\",` +
		`\"EC2InstanceId\":\"%s\"}"}`
	b := strings.NewReader(fmt.Sprintf(payload, s.instance_ids[0]))
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, fmt.Sprintf(`Added instance "%s".`, s.instance_ids[0]))
	c.Assert(recorder.Code, gocheck.Equals, 200)

	// Check upstreams file
	upstream := Config.Upstreams[0]
	content, err := ioutil.ReadFile(upstream.File)
	c.Assert(err, gocheck.IsNil)
	serverLine := fmt.Sprintf("server %s.internal.invalid:80 max_fails=3 fail_timeout=60s;\n", s.instance_ids[0])
	c.Assert(string(content), gocheck.Equals, fmt.Sprintf("upstream %s {\n  %s}\n", upstream.Name, serverLine))

	// Check run NGINX reload
	c.Assert(commandmocker.Ran(cmd), gocheck.Equals, true)
}

func (s *S) TestAddInstance(c *gocheck.C) {
	u := Config.Upstreams[0]
	i := &ec2.Instance{InstanceId: "i-00000", PrivateDNSName: "test.internal"}
	err := addInstance(u, i)
	c.Assert(err, gocheck.IsNil)

	content, err := ioutil.ReadFile(getUpstreamFilenameForInstance(u, i))
	c.Assert(err, gocheck.IsNil)
	c.Assert(string(content), gocheck.Equals, "server test.internal:80 max_fails=3 fail_timeout=60s;\n")
}

func (s *S) TestReadMessageWithTerminateJSON(c *gocheck.C) {
	cmd, err := commandmocker.Add("sudo", "service nginx reload")
	c.Assert(err, gocheck.IsNil)
	defer commandmocker.Remove(cmd)

	// Setup instance file
	u := Config.Upstreams[0]
	instance := &ec2.Instance{InstanceId: s.instance_ids[0], PrivateDNSName: "test.internal"}
	if err := addInstance(u, instance); err != nil {
		c.Error(err)
	}

	payload := `{"TopicArn":"arn:test","Message":
	"{\"AutoScalingGroupARN\":\"arn:asg-test\",\"Event\":\"autoscaling:EC2_INSTANCE_TERMINATE\",\"EC2InstanceId\":\"%s\"}"}`
	b := strings.NewReader(fmt.Sprintf(payload, s.instance_ids[0]))
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, fmt.Sprintf(`Removed instance "%s".`, s.instance_ids[0]))
	c.Assert(recorder.Code, gocheck.Equals, 200)

	// Check upstreams file
	content, err := ioutil.ReadFile(u.File)
	c.Assert(err, gocheck.IsNil)
	c.Assert(string(content), gocheck.Equals, fmt.Sprintf("upstream %s {\n}\n", u.Name))

	// Check run NGINX reload
	c.Assert(commandmocker.Ran(cmd), gocheck.Equals, true)
}

func (s *S) TestRemoveInstance(c *gocheck.C) {
	u := Config.Upstreams[0]

	// Setup test instance
	i := &ec2.Instance{InstanceId: "i-00000", PrivateDNSName: "test.internal"}
	err := addInstance(u, i)
	c.Assert(err, gocheck.IsNil)

	// Remove instance
	err = rmInstance(u, i)
	c.Assert(err, gocheck.IsNil)

	_, err = os.Stat(getUpstreamFilenameForInstance(u, i))
	c.Assert(os.IsNotExist(err), gocheck.Equals, true)
}

func (s *S) TestRemoveInstanceWithoutConfigFile(c *gocheck.C) {
	u := Config.Upstreams[0]
	i := &ec2.Instance{InstanceId: "i-00000", PrivateDNSName: "test.internal"}

	// Remove instance
	err := rmInstance(u, i)
	c.Assert(err, gocheck.ErrorMatches, "Instance \"i-00000\" not found in config.")
}

func (s *S) TestReadMessageWithInvalidJSON(c *gocheck.C) {
	b := strings.NewReader("")
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, "Invalid JSON.\n")
	c.Assert(recorder.Code, gocheck.Equals, 400)
}

func (s *S) TestReadMessageWithInvalidMessageJSON(c *gocheck.C) {
	b := strings.NewReader(`{"TopicArn":"arn:test", "Message":""}`)
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, "Invalid Message field JSON.\n")
	c.Assert(recorder.Code, gocheck.Equals, 400)
}

func (s *S) TestReadMessageFromInvalidTopicArn(c *gocheck.C) {
	b := strings.NewReader(`{"TopicArn":"invalid","Message":"{}"}`)
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, "No handler for the specified ARN (\"invalid\") found.\n")
	c.Assert(recorder.Code, gocheck.Equals, 404)
}

func (s *S) TestReadMessageFromInvalidAutoScalingGroupName(c *gocheck.C) {
	b := strings.NewReader(`{"TopicArn":"arn:test","Message":"{\"AutoScalingGroupARN\":\"arn:asg-invalid\"}"}`)
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, "Invalid Auto Scaling Group ARN \"arn:asg-invalid\".\n")
	c.Assert(recorder.Code, gocheck.Equals, 400)
}

func (s *S) TestGetInstance(c *gocheck.C) {
	i, err := getInstance(s.instance_ids[0])
	c.Assert(err, gocheck.IsNil)
	c.Assert(i.InstanceId, gocheck.Equals, s.instance_ids[0])
}

func (s *S) TestGetUpstreamFilenameForInstance(c *gocheck.C) {
	u := Config.Upstreams[0]
	i := &ec2.Instance{InstanceId: "i-00000"}
	path := getUpstreamFilenameForInstance(u, i)
	c.Assert(path, gocheck.Equals, filepath.Join(u.ContainerFolder, "i-00000.upstream"))
}

func (s *S) TestAutoSubscribe(c *gocheck.C) {
	Config.AutoSubscribe = true

	subscriptionUrlCalled := make(chan bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subscriptionUrlCalled <- true
	}))
	defer server.Close()

	b := strings.NewReader(fmt.Sprintf(`{"TopicArn":"arn:test","Type":"SubscriptionConfirmation","SubscribeURL":"%s"}`, server.URL))

	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	c.Assert(<-subscriptionUrlCalled, gocheck.Equals, true)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, `Subscribed to "arn:test".`)
	c.Assert(recorder.Code, gocheck.Equals, 202)
}

func (s *S) TestAutoSubscribeOff(c *gocheck.C) {
	Config.AutoSubscribe = false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("This should not be called")
	}))
	defer server.Close()

	b := strings.NewReader(fmt.Sprintf(`{"TopicArn":"arn:test","Type":"SubscriptionConfirmation","SubscribeURL":"%s"}`, server.URL))

	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, "")
	c.Assert(recorder.Code, gocheck.Equals, 200)
}
