#!/bin/bash

protoc -I=./pkg/grpcapi \
  --go_out=./pkg/grpcapi --go_opt=paths=source_relative \
  --go-vtproto_out=./pkg/grpcapi --go-vtproto_opt=paths=source_relative \
  --go-vtproto_opt=features=marshal+unmarshal+size \
  --go-drpc_out=./pkg/grpcapi --go-drpc_opt=paths=source_relative \
  --go-drpc_opt=protolib=github.com/planetscale/vtprotobuf/codec/drpc \
  ./pkg/grpcapi/**/*.proto
