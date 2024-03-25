#!/bin/bash

protoc -I=./pkg/api \
  --go_out=./pkg/api --go_opt=paths=source_relative \
  --go-vtproto_out=./pkg/api --go-vtproto_opt=paths=source_relative \
  --go-vtproto_opt=features=marshal+unmarshal+size \
  --go-drpc_out=./pkg/api --go-drpc_opt=paths=source_relative \
  --go-drpc_opt=protolib=github.com/planetscale/vtprotobuf/codec/drpc \
  ./pkg/api/**/*.proto
