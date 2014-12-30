Elastic NGINX
=============

[![Build Status](https://drone.io/github.com/rochacon/elastic-nginx/status.png)](https://drone.io/github.com/rochacon/elastic-nginx/latest)

`elastic-nginx` is an AWS SNS hook for registering and unregistering instances that goes up or down by AWS Auto Scaling.


Setup
-----

Download (or build) the `elastic-nginx` binary and upload it to your server. You can just run the binary, but I recommend you to set up Upstart (or other service manager) to run it at boot time. Below is a simple Upstart example:

```
# Upstart script for Elastic NGINX
start on runlevel [2345]
stop on starting rc RUNLEVEL=[016]

respawn
respawn limit 10 5

console log

env AWS_ACCESS_KEY_ID="A_AWS_ACCESS_KEY_ID"
env AWS_SECRET_ACCESS_KEY="A_AWS_SECRET_ACCESS_KEY"

exec /usr/local/bin/elastic-nginx -aws-region="us-east-1" -config "/etc/elastic-nginx.json"
```

A sample configuration file can be found at etc/elastic-nginx.example.json.
More configuration options can be listed with the `-h` or `--help` flags.

**Notes:**

  * You'll need AWS credentials with EC2 reading permissions.
  * You'll need to subscribe to the SNS Topic after registering the HTTP hook. You can do this automatically by turning auto-subscribe feature on. (see config file)


Testing
-------

To run the test suite you'll need a couple more dependencies:

```
go get github.com/tsuru/commandmocker
go get gopkg.in/check.v1
```

Run `go test ./...` and see everything passing. :smile:
