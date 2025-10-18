#!/bin/bash

set -e

DATE=$(date +%y.%m.%d)
BUILD_DIR=build-temp
ROOT_DIR=mirror_website
ZIP_NAME="linux-release-$DATE.zip"  # 定义ZIP文件名变量

echo "清理旧的构建目录和ZIP包..."
rm -rf $BUILD_DIR
rm -f $ZIP_NAME  # 关键：删除已存在的旧ZIP包，避免追加

# 创建带根目录的完整结构
mkdir -p $BUILD_DIR/$ROOT_DIR/bin
mkdir -p $BUILD_DIR/$ROOT_DIR/configs
mkdir -p $BUILD_DIR/$ROOT_DIR/web
mkdir -p $BUILD_DIR/$ROOT_DIR/systemctl-files

echo "编译后端服务到 bin 目录..."
go build -o $BUILD_DIR/$ROOT_DIR/bin/api ./cmd/api
go build -o $BUILD_DIR/$ROOT_DIR/bin/sync ./cmd/sync

echo "复制配置文件..."
cp configs/config.yaml.example $BUILD_DIR/$ROOT_DIR/configs/config.yaml.example

echo "复制 systemctl 文件..."
cp systemctl-files/mirrorweb-api.service $BUILD_DIR/$ROOT_DIR/systemctl-files/mirrorweb-api.service
cp systemctl-files/mirrorweb-sync.service $BUILD_DIR/$ROOT_DIR/systemctl-files/mirrorweb-sync.service

echo "构建前端项目..."
cd web
pnpm install
pnpm build
cd ..

echo "复制前端构建结果..."
cp -r web/dist $BUILD_DIR/$ROOT_DIR/web/

echo "开始打包..."
cd $BUILD_DIR
zip -r ../$ZIP_NAME $ROOT_DIR  # 使用变量统一文件名

echo "返回原目录并清理临时文件..."
cd ..
rm -rf $BUILD_DIR

echo "打包完成：$ZIP_NAME"