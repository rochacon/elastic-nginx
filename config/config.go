package config

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	TopicArn      string
	AutoSubscribe bool
	Upstreams     []struct {
		AutoScalingGroupARN      string
		UpstreamFile             string
		UpstreamName             string
		UpstreamsContainerFolder string
	}
}

func ReadFile(path string) (c *Config, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return c, err
	}
	c, err = Parse(data)
	if err != nil {
		return c, err
	}
	return c, nil
}

func Parse(data []byte) (*Config, error) {
	c := new(Config)
	err := json.Unmarshal(data, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
