#!/bin/bash
export PATH=$PATH:$GOPATH/bin
protoc --go_out=plugins=grpc:. bc.proto
