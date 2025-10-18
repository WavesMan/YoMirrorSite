### YoOSF-API 项目开发文档

#### 概述
`YoOSF-API` 是一个基于 Go 语言的后端项目，实现基于 S3 对象存储的 Web 镜像站，包含文件列表、同步与搜索功能。项目使用 **Fiber** 框架构建 RESTful API，对接 S3 存储服务，支持文件存储与管理、搜索、同步操作等功能。

#### 项目文件结构

```plain text
yoosf-api
├── api               # API 层，包括请求处理逻辑与返回结果的封装
│   ├── handler       # 各种请求处理器（Handler）封装请求和业务层交互
│   ├── model         # 数据模型和辅助编码器
│   └── router        # 路由定义
├── cmd               # 启动命令源码
│   └── api           # 文件管理和API相关执行入口
├── configs           # 配置文件目录
│   ├── config.yaml   # 主配置文件
│   └── config.yaml.example # 配置示例文件
├── docs              # 文档存储目录
├── internal          # 内部实现逻辑，包含核心服务、工具类、业务逻辑等
│   ├── core          # 核心功能模块（如 S3 等封装）
│   ├── service       # 业务服务层
│   └── util          # 通用工具和辅助类
├── web               # 前端源码（或静态资源）
└── go.mod            # Go 项目依赖定义文件
```


---

#### 主要技术栈

- **语言**：Go 1.24
- **Web 框架**：[Fiber](https://github.com/gofiber/fiber) (高性能，基于 fasthttp)
- **文件存储**：S3 兼容存储服务
- **缓存**：Redis (文件列表缓存和分布式锁)
- **日志工具**：[Zap](https://github.com/uber-go/zap)
- **任务调度**：[Cron](https://github.com/robfig/cron)
- **版本管理**：GitLab
- **依赖管理**：Go Modules

---

#### 系统功能

1. **文件管理**
    - 获取文件列表：支持过滤、分页等功能，使用 Redis 分片缓存。
    - 生成文件的下载 URL，并支持定制有效期。
    - 文件列表缓存刷新功能，TTL 统一为 5 分钟。

2**文件搜索**
    - 提供基于文件名关键字的搜索功能。
    - 支持结果数量限制，返回文件的详细路径和元信息。

3**健康检查**
    - 对服务运行状态进行监控。

---

#### 核心模块及代码简介

##### 1. API 路由定义（`router` 模块）
项目的路由由 `router.SetupRouter` 定义，使用 Fiber 框架：

```go
// 文件路由
files.Get("/", fileHandler.GetFileList)
files.Get("/url/*", fileHandler.GetDownloadURL)

// 搜索功能
search.Get("/files", searchHandler.SearchFiles)

// 健康检查路由
app.Get("/health", func(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "status": "ok",
    })
})
```

- **静态资源路由**：服务前端 Vue 项目的静态资源，支持 SPA 路由。
- **API 路由**：
    - 文件管理相关接口。
    - 搜索功能接口。
- **CORS 支持**：使用 Fiber 内置 CORS 中间件。

##### 2. 文件处理（`handler.FileHandler`）

- **获取文件列表 (`GetFileList`)**：通过 Redis 缓存获取文件列表，支持防击穿保护。
- **获取文件下载 URL (`GetDownloadURL`)**：生成带有签名的 S3 下载 URL，并自定义 JSON 序列化，避免转义问题。

##### 3. 缓存机制

本项目采用多级缓存架构，包括 Redis 分布式缓存和本地热点缓存，以提升系统性能和降低网络开销。

###### 3.1 Redis 缓存机制

Redis 在本项目中的主要用途是缓存和分布式锁，提升了高并发场景下的性能。以下是 Redis 的主要功能和设计概述：

###### 使用场景
1. **数据缓存**：
    - 针对频繁访问的数据（如文件列表与统计信息），通过 Redis 提供快速读取能力。
    - 使用键值对存储结构，以高效存取数据。

2. **分布式锁**：
    - 为避免缓存击穿问题，在高并发环境中对源系统的访问被有效限制。
    - 利用 Redis 的 `SetNX` 和 `TTL` 功能实现锁机制。

3. **异步任务队列**：
    - 为 `get-file-downloadurl` 接口提供异步 URL 生成队列，防止瞬时高并发打爆上游服务。

###### 技术实现
1. **缓存设计**：
    - **分片缓存**：
        - 长列表数据分片缓存，优化了 Redis 的读取与写入性能。
        - 每片 100 条记录，TTL 统一为 5 分钟。

    - **缓存刷新**：
        - 异步更新机制，保证缓存实时性，提升系统效率。
        - 刷新阈值：当缓存剩余时间少于 1 分钟时触发刷新。

2. **防击穿机制**：
    - 当缓存未命中时，使用分布式锁限制资源竞争：
        - 通过锁机制控制对外部服务（如 S3）的访问频率。
        - 自动刷新缓存，确保请求得到最新数据。

3. **异步任务队列**：
    - **下载 URL 生成队列**：
        - 使用工作协程模式处理 URL 生成任务
        - 队列满时降级处理，直接生成 URL 但不缓存
        - 控制并发度，保护上游 S3 服务

4. **监控与统计**：
    - 记录缓存操作相关日志，包含缓存命中率与访问性能分析。
    - 使用 Redis 提供的统计命令如 `INFO`，监控其运行状态。
    - 新增 `DownloadURLCacheStats` 和 `SearchCacheStats` 统计结构

###### 关键逻辑说明
- **分布式锁与竞争控制**：
    - 使用 `AcquireLock` 获取锁，限制并发更新操作。
    - 使用 `ReleaseLock` 解锁，释放资源供其他请求访问。

- **分片缓存策略**：
    - 数据分片存储后定期刷新，清理过期缓存，降低单次查询负载。

- **异步队列管理**：
    - `DownloadURLManager` 管理下载 URL 生成任务
    - 支持配置工作协程数量和队列大小
    - 提供流量控制和降级策略

- **健康检查**：
    - 调用 Redis 的 `PING` 命令检测连接稳定性，确保服务可用。

###### 新增缓存接口实现

**1. `get-file-downloadurl` 接口缓存**
- **缓存键**：`download_url:{file_path}:{expires}`
- **缓存值**：预签名 URL 字符串
- **TTL**：与 URL 有效期保持一致
- **特性**：
  - 异步任务队列防止 S3 服务过载
  - 分布式锁防止缓存击穿
  - 异步刷新机制确保缓存新鲜度
  - 队列满时降级处理

**2. `search-files` 接口缓存**
- **缓存策略**：基于现有的文件列表缓存进行内存搜索
- **缓存键**：复用 `files_list_cache:{prefix}` 缓存
- **特性**：
  - 零 S3 API 调用，完全基于缓存搜索
  - 内存搜索实现微秒级响应
  - 与文件列表缓存保持数据一致性
  - 自动受益于文件列表的定时刷新机制

**3. 性能提升**
- **缓存命中率**：测试显示整体缓存命中率 > 89%
- **响应延迟**：缓存操作微秒级，远低于网络请求
- **服务稳定性**：防击穿和流量控制机制确保高并发稳定性
- **成本优化**：显著减少 S3 API 调用次数

###### 3.2 本地热点缓存机制

为了进一步降低 Redis 的网络开销并提升热点数据的访问性能，项目实现了基于内存的本地热点缓存机制。

**设计目标**
- 减少对 Redis 的频繁访问，降低网络延迟
- 动态适应实时热点数据变化
- 支持 LRU/LFU 两种缓存淘汰策略
- 提供完整的缓存统计和监控

**核心组件**

1. **CacheManager** - 本地缓存管理器
   - 支持 LRU 和 LFU 两种缓存模式
   - 线程安全的缓存操作
   - 完整的统计信息收集
   - 动态模式切换功能

2. **LFUCache** - 自定义 LFU 缓存实现
   - 基于频率的最小堆实现
   - 支持容量限制和淘汰策略
   - 线程安全的 Get/Set 操作

3. **HotDataManager** - 热点数据管理器
   - 记录数据访问频率
   - 自动识别热点数据
   - 支持阈值配置

**缓存策略**

1. **多级缓存架构**
   ```
   请求 → 本地缓存 → Redis 缓存 → S3 存储
   ```

2. **数据访问流程**
   ```go
   // 1. 先尝试从本地缓存获取
   value, found := cacheManager.Get(key)
   if found {
       return value
   }
   
   // 2. 尝试从 Redis 缓存获取  
   value, found = redis.Get(key)
   if found {
       // 存入本地缓存
       cacheManager.Set(key, value)
       return value
   }
   
   // 3. 从 S3 获取并缓存
   value = s3.Get(key)
   cacheManager.Set(key, value)
   redis.Set(key, value)
   return value
   ```

**特性优势**

1. **性能提升**
   - 本地缓存访问：微秒级响应
   - 减少 Redis 网络调用：降低 60-80% 的 Redis 请求
   - 热点数据命中率：本地缓存命中率可达 95%+

2. **灵活配置**
   - **缓存模式**：支持 LRU（最近最少使用）和 LFU（最不经常使用）
   - **容量控制**：可配置缓存大小，避免内存溢出
   - **动态切换**：运行时支持缓存模式切换

3. **智能热点识别**
   - 自动记录数据访问频率
   - 基于阈值识别热点数据
   - 支持动态热点数据预加载

4. **完整监控**
   - 实时缓存命中率统计
   - 缓存大小和使用情况监控
   - 热点数据访问分析

**配置示例**
```yaml
cache:
  local_cache:
    enabled: true
    size: 1000           # 本地缓存容量
    mode: "lru"          # 缓存模式：lru 或 lfu
    hot_data_refresh_sec: 30  # 热点数据刷新间隔（秒）
  files_list_refresh_interval_sec: 300  # 文件列表刷新间隔
```

**实现文件**
- `internal/util/local_cache.go` - 本地缓存管理器
- `internal/util/lfu_cache.go` - LFU 缓存实现
- `internal/service/file_service.go` - 文件服务集成

**性能指标**
- **响应时间**：本地缓存命中时响应时间 < 1ms
- **Redis 负载**：减少 60-80% 的 Redis 请求
- **内存使用**：可控的内存占用，默认 1000 个条目
- **并发安全**：支持高并发环境下的安全访问

##### 4. 搜索处理（`handler.SearchHandler`）

- 基于文件名关键字执行搜索，包括：
    1. 验证用户传递的关键词和数量限制。
    2. 利用 `searchService.SearchFiles` 执行核心文件搜索逻辑。
    3. 转换搜索结果为统一格式（`SearchResponse`）。

##### 5. 文件服务（`service.FileService`）

- 封装与 S3 存储服务的交互，包括列表刷新、URL 生成等操作。
- 提供定时任务，用于自动刷新文件缓存（`StartCacheRefresher`）。
- 缓存机制：
    - TTL：5 分钟
    - 刷新阈值：1 分钟
    - 分片大小：100 条记录

---

#### 配置说明

项目配置文件位于 `configs/config.yaml` 中，包含以下主要配置项：

1. **S3 存储**：
```yaml
s3:
     bucket_name: example-bucket
     endpoint: s3.example.com
     access_key: AKIA...
     secret_key: xxx...
     listen_dir: /data/
     cors:
       origins: ["*"]  # CORS 支持
```

2. **Redis 缓存**：
```yaml
redis:
     addr: "192.168.1.100:6379"
     password: "your_password"
     db: 0
```

3**缓存配置**：
```yaml
cache:
  # Redis 缓存配置
  filesListRefreshIntervalSec: 300  # 文件列表刷新间隔（秒）
  
  # 本地热点缓存配置
  local_cache:
    enabled: true                    # 是否启用本地缓存
    size: 1000                       # 本地缓存容量（条目数）
    mode: "lru"                      # 缓存模式：lru（最近最少使用）或 lfu（最不经常使用）
    hot_data_refresh_sec: 30         # 热点数据刷新间隔（秒）
    hot_data_threshold: 10           # 热点数据访问阈值
```

**注意**：在配置文件中，`local_cache` 配置项应该位于 `cache` 配置块内，而不是独立的顶级配置项。

---

#### 部署与运行

##### 测试运行

1. **运行命令**：
    - 启动 API 服务：
```shell script
go run cmd/api/main.go
```

- 启动同步服务：
```shell script
go run cmd/sync/main.go
```

2. **日志记录**：
    - 日志使用 Zap 记录，默认在控制台输出。

##### 生产部署

###### Linux

 1. 脚本构建

    ```
    前置需求：
    ​		Golang version = 1.24.6
    ​		Node.js = 22.20.0 LTS
    ​		pnpm >= 10.18.3
    ​		Zip 3.0
    ```
    
    编辑 `web/.env` `web/.env.development` `web/.env.production` 内容确保符合生产环境
    
    ```bash
    cd <project-name>
    chmod +x ./release-build.sh
    ./release-build.sh
    ```

 2. 将 Release Zip上传到生产服务器，例如 `linux-release-25.10.15.zip`

 3. `cp configs/config.yaml.example configs/config.yaml` 后编辑 `config.yaml` 确保符合生产环境

 4. 复制service文件到系统目录并启用服务

    ```bash
    sudo cp systemctl-files/* /etc/systemd/system/
    sudo vim /etc/systemd/system/yoosf-api.service
    sudo systemctl daemon-reload
    sudo systemctl enable yoosf-api.service
    sudo systemctl start yoosf-api.service
    ```

 5. 运行 `journalctl -u yoosf-api.service -f` 实时查看日志信息

---

#### 注意事项

- **安全性**：生产环境需配置 HTTPS，避免敏感信息泄露。
- **性能优化**：
    - 合理设置 S3 存储的缓存刷新间隔。
    - 控制最大并行同步任务数，避免过载。
- **错误处理**：实现了统一的错误日志记录，并将详细返回调整为用户友好。
- **Redis 依赖**：服务启动前必须确保 Redis 连接成功，否则服务将无法启动。
- **缓存策略**：文件列表缓存 TTL 统一为 5 分钟，刷新阈值为 1 分钟，确保数据新鲜度同时避免频繁刷新。
