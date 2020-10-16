#!/bin/bash

set -ex

case "$#" in

1)  echo "using ctr command"
    CMD="ctr"
    ;;
2)  echo  "Using $1 ctr command"
    CMD="$1 ctr"
    shift;
    ;;
*)     echo "usage:  ./containerd_import.sh [microk8s|k3s] image:image_tag"
    exit 0;
   ;;
esac

docker save $1 -o image-tmp.tar
$CMD image import image-tmp.tar
rm image-tmp.tar
