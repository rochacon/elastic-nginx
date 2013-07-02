Elastic NGINX
=============

This is a simple app for registering and unregistering instances that goes live or dead by the Auto Scaling definitions.


Building
--------

To build the project you'll need the Go compiler. You can find instructions on how to setup your Go environment [here](http://golang.org/doc/install).

With your Go environment setup, run the Makefile (e.g. `make`). The binary will be located at `dist/elastic-nginx`


Installing
----------

Just upload the binary to your server. =)


Running
-------

You'll need AWS credentials with EC2 reading permissions.

Export the AWS credentials as `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` as environment variables. Other options can be listed with the `-h` or `--help` flags.
