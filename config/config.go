package config

import (
	"encoding/json"
	"io/ioutil"
)

type Upstream struct {
	AutoScalingGroupARN string
	File                string
	Name                string
	ContainerFolder     string
}

type Config struct {
	TopicArn      string
	AutoSubscribe bool
	Upstreams     []Upstream
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
