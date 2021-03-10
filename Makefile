.PHONY: all build docker

build:
	go get -d ./.
	go build -o bin/ettus-device-plugin 
docker:
	docker build -t gradiant/ettus-device-plugin:0.0.1 .

kubernetes:
    kubectl apply -f ettus-daemonset.yaml