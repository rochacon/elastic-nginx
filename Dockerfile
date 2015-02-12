FROM ubuntu:14.04
RUN apt-get update -yq && apt-get install -yq bzr git golang nginx sudo && apt-get clean
ENV GOPATH /app
ADD . /app/src/github.com/rochacon/elastic-nginx
RUN go get -d github.com/rochacon/elastic-nginx && go install github.com/rochacon/elastic-nginx
RUN cp /app/src/github.com/rochacon/elastic-nginx/etc/elastic-nginx.example.json /etc/elastic-nginx.json
RUN mkdir -p /etc/nginx/upstreams.d/backends-0 /etc/nginx/upstreams.d/backends-1
WORKDIR /app
EXPOSE 5000
# ENV AWS_ACCESS_KEY_ID <SECRET>
# ENV AWS_SECRET_ACCESS_KEY <SECRET>
ENTRYPOINT ["/app/bin/elastic-nginx"]
CMD ["-listen", "0.0.0.0:5000"]
