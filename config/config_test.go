package config

import (
	"fmt"
	"gopkg.in/check.v1"
	"io/ioutil"
	"testing"
)

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct {
	data string
}

var _ = check.Suite(&S{
	data: `{
        "TopicArn": "arn:topicarn",
        "AutoSubscribe": true,
        "Upstreams": [
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "File": "/etc/nginx/upstreams.d/backend-0.upstream",
                "Name": "backend-0",
                "ContainerFolder": "/etc/nginx/upstreams.d/backend-0"
            },
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "File": "/etc/nginx/upstreams.d/backend-1.upstream",
                "Name": "backend-1",
                "ContainerFolder": "/etc/nginx/upstreams.d/backend-1"
            },
            {
                "AutoScalingGroupARN": "arn:asgtest",
                "File": "/etc/nginx/upstreams.d/backend-2.upstream",
                "Name": "backend-2",
                "ContainerFolder": "/etc/nginx/upstreams.d/backend-2"
            }
        ]
    }
    `,
})

func (s *S) TestParse(c *check.C) {
	cfg, err := Parse([]byte(s.data))
	c.Check(err, check.IsNil)
	c.Check(cfg.TopicArn, check.Equals, "arn:topicarn")
	c.Check(cfg.AutoSubscribe, check.Equals, true)
	for i, upstream := range cfg.Upstreams {
		c.Check(upstream.AutoScalingGroupARN, check.Equals, "arn:asgtest")
		c.Check(upstream.File, check.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d.upstream", i))
		c.Check(upstream.Name, check.Equals, fmt.Sprintf("backend-%d", i))
		c.Check(upstream.ContainerFolder, check.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d", i))
	}
}

func (s *S) TestReadFile(c *check.C) {
	fp, err := ioutil.TempFile(c.MkDir(), "conf")
	c.Check(err, check.IsNil)
	defer fp.Close()

	err = ioutil.WriteFile(fp.Name(), []byte(s.data), 0644)
	c.Check(err, check.IsNil)

	cfg, err := ReadFile(fp.Name())
	c.Check(err, check.IsNil)
	c.Check(cfg.TopicArn, check.Equals, "arn:topicarn")
	c.Check(cfg.AutoSubscribe, check.Equals, true)
	for i, upstream := range cfg.Upstreams {
		c.Check(upstream.AutoScalingGroupARN, check.Equals, "arn:asgtest")
		c.Check(upstream.File, check.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d.upstream", i))
		c.Check(upstream.Name, check.Equals, fmt.Sprintf("backend-%d", i))
		c.Check(upstream.ContainerFolder, check.Equals, fmt.Sprintf("/etc/nginx/upstreams.d/backend-%d", i))
	}
}
