FROM golang:1.16.0-buster as builder

# Define working directory.
WORKDIR /opt/ettus-device-plugin

COPY . .
RUN go get -d ./. && \
	go build -o bin/ettus-device-plugin
    


FROM python:3.8-slim

LABEL maintainer="Carlos Giraldo <cgiraldo@gradiant.org"

# uhd_images_download requirements
RUN pip install six requests

COPY --from=builder /opt/ettus-device-plugin/bin /usr/local/bin
ENTRYPOINT ["/usr/local/bin/ettus-device-plugin"]