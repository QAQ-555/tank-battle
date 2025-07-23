#!/bin/bash

# 设置 Protobuf 编译器的搜索路径
PROTO_PATH="."

# 设置 Python 代码的输出目录
PY_OUT_DIR="../test_py"

# 查找所有 .proto 文件
PROTO_FILES=$(find . -name "*.proto")

# 确保输出目录存在
mkdir -p "$PY_OUT_DIR"

# 使用 protoc 编译所有 .proto 文件
protoc --proto_path="$PROTO_PATH" \
       --python_out="$PY_OUT_DIR" \
       $PROTO_FILES

# 检查编译是否成功
if [ $? -eq 0 ]; then
    echo "Protobuf 文件编译为 Python 成功"
else
    echo "Protobuf 文件编译为 Python 失败"
fi
