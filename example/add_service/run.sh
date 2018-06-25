#!/usr/bin/env bash

GO_SRC=$GOPATH/src
GOOGLE_API_PATH=$GO_SRC/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis
KITTY_PLUGIN=$GOPATH/bin/protoc-gen-kitty

cd ../../protoc-gen-kitty

go build
mv protoc-gen-kitty ~/go/bin

cd ../example/add_service
protoc -I=. -I=$GO_SRC -I=$GOOGLE_API_PATH --gogo_out=plugins=grpc,bsontag:. add_service.proto
protoc -I=. -I=$GO_SRC -I=$GOOGLE_API_PATH --plugin=$KITTY_PLUGIN --kitty_out=logtostderr=true:. add_service.proto