from ubuntu:12.04
run echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
run apt-get update -yq
run apt-get install -yq bzr git golang nginx sudo
env GOPATH /app
run go get github.com/rochacon/elastic-nginx
add etc/elastic-nginx.example.json /etc/elastic-nginx.json
run mkdir -p /etc/nginx/upstreams.d/backends-0 /etc/nginx/upstreams.d/backends-1
workdir /app
expose 5000
env AWS_ACCESS_KEY_ID <SECRET>
env AWS_SECRET_ACCESS_KEY <SECRET>
cmd /app/bin/elastic-nginx
