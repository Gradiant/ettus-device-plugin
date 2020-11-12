.PHONY: all build docker

build:
	go get -d ./. 
	go build -o bin/ettus-device-plugin

docker: build
	docker build -t gradiant/ettus-device-plugin:0.0.1 .
