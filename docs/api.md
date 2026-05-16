# YoMirrorSite API 文档

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **健康检查**: `GET /health`

## 统一响应格式

```json
{
  "success": true,
  "data": {},
  "error": "",
  "total": 0
}
```

---

## 健康检查

### GET /health

返回服务及各依赖组件状态。

**响应**:
```json
{
  "status": "ok",
  "deps": {
    "redis": "ok",
    "s3": "ok"
  }
}
```
（PostgreSQL 启用时 deps 含 `"postgres":"ok"`）

---

## 文件管理（YoOSF-API 继承）

### GET /api/files

获取 S3 中的文件列表。

**查询参数**:
| 参数 | 类型 | 说明 |
|---|---|---|
| `prefix` | string | 文件路径前缀过滤 |

**响应**:
```json
{
  "success": true,
  "data": {
    "files": [
      {
        "name": "example.zip",
        "key": "public/example.zip",
        "size": 1048576,
        "last_modified": "2026-01-01T00:00:00Z"
      }
    ],
    "count": 1
  }
}
```

### GET /api/files/url/*

生成指定文件的预签名下载 URL。

**路径参数**: S3 对象 Key（例如 `/api/files/url/public/example.zip`）

**查询参数**:
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `expires` | int | 3600 | 签名有效期（秒） |

**响应**:
```json
{
  "success": true,
  "data": {
    "url": "https://s3.example.com/bucket/public/example.zip?...",
    "expires_in_seconds": 3600
  }
}
```

---

## 文件搜索（YoOSF-API 继承）

### GET /api/search/files

按关键字搜索 S3 中的文件。

**查询参数**:
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `keyword` | string | — | 搜索关键词 |
| `limit` | int | 50 | 返回数量上限 |

**响应**:
```json
{
  "success": true,
  "data": {
    "results": [
      {
        "key": "public/example.zip",
        "name": "example.zip",
        "size": 1048576,
        "last_modified": "2026-01-01T00:00:00Z"
      }
    ],
    "keyword": "example",
    "total": 1
  }
}
```

---

## 软件镜像站（YoMirrorSite 新增）

### 数据模型

#### Software（列表项）
```json
{
  "id": "vscode",
  "name": "Visual Studio Code",
  "description": "Code editing. Redefined.",
  "github_repo": "microsoft/vscode",
  "homepage": "https://code.visualstudio.com",
  "icon_url": "",
  "category": "editor",
  "tags": ["开发工具", "编辑器"],
  "license": "MIT",
  "stars": 168000,
  "latest_ver": "v1.92.0",
  "total_assets": 12,
  "updated_at": "2026-01-01T00:00:00Z"
}
```

#### SoftwareDetail（详情，含版本列表）
```json
{
  "id": "vscode",
  "name": "Visual Studio Code",
  "tags": ["开发工具"],
  "versions": [
    {
      "tag_name": "v1.92.0",
      "name": "August 2024",
      "prerelease": false,
      "published_at": "2024-08-01",
      "asset_count": 3
    }
  ],
  "total_versions": 42,
  "total_size": 5368709120,
  "readme_md": "# Visual Studio Code\n..."
}
```

#### AssetInfo（下载资产）
```json
{
  "name": "VSCode-win32-x64-1.92.0.zip",
  "size": 104857600,
  "size_human": "100.0 MB",
  "platform": "windows-x64",
  "content_type": "application/zip",
  "download_url": "https://s3.example.com/mirrors/vscode/versions/v1.92.0/VSCode-win32-x64-1.92.0.zip?...",
  "checksum": "abc123...",
  "downloads": 1234
}
```

---

### GET /api/mirror/software

获取软件列表（分页 + 排序 + 筛选）。

**查询参数**:
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `category` | string | — | 分类筛选 |
| `keyword` | string | — | 关键词搜索（匹配名称和描述） |
| `page` | int | 1 | 页码 |
| `page_size` | int | 20 | 每页数量（最大 100） |

**响应**:
```json
{
  "success": true,
  "data": {
    "items": [ { "id": "vscode", ... } ],
    "page": 1,
    "page_size": 20,
    "total_count": 5
  }
}
```

### GET /api/mirror/software/:id

获取软件详情（含版本概要列表）。

**路径参数**: `id` — 软件唯一标识（如 `vscode`）

**响应**: `SoftwareDetail` 对象

### GET /api/mirror/software/:id/versions/:tag

获取指定版本详情（含资产列表和下载 URL）。

**路径参数**: `id` — 软件标识，`tag` — git tag（如 `v1.92.0`）

**查询参数**:
| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `expires` | int | 3600 | 下载 URL 签名有效期（秒） |

**响应**:
```json
{
  "success": true,
  "data": {
    "tag_name": "v1.92.0",
    "name": "August 2024",
    "body": "## Release Notes\n...",
    "prerelease": false,
    "published_at": "2024-08-01T00:00:00Z",
    "assets": [
      {
        "name": "VSCode-win32-x64-1.92.0.zip",
        "size": 104857600,
        "size_human": "100.0 MB",
        "platform": "windows-x64",
        "download_url": "https://s3.example.com/...?X-Amz-Signature=...",
        "checksum": "",
        "downloads": 0
      }
    ]
  }
}
```

### GET /api/mirror/software/:id/download/:tag/:asset

下载资产（302 重定向到 S3 预签名 URL）。

**路径参数**: `id` — 软件标识，`tag` — git tag，`asset` — 文件名（URL 编码）

**响应**: HTTP 302 重定向

### GET /api/mirror/stats

获取镜像站统计信息。

**响应**:
```json
{
  "success": true,
  "data": {
    "total_software": 5,
    "total_versions": 120,
    "total_assets": 480,
    "total_size": 53687091200,
    "last_sync_at": "2026-01-01T00:00:00Z",
    "sync_in_progress": false
  }
}
```

---

## 同步管理

### GET /api/sync/status

获取当前同步状态。

**响应**:
```json
{
  "success": true,
  "data": {
    "in_progress": false,
    "current_job": "",
    "queue_length": 0,
    "last_sync_at": "2026-01-01T00:00:00Z",
    "last_result": {
      "software_id": "vscode",
      "new_versions": 1,
      "new_assets": 3,
      "errors": [],
      "duration": "45s"
    }
  }
}
```

### POST /api/sync/trigger

手动触发同步。

**请求体**:
```json
{
  "software_id": "vscode"
}
```
`software_id` 为空时触发全量同步。

**响应**: HTTP 200，异步执行同步任务。
