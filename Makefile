default: build

build:
	go build -o dist/elastic-nginx elnginx.go

build_linux:
	GOOS=linux GOARCH=amd64 go build -o dist/elastic-nginx elnginx.go
