# PowerShell 构建 Linux 发行版脚本

# 确保脚本在发生错误时停止执行
$ErrorActionPreference = "Stop"

# 环境变量设置
$Env:CGO_ENABLED = "0"
$Env:GOARCH = "amd64"
$Env:GOOS = "linux"

# 定义构建相关变量
$DATE = Get-Date -Format "yy.MM.dd"
$BUILD_DIR = "build-temp"
$ROOT_DIR = "mirror_website"
$ZIP_NAME = "linux-release-$DATE.zip"

Write-Host "清理旧的构建目录和 ZIP 包..."
if (Test-Path $BUILD_DIR) { Remove-Item -Recurse -Force $BUILD_DIR }
if (Test-Path $ZIP_NAME) { Remove-Item -Force $ZIP_NAME }

# 创建构建所需的目录结构
Write-Host "创建构建目录结构..."
New-Item -ItemType Directory -Force -Path "$BUILD_DIR/$ROOT_DIR/bin"
New-Item -ItemType Directory -Force -Path "$BUILD_DIR/$ROOT_DIR/configs"
New-Item -ItemType Directory -Force -Path "$BUILD_DIR/$ROOT_DIR/web"
New-Item -ItemType Directory -Force -Path "$BUILD_DIR/$ROOT_DIR/systemctl-files"

# 编译后端服务到 bin 目录
Write-Host "编译后端服务到 bin 目录..."
go build -o "$BUILD_DIR/$ROOT_DIR/bin/api" "./cmd/api"
go build -o "$BUILD_DIR/$ROOT_DIR/bin/sync" "./cmd/sync"

# 复制配置文件
Write-Host "复制配置文件..."
Copy-Item "configs/config.yaml.example" "$BUILD_DIR/$ROOT_DIR/configs/config.yaml.example"

# 复制 systemctl 文件
Write-Host "复制 systemctl 文件..."
Copy-Item "systemctl-files/mirrorweb-api.service" "$BUILD_DIR/$ROOT_DIR/systemctl-files/mirrorweb-api.service"
Copy-Item "systemctl-files/mirrorweb-sync.service" "$BUILD_DIR/$ROOT_DIR/systemctl-files/mirrorweb-sync.service"

# 构建前端项目
Write-Host "构建前端项目..."
Push-Location "web"
pnpm install
pnpm build
Pop-Location

# 复制前端构建结果
Write-Host "复制前端构建结果..."
Copy-Item -Recurse "web/dist" "$BUILD_DIR/$ROOT_DIR/web/"

# 开始打包
Write-Host "开始打包..."
Push-Location $BUILD_DIR
Compress-Archive -Path "$ROOT_DIR/*" -DestinationPath "../$ZIP_NAME"
Pop-Location

# 清理临时文件
Write-Host "返回原目录并清理临时文件..."
Remove-Item -Recurse -Force $BUILD_DIR

Write-Host "打包完成：$ZIP_NAME"