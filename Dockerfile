FROM python:3.8-slim

LABEL maintainer="Carlos Giraldo <cgiraldo@gradiant.org"

# uhd_images_download requirements
RUN pip install six requests

COPY bin/ /usr/local/bin

ENTRYPOINT ["/usr/local/bin/ettus-device-plugin"]