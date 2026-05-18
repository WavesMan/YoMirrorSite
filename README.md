# YoMirrorSite

> 自由专属软件镜像站 — GitHub Release 自动同步 + S3 / Redis / PostgreSQL 多级存储架构。
> 上游项目：[YoOSF-API](https://github.com/WavesMan/YoOSF-API) — 基于 Fiber + S3 的高性能文件分发服务。

---

## 项目简介

YoMirrorSite 是一个基于 Go 语言开发的软件镜像站，专为团队内部或社区提供常用开发工具的加速下载。通过自动同步 GitHub Release 资产到 S3 对象存储，配合 PostgreSQL 持久化元数据和 Redis 多级缓存，实现高性能的软件浏览、搜索和下载体验。

---

## 核心功能

| 模块 | 能力 |
|---|---|
| **GitHub 同步** | 增量检测 Release、分布式锁互斥、流式上传 S3（不落磁盘）、幂等重试、多同步规则（增量 / 只保留最新 / 全量历史） |
| **软件镜像** | 分类浏览、关键词搜索、星数排序、分页加载 |
| **软件详情** | 版本时间线、Release Notes 渲染、多平台下载、下载计数 |
| **文件下载** | S3 预签名 URL、防击穿锁、TTL 可控 |
| **多级缓存** | 本地 LRU → Redis → PostgreSQL → S3 源站 |
| **健康检查** | `/health` 返回 Redis + S3 + PG 三组件可达性 |
| **同步管理** | 状态查询 + 手动触发 + 历史日志持久化 |

---

## 技术栈

| 层 | 选型 |
|---|---|
| HTTP 框架 | Fiber v2 |
| ORM | GORM + PostgreSQL |
| 缓存 | Redis 7 + go-redis |
| 对象存储 | S3 兼容（MinIO / AWS / Cloudflare R2） |
| 前端 | Vue 3 + NaiveUI + Vite + TypeScript |
| 日志 | Zap |

---

## 快速开始

### 1. 环境要求

- Go 1.22+
- Node.js 20+
- pnpm
- PostgreSQL 16+
- Redis 7+
- S3 兼容存储（MinIO 或云服务商）

### 2. 获取代码

```bash
git clone https://github.com/WavesMan/YoMirrorSite.git
cd YoMirrorSite
```

### 3. 配置

```bash
cp configs/config.yaml.example configs/config.yaml
# 编辑 config.yaml，填入实际的 S3 / Redis / PostgreSQL 连接信息
```

### 4. 一键启动（Docker Compose）

```bash
docker compose up -d
# 自动启动 Redis + PostgreSQL + YoMirrorSite
```

### 5. 本地开发

```bash
# 后端
go run ./cmd/api/

# 前端（另一个终端）
cd web && pnpm install && pnpm dev
```

---

## 架构概览

```
                      ┌──────────┐
                      │  用户     │
                      └────┬─────┘
                           │
                  ┌────────▼────────┐
                  │   Fiber HTTP    │
                  │   :8080         │
                  └──┬──────┬───────┘
                     │      │
         ┌───────────┤      ├──────────────┐
         ▼           ▼                    ▼
   ┌──────────┐ ┌──────────┐     ┌─────────────┐
   │  Redis   │ │PostgreSQL│     │  S3 (MinIO) │
   │  热缓存   │ │ 持久元数据│     │  文件本体    │
   │  分布式锁 │ │ 同步历史  │     │             │
   └──────────┘ └──────────┘     └─────────────┘
         │           │                    │
         └───────────┴────────────────────┘
                     │
              ┌──────▼──────┐
              │ GitHub API  │
              │ Release 同步 │
              └─────────────┘
```

---

## 配置说明

```yaml
server:
  port: 8080                   # 服务端口
  goroutine_pool_size: 100     # 协程池大小

redis:
  addr: "localhost:6379"       # Redis 地址
  password: ""                 # Redis 密码
  db: 0

cache:
  filesListRefreshIntervalSec: 300  # 文件列表缓存刷新间隔
  local_cache:
    enabled: true              # 启用本地 LRU/LFU 热点缓存
    size: 1000
    mode: "lru"

s3:
  access_key: ""               # S3 Access Key
  secret_key: ""               # S3 Secret Key
  endpoint: ""                 # S3 Endpoint URL
  bucket_name: ""              # 存储桶名称
  listen_dir: "mirrors/"

mirror:
  software_list:               # 需要镜像的软件列表（可配多条）
    - id: "vscode"
      name: "Visual Studio Code"
      github_repo: "microsoft/vscode"
      category: "editor"
      tags: ["开发工具"]
      filter_assets:           # 只同步匹配的文件
        - platform: "windows-x64"
          pattern: "VSCode-win32-x64-*.zip"
  sync:
    interval_minutes: 30       # 同步间隔
    github_token: ""           # GitHub Token（可选，提升速率限制）
    max_concurrent: 3          # 同时同步的软件数
    retry_attempts: 3          # 失败重试次数

postgres:                      # 可选，不配则降级到纯 S3+Redis
  host: "localhost"
  port: 5432
  user: "yomirror"
  password: ""
  dbname: "yomirror"
  sslmode: "disable"
  max_conns: 10
```

### 多同步规则

`sync_rule` 支持按软件仓库分别配置三种同步策略：

| 规则 | 配置值 | 行为 |
|------|--------|------|
| 增量同步 | `incremental`（默认） | 从上次同步 tag 之后只拉取新版本 |
| 只保留最新 | `latest_only` | 增量同步后自动清理旧版本，仅保留最近 `keep_versions` 个 |
| 全量历史 | `full_historical` | 无视同步标记，拉取所有 Release |

示例：

```yaml
mirror:
  software_list:
    # 增量同步（默认）
    - id: "vscode"
      github_repo: "microsoft/vscode"
      # sync_rule 不写即默认 incremental

    # 只保留最新 3 个版本
    - id: "golang"
      github_repo: "golang/go"
      sync_rule: "latest_only"
      keep_versions: 3

    # 首次全量同步，之后自动切回增量
    - id: "redis"
      github_repo: "redis/redis"
      sync_rule: "full_historical"
      full_sync_once: true
```

---

## API 概览

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查（含 Redis / S3 / PG 状态） |
| GET | `/api/files` | 文件列表 |
| GET | `/api/files/url/*` | 文件下载 URL |
| GET | `/api/search/files` | 文件搜索 |
| GET | `/api/mirror/software` | 软件列表（分页 + 排序 + 筛选） |
| GET | `/api/mirror/software/:id` | 软件详情（含版本列表） |
| GET | `/api/mirror/software/:id/versions/:tag` | 版本详情（含资产列表） |
| GET | `/api/mirror/software/:id/download/:tag/:asset` | 资产下载（S3 预签名 URL） |
| GET | `/api/mirror/stats` | 镜像站统计 |
| GET | `/api/sync/status` | 同步状态 |
| POST | `/api/sync/trigger` | 手动触发同步 |

详见 [API 文档](docs/api.md)

---

## 部署

### Docker Compose（推荐）

```bash
# 配置环境变量
cp .env.example .env
# 启动
docker compose up -d
```

### 生产部署

```bash
chmod +x ./release-build.sh
./release-build.sh
# 上传部署包至服务器
sudo cp systemctl-files/yomirrorsite.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable yomirrorsite
sudo systemctl start yomirrorsite
```

---

## 上游声明

本项目基于 [YoOSF-API](https://github.com/WavesMan/YoOSF-API) 的 Fiber + S3 + Redis 多级缓存架构扩展，保留原项目的文件管理、搜索、缓存能力。新增模块：

- `internal/core/github/` — GitHub API 客户端
- `internal/syncer/` — Release 同步器 + 定时调度器
- `internal/core/postgres/` — GORM + PostgreSQL 持久化层
- `internal/model/software*` — 软件镜像数据模型
- `internal/service/software_service.go` — 软件业务服务
- `api/handler/software_handler.go` + `sync_handler.go` — 镜像站 HTTP 接口
- `web/` — Vue 3 + NaiveUI 前端

---

## 许可证

[ GPL-v2.0 License ](LICENSE)
