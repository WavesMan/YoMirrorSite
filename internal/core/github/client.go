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

const (
	DefaultBaseURL = "https://api.github.com"
	DefaultPerPage = 100
	DefaultTimeout = 30 * time.Second
	MaxRetries     = 3
	MaxPages       = 50
	RateLimitBuffer = 10
)

type Client struct {
	httpClient  *http.Client
	token       string
	baseURL     string
	rateLimiter *RateLimiter
	etagCache   map[string]string
	etagMu      sync.RWMutex
}

type RateLimiter struct {
	limit     int64
	remaining atomic.Int64
	resetAt   atomic.Int64
	mu        sync.Mutex
}

type RepoInfo struct {
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Homepage      string `json:"homepage"`
	Stars         int    `json:"stargazers_count"`
	License       string `json:"-"`
	DefaultBranch string `json:"default_branch"`
}

type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

type Asset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	DownloadURL string `json:"browser_download_url"`
	APIURL      string `json:"url"`
	State       string `json:"state"`
}

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

func NewClientFromConfig(cfg *config.SyncConfig, httpClient *http.Client) *Client {
	return NewClient(httpClient, cfg.GitHubToken)
}

func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{}
	rl.limit = 60
	rl.remaining.Store(60)
	rl.resetAt.Store(time.Now().Add(time.Hour).Unix())
	return rl
}

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

func (c *Client) ListAllReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	var allReleases []Release
	page := 1
	for {
		releases, err := c.ListReleases(ctx, owner, repo, page, DefaultPerPage)
		if err != nil {
			if len(allReleases) > 0 {
				break
			}
			return nil, err
		}
		allReleases = append(allReleases, releases...)
		if len(releases) < DefaultPerPage {
			break
		}
		page++
		if page > MaxPages {
			break
		}
	}
	return allReleases, nil
}

func (c *Client) DownloadAsset(ctx context.Context, downloadURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}
	c.setHeaders(req)
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
		req, _ = http.NewRequestWithContext(ctx, "GET", redirectURL, nil)
		return c.httpClient.Do(req)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.handleError(resp, "下载资产")
	}
	return resp, nil
}

func (c *Client) GetReadme(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/readme", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 README 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", c.handleError(resp, "获取 README")
	}
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 README 响应失败: %w", err)
	}
	if result.Encoding == "base64" {
		decoded, err := decodeBase64(result.Content)
		if err != nil {
			return "", fmt.Errorf("解码 README base64 失败: %w", err)
		}
		return string(decoded), nil
	}
	return result.Content, nil
}

func decodeBase64(content string) ([]byte, error) {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, content)
	return base64Decode(clean)
}

func (c *Client) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待被取消: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	c.setHeaders(req)
	c.etagMu.RLock()
	if etag, ok := c.etagCache[url]; ok {
		req.Header.Set("If-None-Match", etag)
	}
	c.etagMu.RUnlock()
	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
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
		c.updateRateLimit(resp)
		if etag := resp.Header.Get("ETag"); etag != "" {
			c.etagMu.Lock()
			c.etagCache[url] = etag
			c.etagMu.Unlock()
		}
		if resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			return nil, fmt.Errorf("内容未修改 (304)")
		}
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
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("请求失败（已重试 %d 次）: %w", MaxRetries, lastErr)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "YoMirrorSite/1.0")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

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

func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
	if after := resp.Header.Get("Retry-After"); after != "" {
		if sec, err := strconv.Atoi(after); err == nil {
			return time.Duration(sec) * time.Second
		}
	}
	return 60 * time.Second
}

func (c *Client) handleError(resp *http.Response, context string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("%s: HTTP %d - %s", context, resp.StatusCode, string(body))
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		remaining := rl.remaining.Load()
		if remaining > RateLimitBuffer {
			rl.remaining.Store(remaining - 1)
			return nil
		}
		now := time.Now().Unix()
		resetAt := rl.resetAt.Load()
		waitSeconds := resetAt - now + 5
		if waitSeconds <= 0 {
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

func base64Decode(s string) ([]byte, error) {
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