// GitHub API v3 REST 客户端
// 纯 net/http 实现，不引入第三方 GitHub SDK
// 复用项目已有的 proxy.go 代理配置，支持 ETag 条件请求和速率限制

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"yomirrorsite/internal/config"
)

// ============================================================
// 常量定义
// ============================================================

const (
	// DefaultBaseURL GitHub API v3 基础地址
	DefaultBaseURL = "https://api.github.com"
	// DefaultPerPage 每页默认结果数
	DefaultPerPage = 100
	// DefaultTimeout HTTP 请求默认超时
	DefaultTimeout = 30 * time.Second
	// MaxRetries 最大重试次数
	MaxRetries = 3
	// MaxPages 最大翻页数（防止无限循环导致 OOM）
	MaxPages = 50
	// RateLimitBuffer 速率限制缓冲区（提前停止请求）
	RateLimitBuffer = 10
)

// ============================================================
// 数据结构
// ============================================================

// Client GitHub API 客户端
// 基于 net/http，支持 Token 认证、ETag 缓存、速率限制
type Client struct {
	httpClient  *http.Client       // HTTP 客户端（复用 proxy 配置）
	token       string             // GitHub Personal Access Token（可选）
	baseURL     string             // API 基础地址
	rateLimiter *RateLimiter       // 速率控制器
	etagCache   map[string]string  // ETag 缓存（URL → ETag）
	etagMu      sync.RWMutex       // ETag 缓存锁
}

// RateLimiter GitHub API 速率限制控制器
type RateLimiter struct {
	limit     int64         // 每小时最大请求数
	remaining atomic.Int64  // 剩余请求数
	resetAt   atomic.Int64  // 重置时间戳（Unix 秒）
	mu        sync.Mutex
}

// RepoInfo GitHub 仓库基本信息
type RepoInfo struct {
	FullName    string `json:"full_name"`    // "owner/repo"
	Description string `json:"description"`  // 仓库描述
	Homepage    string `json:"homepage"`     // 项目主页
	Stars       int    `json:"stargazers_count"` // 星数
	License     string `json:"-"`            // 许可证（从 License 嵌套对象提取）
	DefaultBranch string `json:"default_branch"` // 默认分支
}

// Release GitHub Release 信息
type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`     // Git tag
	Name        string    `json:"name"`         // 发布名称
	Body        string    `json:"body"`         // Release Notes（Markdown）
	Prerelease  bool      `json:"prerelease"`   // 是否预发布
	PublishedAt time.Time `json:"published_at"` // 发布时间
	Assets      []Asset   `json:"assets"`       // 资产列表
}

// Asset GitHub Release 资产（下载文件）
type Asset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`         // 文件名
	Size        int64  `json:"size"`         // 文件大小（字节）
	ContentType string `json:"content_type"` // MIME 类型
	DownloadURL string `json:"browser_download_url"` // 实际下载地址
	APIURL      string `json:"url"`          // API 地址（用于条件请求）
	State       string `json:"state"`        // "uploaded"
}

// ============================================================
// 客户端初始化
// ============================================================

// NewClient 创建 GitHub API 客户端
// httpClient 可传入复用项目代理配置的 *http.Client
func NewClient(httpClient *http.Client, token string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	c := &Client{
		httpClient:  httpClient,
		token:       token,
		baseURL:     DefaultBaseURL,
		rateLimiter: NewRateLimiter(),
		etagCache:   make(map[string]string),
	}
	return c
}

// NewClientFromConfig 从项目配置创建客户端
// 复用 proxy.go 中配置的 HTTP 客户端
func NewClientFromConfig(cfg *config.SyncConfig, httpClient *http.Client) *Client {
	return NewClient(httpClient, cfg.GitHubToken)
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{}
	rl.limit = 60 // 未认证时默认 60 req/h
	rl.remaining.Store(60)
	rl.resetAt.Store(time.Now().Add(time.Hour).Unix())
	return rl
}

// ============================================================
// 仓库信息
// ============================================================

// GetRepoInfo 获取 GitHub 仓库基本信息
// GET /repos/{owner}/{repo}
func (c *Client) GetRepoInfo(ctx context.Context, owner, repo string) (*RepoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, owner, repo)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("获取仓库信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp, "获取仓库信息")
	}

	var info RepoInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("解析仓库信息失败: %w", err)
	}

	return &info, nil
}

// ============================================================
// Release 列表
// ============================================================

// ListReleases 分页获取仓库的 Release 列表
// GET /repos/{owner}/{repo}/releases?page={page}&per_page={perPage}
// 返回按发布时间倒序排列的 Release 列表
func (c *Client) ListReleases(ctx context.Context, owner, repo string, page, perPage int) ([]Release, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = DefaultPerPage
	}
	if page <= 0 {
		page = 1
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases?page=%d&per_page=%d",
		c.baseURL, owner, repo, page, perPage)

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("获取 Release 列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp, "获取 Release 列表")
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析 Release 列表失败: %w", err)
	}

	return releases, nil
}

// ListAllReleases 获取所有 Release（自动翻页，从最新开始）
// 使用 ETag 条件请求减少 API 消耗
func (c *Client) ListAllReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	var allReleases []Release
	page := 1

	for {
		releases, err := c.ListReleases(ctx, owner, repo, page, DefaultPerPage)
		if err != nil {
			// 如果已经获取了一批，部分失败可以接受
			if len(allReleases) > 0 {
				break
			}
			return nil, err
		}

		allReleases = append(allReleases, releases...)

		// 返回数量少于 perPage 说明已是最后一页
		if len(releases) < DefaultPerPage {
			break
		}
		page++
		// 防止无限翻页（恶意仓库或有问题的分页）
		if page > MaxPages {
			break
		}
	}

	return allReleases, nil
}

// ============================================================
// 资产下载
// ============================================================

// DownloadAsset 下载 Release 资产
// 返回响应体（调用方负责 defer Close），响应体为文件流
// 注意：GitHub 资产 URL 会 302 重定向到实际下载地址
func (c *Client) DownloadAsset(ctx context.Context, downloadURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}

	c.setHeaders(req)

	// 不跟随重定向，以便获取实际下载地址
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载资产失败: %w", err)
	}

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		redirectURL := resp.Header.Get("Location")
		resp.Body.Close()
		if redirectURL == "" {
			return nil, fmt.Errorf("重定向 URL 为空")
		}
		// 跟随重定向
		req, _ = http.NewRequestWithContext(ctx, "GET", redirectURL, nil)
		return c.httpClient.Do(req)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleError(resp, "下载资产")
	}

	return resp, nil
}

// ============================================================
// README 获取
// ============================================================

// GetReadme 获取仓库 README.md 内容
// GET /repos/{owner}/{repo}/readme
func (c *Client) GetReadme(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/readme", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	c.setHeaders(req)
	// README API 返回 base64 编码的内容
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 README 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // 没有 README 不算错误
	}
	if resp.StatusCode != http.StatusOK {
		return "", c.handleError(resp, "获取 README")
	}

	// 解析响应
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 README 响应失败: %w", err)
	}

	if result.Encoding == "base64" {
		// 简单 base64 解码（使用标准库）
		decoded, err := decodeBase64(result.Content)
		if err != nil {
			return "", fmt.Errorf("解码 README base64 失败: %w", err)
		}
		return string(decoded), nil
	}

	return result.Content, nil
}

// decodeBase64 解码 base64 内容（处理 GitHub 返回的换行字符）
func decodeBase64(content string) ([]byte, error) {
	// 移除换行符
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, content)
	return base64Decode(clean)
}

// ============================================================
// HTTP 请求核心
// ============================================================

// doRequest 执行 HTTP 请求（带速率限制、ETag、重试）
func (c *Client) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	// 等待速率限制
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待被取消: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setHeaders(req)

	// 设置 ETag 缓存头
	c.etagMu.RLock()
	if etag, ok := c.etagCache[url]; ok {
		req.Header.Set("If-None-Match", etag)
	}
	c.etagMu.RUnlock()

	// 带重试的执行
	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避：1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// 更新速率限制
		c.updateRateLimit(resp)

		// 缓存 ETag
		if etag := resp.Header.Get("ETag"); etag != "" {
			c.etagMu.Lock()
			c.etagCache[url] = etag
			c.etagMu.Unlock()
		}

		// 304 Not Modified 表示内容未变
		if resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			return nil, fmt.Errorf("内容未修改 (304)")
		}

		// 429 Too Many Requests 需要等待重试
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := c.parseRetryAfter(resp)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryAfter):
			}
			continue
		}

		// 5xx 服务端错误重试
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", MaxRetries, lastErr)
}

// setHeaders 设置通用请求头
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "YoMirrorSite/1.0")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// updateRateLimit 更新速率限制信息
func (c *Client) updateRateLimit(resp *http.Response) {
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.ParseInt(remaining, 10, 64); err == nil {
			c.rateLimiter.remaining.Store(val)
		}
	}
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		if val, err := strconv.ParseInt(limit, 10, 64); err == nil {
			c.rateLimiter.limit = val
		}
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			c.rateLimiter.resetAt.Store(val)
		}
	}
}

// parseRetryAfter 解析 Retry-After 头
func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
	if after := resp.Header.Get("Retry-After"); after != "" {
		if sec, err := strconv.Atoi(after); err == nil {
			return time.Duration(sec) * time.Second
		}
	}
	return 60 * time.Second // 默认等 60 秒
}

// handleError 处理错误响应
func (c *Client) handleError(resp *http.Response, context string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("%s: HTTP %d - %s", context, resp.StatusCode, string(body))
}

// ============================================================
// 速率限制器方法
// ============================================================

// Wait 等待直到可以发送请求
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		remaining := rl.remaining.Load()
		if remaining > RateLimitBuffer {
			rl.remaining.Store(remaining - 1)
			return nil
		}

		// 计算等待时间
		now := time.Now().Unix()
		resetAt := rl.resetAt.Load()
		waitSeconds := resetAt - now + 5 // 多等 5 秒确保安全

		if waitSeconds <= 0 {
			// 已重置
			rl.remaining.Store(rl.limit)
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(waitSeconds) * time.Second):
			rl.remaining.Store(rl.limit)
		}
	}
}

// ============================================================
// 辅助函数
// ============================================================

// base64Decode 简单 base64 解码（避免外部依赖）
func base64Decode(s string) ([]byte, error) {
	// 标准 base64 解码表
	const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	decodeMap := make(map[byte]byte)
	for i := 0; i < len(base64Table); i++ {
		decodeMap[base64Table[i]] = byte(i)
	}

	var result []byte
	for i := 0; i < len(s); i += 4 {
		if i+3 >= len(s) {
			break
		}
		a, ok1 := decodeMap[s[i]]
		b, ok2 := decodeMap[s[i+1]]
		c := byte(0)
		d := byte(0)
		ok3 := s[i+2] != '=' && s[i+2] != 0
		ok4 := s[i+3] != '=' && s[i+3] != 0

		if !ok1 || !ok2 {
			continue
		}

		if ok3 {
			c = decodeMap[s[i+2]]
		}
		if ok4 {
			d = decodeMap[s[i+3]]
		}

		val := uint32(a)<<18 | uint32(b)<<12 | uint32(c)<<6 | uint32(d)
		result = append(result, byte(val>>16))
		if ok3 {
			result = append(result, byte(val>>8))
		}
		if ok4 {
			result = append(result, byte(val))
		}
	}
	return result, nil
}