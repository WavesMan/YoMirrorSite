# YoOSF API 文档

## 概述

S3文件服务是一个基于Go和Gin框架的RESTful API服务，用于管理S3对象存储中的文件。支持文件列表查询、下载URL生成、文件上传等功能。

## 基础信息

- **基础URL**: `http://localhost:8080`
- **API前缀**: `/api`
- **默认端口**: 8080

## 认证

当前版本API无需认证，但可以通过配置CORS来限制访问来源。

## API端点

### 健康检查

检查API服务器状态。

**请求**
- **方法**: GET
- **路径**: `/health`

**响应**
```json
{
  "status": "ok"
}
```

**示例**
```bash
curl "http://localhost:8080/health"
```

### 获取文件列表

获取S3存储桶中的文件列表。

**请求**
- **方法**: GET
- **路径**: `/api/files`
- **查询参数**:
  - `prefix` (可选): 文件路径前缀过滤

**响应**
```json
{
  "success": true,
  "data": {
    "files": [
      {
        "name": "dist/index.html",
        "key": "dist/index.html",
        "size": 6231,
        "last_modified": "2025-10-04T14:31:10.792Z"
      }
    ],
    "count": 1
  }
}
```

**示例**
```bash
# 获取所有文件
curl "http://localhost:8080/api/files"

# 获取指定前缀的文件
curl "http://localhost:8080/api/files?prefix=dist/"
```

### 获取文件下载URL

为指定文件生成预签名的下载URL。

**请求**
- **方法**: GET
- **路径**: `/api/files/url/{文件路径}`
- **路径参数**:
  - `文件路径`: S3对象的完整路径（支持包含斜杠的路径）
- **查询参数**:
  - `expires` (可选): URL过期时间（秒），默认3600秒

**响应**
```json
{
  "success": true,
  "data": {
    "url": "https://cn-nb1.rains3.com/ly-test/dist/index.html?X-Amz-Algorithm=...",
    "expires_in_seconds": 3600
  }
}
```

**示例**
```bash
# 获取文件下载URL（默认1小时过期）
curl "http://localhost:8080/api/files/url/dist/index.html"

# 获取文件下载URL（30分钟过期）
curl "http://localhost:8080/api/files/url/dist/index.html?expires=1800"

# 获取子目录中的文件
curl "http://localhost:8080/api/files/url/dist/css/index-5115566d.css"
```

## 搜索相关API

### 搜索文件

在存储桶中全局搜索文件，支持文件名模糊匹配。

**请求**
- **方法**: GET
- **路径**: `/api/search/files`
- **查询参数**:
  - `keyword` (必需): 搜索关键词
  - `limit` (可选): 结果数量限制，默认50，最大100

**响应**
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "key": "mirror/papermc/paper/1.12.2/paper-1.12.2-1575.jar",
        "name": "paper-1.12.2-1575.jar",
        "path": "mirror/papermc/paper/1.12.2/",
        "size": 40226024,
        "last_modified": "2025-10-08T11:13:35.335Z",
        "type": "file"
      }
    ],
    "total_count": 1,
    "keyword": "1.12",
    "limit": 50
  }
}
```

**示例**
```bash
# 搜索包含"paper"的文件
curl "http://localhost:8080/api/search/files?keyword=paper"

# 搜索包含"1.12"的文件，限制10个结果
curl "http://localhost:8080/api/search/files?keyword=1.12&limit=10"
```

## 错误处理

所有API都使用统一的错误响应格式：

```json
{
  "success": false,
  "error": "错误描述信息"
}
```

### 常见错误码

- **400**: 请求参数错误
- **404**: 资源未找到
- **500**: 服务器内部错误

## 配置说明

### S3配置
```yaml
s3:
  access_key: "your-access-key"
  secret_key: "your-secret-key"
  endpoint: "https://your-s3-endpoint.com"
  bucket_name: "your-bucket-name"
  listen_dir: ""  # 监听目录前缀
```

## 使用示例

### 完整工作流程

1. **检查服务状态**
   ```bash
   curl "http://localhost:8080/health"
   ```

2. **查看可用文件**
   ```bash
   curl "http://localhost:8080/api/files"
   ```

3. **获取文件下载URL**
   ```bash
   curl "http://localhost:8080/api/files/url/dist/index.html"
   ```

4. **使用URL下载文件**
   ```bash
   # 复制返回的URL到浏览器或使用curl下载
   curl -O "返回的下载URL"
   ```
