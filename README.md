Elastic NGINX
=============

`Elastic NGINX` is a AWS SNS hook for registering and unregistering instances that goes up or down by AWS Auto Scaling.


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

# Settings
env AWS_ACCESS_KEY_ID="A_AWS_ACCESS_KEY_ID"
env AWS_SECRET_ACCESS_KEY="A_AWS_SECRET_ACCESS_KEY"

exec /usr/local/bin/elastic-nginx -aws-region="us-east-1" -topic-arn="arn:test" -upstream="backends" -upstream-file="/etc/nginx/conf.d/upstreams/backends.upstreams" -upstreams-path="/etc/nginx/conf.d/upstreams/backends"
```

Configuration options can be listed with the `-h` or `--help` flags.

**Notes:**

  * You'll need AWS credentials with EC2 reading permissions.
  * You'll need to subscribe to the SNS Topic after registering the HTTP hook. Look at the `elastic-nginx` output to get the received subscribe URL.


Testing
-------

To run test you'll need a couple more dependencies:

```
go get github.com/globocom/commandmocker
go get launchpad.net/gocheck
```

Run `go test` and see everything passing. :smile:
