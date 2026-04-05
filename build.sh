#!/bin/sh
GOPATH=$PWD/go GOOS=linux GOARCH=arm exec go build -o monni-arm monni.go
