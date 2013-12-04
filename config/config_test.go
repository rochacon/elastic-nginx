package config

import (
	"fmt"
	"io/ioutil"
	"launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

type S struct {
	data string
}

var _ = gocheck.Suite(&S{
	data: `{
        "TopicArn": "arn:topicarn",
        "AutoSubscribe": true,
        "Upstreams": [
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "UpstreamFile": "/etc/nginx/upstreams.d/backend-0.upstream",
                "UpstreamName": "backend-0",
                "UpstreamsContainerFolder": "/etc/nginx/upstreams.d/backend-0"
            },
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "UpstreamFile": "/etc/nginx/upstreams.d/backend-1.upstream",
                "UpstreamName": "backend-1",
                "UpstreamsContainerFolder": "/etc/nginx/upstreams.d/backend-1"
            },
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "UpstreamFile": "/etc/nginx/upstreams.d/backend-2.upstream",
                "UpstreamName": "backend-2",
                "UpstreamsContainerFolder": "/etc/nginx/upstreams.d/backend-2"
            }
        ]
    }
    `,
})

func (s *S) TestParse(c *gocheck.C) {
	cfg, err := Parse([]byte(s.data))
	c.Check(err, gocheck.IsNil)
	c.Check(cfg.TopicArn, gocheck.Equals, "arn:topicarn")
	c.Check(cfg.AutoSubscribe, gocheck.Equals, true)
	for i, upstream := range cfg.Upstreams {
		c.Check(upstream.AutoScalingGroupARN, gocheck.Equals, "arn:asgtest")
		c.Check(upstream.UpstreamFile, gocheck.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d.upstream", i))
		c.Check(upstream.UpstreamName, gocheck.Equals, fmt.Sprintf("backend-%d", i))
		c.Check(upstream.UpstreamsContainerFolder, gocheck.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d", i))
	}
}

func (s *S) TestReadFile(c *gocheck.C) {
	fp, err := ioutil.TempFile(c.MkDir(), "conf")
	c.Check(err, gocheck.IsNil)
	defer fp.Close()

	err = ioutil.WriteFile(fp.Name(), []byte(s.data), 0644)
	c.Check(err, gocheck.IsNil)

	cfg, err := ReadFile(fp.Name())
	c.Check(err, gocheck.IsNil)
	c.Check(cfg.TopicArn, gocheck.Equals, "arn:topicarn")
	c.Check(cfg.AutoSubscribe, gocheck.Equals, true)
	for i, upstream := range cfg.Upstreams {
		c.Check(upstream.AutoScalingGroupARN, gocheck.Equals, "arn:asgtest")
		c.Check(upstream.UpstreamFile, gocheck.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d.upstream", i))
		c.Check(upstream.UpstreamName, gocheck.Equals, fmt.Sprintf("backend-%d", i))
		c.Check(upstream.UpstreamsContainerFolder, gocheck.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d", i))
	}
}
