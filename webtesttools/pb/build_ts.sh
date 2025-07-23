#!/bin/bash

PROTO_PATH="."
TS_OUT_DIR="./ts_out"

mkdir -p $TS_OUT_DIR

PROTO_FILES=$(find $PROTO_PATH -name "*.proto")

# Get the absolute path of protoc-gen-ts_proto
TS_PROTO_PLUGIN=$(pwd)/node_modules/.bin/protoc-gen-ts_proto

for file in $PROTO_FILES; do
  echo "Generating: $file"
  protoc \
    --plugin=protoc-gen-ts_proto=$TS_PROTO_PLUGIN \
    --proto_path=$PROTO_PATH \
    --ts_proto_out=$TS_OUT_DIR \
    --ts_proto_opt=outputServices=grpc-js,esModuleInterop=true,forceLong=string,useOptionals=messages \
    $file
done

echo "TS Proto files generated successfully"