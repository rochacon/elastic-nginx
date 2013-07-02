package main

import (
    // "fmt"
    "launchpad.net/gocheck"
    "net/http"
    "net/http/httptest"
    "io"
    "io/ioutil"
    // "os"
    "strings"
    "testing"
)

func Test(t *testing.T) { gocheck.TestingT(t) }

type TestingSuite struct{}
var _ = gocheck.Suite(&TestingSuite{})

func (s *TestingSuite) SetUpSuite(c *gocheck.C) {
    TopicArn = "arn:test"
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

func (s *TestingSuite) TestReadMessageWithLaunchJSON(c *gocheck.C) {
    b := strings.NewReader(`{"TopicArn":"arn:test","Message":"{\"Event\":\"autoscaling:EC2_INSTANCE_LAUNCH\",\"EC2InstanceId\":\"i-00000\"}"}`)
    recorder, request := newRequest("POST", "/", b, c)
    readMessage(recorder, request)
    body := readBody(recorder.Body, c)
    c.Assert(body, gocheck.Equals, `Added instance "i-00000"`)
    c.Assert(recorder.Code, gocheck.Equals, 200)
}

func (s *TestingSuite) TestReadMessageWithTerminateJSON(c *gocheck.C) {
    b := strings.NewReader(`{"TopicArn":"arn:test","Message":"{\"Event\":\"autoscaling:EC2_INSTANCE_TERMINATE\",\"EC2InstanceId\":\"i-00000\"}"}`)
    recorder, request := newRequest("POST", "/", b, c)
    readMessage(recorder, request)
    body := readBody(recorder.Body, c)
    c.Assert(body, gocheck.Equals, `Removed instance "i-00000"`)
    c.Assert(recorder.Code, gocheck.Equals, 200)
}

func (s *TestingSuite) TestReadMessageWithInvalidJSON(c *gocheck.C) {
    b := strings.NewReader("")
    recorder, request := newRequest("POST", "/", b, c)
    readMessage(recorder, request)
    body := readBody(recorder.Body, c)
    c.Assert(body, gocheck.Equals, "Invalid JSON.\n")
    c.Assert(recorder.Code, gocheck.Equals, 400)
}

func (s *TestingSuite) TestReadMessageWithInvalidMessageJSON(c *gocheck.C) {
    b := strings.NewReader(`{"Message":""}`)
    recorder, request := newRequest("POST", "/", b, c)
    readMessage(recorder, request)
    body := readBody(recorder.Body, c)
    c.Assert(body, gocheck.Equals, "Invalid Message field JSON.\n")
    c.Assert(recorder.Code, gocheck.Equals, 400)
}

func (s *TestingSuite) TestReadMessageFromInvalidTopicArn(c *gocheck.C) {
    b := strings.NewReader(`{"TopicArn":"invalid","Message":"{}"}`)
    recorder, request := newRequest("POST", "/", b, c)
    readMessage(recorder, request)
    body := readBody(recorder.Body, c)
    c.Assert(body, gocheck.Equals, "No handler for the specified arn found.\n")
    c.Assert(recorder.Code, gocheck.Equals, 404)
}
