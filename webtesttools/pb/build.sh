#!/bin/bash

# 设置 Protobuf 编译器的搜索路径
PROTO_PATH="."

# 设置 Go 代码的输出目录
GO_OUT_DIR="../test"

# 查找所有 .proto 文件
PROTO_FILES=$(find . -name "*.proto")

# 使用 protoc 编译所有 .proto 文件，并通过 --go_opt=M 选项指定包路径

mkdir -p $GO_OUT_DIR

protoc --proto_path=$PROTO_PATH \
       --go_out=$GO_OUT_DIR \
       --go_opt=paths=source_relative \
       $PROTO_FILES

# 检查编译是否成功
if [ $? -eq 0 ]; then
    echo "Protobuf 文件编译成功"
else
    echo "Protobuf 文件编译失败"
fi