Elastic NGINX
=============

`Elastic NGINX` is a AWS SNS hook for registering and unregistering instances that goes up or down by AWS Auto Scaling.


Setup
-----

To run `elastic-nginx` all you need to do is to upload the compiled binary to your server. Although, I recommend you set up Upstart (or other service manager) to run it at boot time. Below is a simple Upstart example:

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

You can also install from source using: `go get github.com/rochacon/elastic-nginx`


Run
---

You'll need AWS credentials with EC2 reading permissions.

Export the AWS credentials as environment variables: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. Configuration options can be listed with the `-h` or `--help` flags.


Testing
-------

To run test you'll need a couple more dependencies:

```
go get github.com/globocom/commandmocker
go get launchpad.net/gocheck
```

Run `go test` and see everything passing. :smile:
