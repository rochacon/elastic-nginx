package main

import (
	"bytes"
	"fmt"
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
	"strings"
	"testing"
)

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

type S struct {
	ec2 *ec2.EC2
	logOutput *bytes.Buffer
	instance_ids []string
	testPath string
	testServer *ec2test.Server
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
	TopicArn = "arn:test"
	UpstreamName = "testupstream"
	UpstreamFile = path.Join(s.testPath, "testupstreamfile")
	UpstreamsPath = path.Join(s.testPath, "testupstreampath")
}

func (s *S) SetUpTest(c *gocheck.C) {
	s.logOutput = &bytes.Buffer{}
	log.SetOutput(s.logOutput)
	os.MkdirAll(UpstreamsPath, 0755)
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
	b := strings.NewReader(fmt.Sprintf(`{"TopicArn":"arn:test","Message":"{\"Event\":\"autoscaling:EC2_INSTANCE_LAUNCH\",\"EC2InstanceId\":\"%s\"}"}`, s.instance_ids[0]))
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, fmt.Sprintf(`Added instance "%s".`, s.instance_ids[0]))
	c.Assert(recorder.Code, gocheck.Equals, 200)

	// Check upstreams file
	content, err := ioutil.ReadFile(UpstreamFile)
	c.Assert(err, gocheck.IsNil)
	serverLine := "server :80 max_fails=3 fail_timeout=60s;\n"  // ec2test.Instance does not have a PrivateDNSName :'(
	c.Assert(string(content), gocheck.Equals, fmt.Sprintf("upstream %s {\n  %s}\n", UpstreamName, serverLine))
}

func (s *S) TestAddInstance(c *gocheck.C) {
	i := &ec2.Instance{InstanceId: "i-00000", PrivateDNSName: "test.internal"}
	err := addInstance(i)
	c.Assert(err, gocheck.IsNil)

	content, err := ioutil.ReadFile(getUpstreamFilenameForInstance(i))
	c.Assert(err, gocheck.IsNil)
	c.Assert(string(content), gocheck.Equals, "server test.internal:80 max_fails=3 fail_timeout=60s;\n")
}

func (s *S) TestReadMessageWithTerminateJSON(c *gocheck.C) {
	// Setup instance file
	instance := &ec2.Instance{InstanceId: s.instance_ids[0], PrivateDNSName: "test.internal"}
	if err := addInstance(instance); err != nil {
		c.Error(err)
	}

	b := strings.NewReader(fmt.Sprintf(`{"TopicArn":"arn:test","Message":"{\"Event\":\"autoscaling:EC2_INSTANCE_TERMINATE\",\"EC2InstanceId\":\"%s\"}"}`, s.instance_ids[0]))
	recorder, request := newRequest("POST", "/", b, c)
	readMessage(recorder, request)
	body := readBody(recorder.Body, c)
	c.Assert(body, gocheck.Equals, fmt.Sprintf(`Removed instance "%s".`, s.instance_ids[0]))
	c.Assert(recorder.Code, gocheck.Equals, 200)

	// Check upstreams file
	content, err := ioutil.ReadFile(UpstreamFile)
	c.Assert(err, gocheck.IsNil)
	c.Assert(string(content), gocheck.Equals, fmt.Sprintf("upstream %s {\n}\n", UpstreamName))

	// TODO test logging ??
}

func (s *S) TestRemoveInstance(c *gocheck.C) {
	// Setup test instance
	i := &ec2.Instance{InstanceId: "i-00000", PrivateDNSName: "test.internal"}
	err := addInstance(i)
	c.Assert(err, gocheck.IsNil)

	// Remove instance
	err = rmInstance(i)
	c.Assert(err, gocheck.IsNil)

	_, err = ioutil.ReadFile(getUpstreamFilenameForInstance(i))
	c.Assert(os.IsNotExist(err), gocheck.Equals, true)
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

func (s *S) TestGetInstance(c *gocheck.C) {
	i, err := getInstance(s.instance_ids[0])
	c.Assert(err, gocheck.IsNil)
	c.Assert(i.InstanceId, gocheck.Equals, s.instance_ids[0])
}

