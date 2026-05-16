# YoMirrorSite 开发者文档

## 目录结构

```
YoMirrorSite/
├── api/                          # API 层
│   ├── handler/                  # HTTP 处理器（薄层，只做参数绑定）
│   │   ├── file_handler.go       # 文件列表 / 下载 URL
│   │   ├── search_handler.go     # 文件搜索
│   │   ├── software_handler.go   # 软件列表 / 详情 / 下载
│   │   └── sync_handler.go       # 同步状态 / 手动触发
│   ├── model/                    # 请求 / 响应数据模型
│   └── router/                   # Fiber 路由注册
│       └── router.go             # SetupRouter + RegisterMirrorRoutes
├── cmd/api/main.go               # 入口：初始化 → 路由 → 启动
├── configs/
│   ├── config.yaml               # 运行配置（不入库）
│   └── config.yaml.example       # 配置模板
├── docs/                         # 项目文档
├── internal/
│   ├── config/config.go          # 配置结构体 + 加载 + 验证
│   ├── core/
│   │   ├── github/client.go      # GitHub API v3 客户端
│   │   ├── postgres/             # GORM PostgreSQL 层
│   │   │   ├── client.go         # 连接管理 + AutoMigrate
│   │   │   ├── software_repo.go  # 软件 CRUD + 分页排序
│   │   │   ├── version_repo.go   # 版本 / 资产操作
│   │   │   └── sync_log_repo.go  # 同步日志
│   │   └── s3/                   # S3 对象存储封装
│   ├── model/
│   │   ├── software.go           # 接口数据模型
│   │   └── software_gorm.go      # GORM 表映射
│   ├── service/
│   │   ├── file_service.go       # 文件业务服务
│   │   ├── software_service.go   # 软件业务服务
│   │   └── search_service.go     # 搜索服务
│   ├── syncer/
│   │   ├── github.go             # GitHub Release 同步器
│   │   └── scheduler.go          # 定时调度器
│   └── util/                     # 工具库
├── web/                          # Vue 3 + NaiveUI 前端
│   └── src/
│       ├── views/                # 页面组件
│       ├── components/           # 通用组件
│       ├── api/                  # API 请求封装
│       ├── stores/               # Pinia 状态
│       └── types/                # TypeScript 类型
├── Dockerfile                    # 多阶段构建
├── docker-compose.yml            # 本地开发编排
└── go.mod
```

## 模块说明

### GitHub 客户端 (`internal/core/github/client.go`)

纯 `net/http` 实现，不依赖第三方 SDK。核心能力：
- `GetRepoInfo` — 获取 Stars、License、Description
- `ListReleases` — 分页拉取 Release 列表
- `DownloadAsset` — 流式下载 Release 资产
- ETag 条件请求减少 API 消耗

### GitHub 同步器 (`internal/syncer/github.go`)

核心同步流程：
1. `AcquireLock` — Redis SetNX 分布式锁（30min TTL）
2. 读取 `last_synced:software:{id}` 增量标记
3. 分页拉取 GitHub Releases，仅处理新版本
4. 逐资产：HeadObject 检查 S3 → 流式下载 → PutObject
5. 每条资产上传后立即 INSERT asset 到 PG（幂等：s3_key unique）
6. Upsert version + software + sync_log
7. `ReleaseLock`

### 定时调度器 (`internal/syncer/scheduler.go`)

- 启动时立即全量同步一次
- `time.NewTicker(interval)` 循环触发
- Context-with-cancel 实现优雅停止
- Channel 信号量控制并发数

### PostgreSQL 层 (`internal/core/postgres/`)

- `AutoMigrate` 自动建表（幂等：CREATE TABLE IF NOT EXISTS）
- `OnConflict` 实现 Upsert 语义
- 对账辅助：`ListS3KeysBySoftware` → 与 S3 实际对象列表 diff

### 软件业务服务 (`internal/service/software_service.go`)

查询链路：本地 LRU → Redis → **PostgreSQL** → S3 源站。PG 启用时直接用 SQL 分页+排序。

## 数据流

### 同步流程

```
Scheduler.tick
  └─ GitHubSyncer.SyncSoftware
       ├─ AcquireLock
       ├─ ListReleases
       ├─ FOR EACH 新 Release:
       │   ├─ UpsertVersion (PG)
       │   └─ FOR EACH Asset:
       │       ├─ S3 PutObject
       │       └─ BatchInsertAssets (PG)
       ├─ UpsertSoftware (PG)
       ├─ CreateSyncLog → FinishSyncLog (PG)
       └─ ReleaseLock
```

### 查询流程

```
HTTP GET /api/mirror/software/:id
  └─ SoftwareService.GetSoftware
       ├─ PG: GetSoftware + GetTags + ListVersions → 组装返回
       └─ 降级: Redis → S3 meta.json
```

## 开发环境

### Go 编译

```bash
# 设置国内代理
set GOPROXY=https://mirrors.cloud.tencent.com/go/,direct
go mod tidy
go build -o yo-mirror.exe ./cmd/api/
```

### 前端开发

```bash
cd web
pnpm install
pnpm dev          # Vite HMR，代理 /api → localhost:8080
pnpm build        # 生产构建 → web/dist/
```

### 中间件配置

开发时可通过 `config.yaml` 连接远程中间件：

```yaml
postgres:
  host: "172.17.13.118"
  port: 5432
  user: "yomirror_dev"
  password: "YoMir2026Site"
  dbname: "YoMirrorSite"
  sslmode: "require"    # 自签证书用 require 不验证 CN

redis:
  addr: "172.17.13.118:6379"
  password: "redis_yQzzP2"
```

## 数据库

### 表结构（GORM AutoMigrate 自动创建）

| 表名 | 说明 | 唯一键 |
|---|---|---|
| `software` | 软件基本信息 | pk: id |
| `software_tag` | 软件标签 | pk: (software_id, tag) |
| `version` | 版本记录 | uk: (software_id, tag_name) |
| `asset` | 资产文件 | uk: s3_key |
| `sync_log` | 同步日志 | pk: id |

### 调试 SQL

```bash
# 连接 PG
psql -h 172.17.13.118 -U yomirror_dev -d YoMirrorSite

# 查看已同步的软件
SELECT id, name, latest_ver, stars, updated_at FROM software;

# 查看同步历史
SELECT software_id, status, new_versions, new_assets, finished_at
FROM sync_log ORDER BY started_at DESC LIMIT 10;

# 对账：PG 与 S3 的资产差异
SELECT s3_key FROM asset WHERE s3_key NOT IN (...);  -- 手动对比
```
