#!/bin/sh
sudo docker pull docker.io/library/golang:1.16
sudo docker run -v $PWD:/src -it docker.io/library/golang:1.16 /bin/bash -c "cd /src; ./build.sh"

