package config

import (
    "encoding/json"
    // "os"
)

type Config struct {
    TopicArn string
    AutoSubscribe bool
    Upstreams []struct {
        AutoScalingGroupARN string
        UpstreamFile string
        UpstreamName string
        UpstreamsContainerFolder string
    }
}

func ReadFile(path string) (c *Config, err error) {
    return
}

func Parse(data []byte) (*Config, error) {
    c := new(Config)
    err := json.Unmarshal(data, c)
    if err != nil {
        return nil, err
    }
    return c, nil
}
