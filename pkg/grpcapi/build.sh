#!/bin/bash

export GOPATH=$(go env GOPATH)

echo **/*.proto

protoc -I=. \
  --go_out="$PWD" --go_opt=paths=source_relative \
  --go-vtproto_out="$PWD" --go-vtproto_opt=paths=source_relative \
  --go-vtproto_opt=features=marshal+unmarshal+size \
  --go-drpc_out="$PWD" --go-drpc_opt=paths=source_relative \
  --go-drpc_opt=protolib=github.com/planetscale/vtprotobuf/codec/drpc \
  **/*.proto
