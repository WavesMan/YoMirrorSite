package util

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ProxyTestURL 代理可用性检测地址
// 默认使用本地 MinIO 健康检查端点，避免依赖外网且不泄露检测意图
// 可通过环境变量 PROXY_TEST_URL 覆盖
var ProxyTestURL = getEnvDefault("PROXY_TEST_URL", "http://localhost:9000/minio/health/live")

// ProxyConfig 代理配置
type ProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

// getEnvDefault 获取环境变量，不存在时返回默认值
func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetSystemProxy 获取系统代理配置
func GetSystemProxy() *ProxyConfig {
	config := &ProxyConfig{}

	// 从环境变量获取代理设置
	config.HTTPProxy = os.Getenv("HTTP_PROXY")
	config.HTTPSProxy = os.Getenv("HTTPS_PROXY")
	config.NoProxy = os.Getenv("NO_PROXY")

	// 如果HTTPS代理未设置但HTTP代理已设置，使用HTTP代理
	if config.HTTPSProxy == "" && config.HTTPProxy != "" {
		config.HTTPSProxy = config.HTTPProxy
	}

	// 在Windows上，尝试从注册表获取代理设置
	if runtime.GOOS == "windows" {
		config = getWindowsProxy(config)
	}

	// 在Linux上，尝试检测系统代理
	if runtime.GOOS == "linux" {
		config = getLinuxProxy(config)
	}

	return config
}

// getWindowsProxy 获取Windows系统代理设置
func getWindowsProxy(defaultConfig *ProxyConfig) *ProxyConfig {
	// 首先检查环境变量
	if defaultConfig.HTTPProxy != "" || defaultConfig.HTTPSProxy != "" {
		return defaultConfig
	}

	// 检测常见的代理软件
	config := detectCommonProxies(defaultConfig)
	if config.HTTPProxy != "" || config.HTTPSProxy != "" {
		return config
	}

	// 如果还没有找到代理，尝试检测系统代理
	return detectSystemProxy(defaultConfig)
}

// detectCommonProxies 检测常见代理软件
func detectCommonProxies(defaultConfig *ProxyConfig) *ProxyConfig {
	config := &ProxyConfig{
		HTTPProxy:  defaultConfig.HTTPProxy,
		HTTPSProxy: defaultConfig.HTTPSProxy,
		NoProxy:    defaultConfig.NoProxy,
	}

	// 检测 Clash 代理（默认端口 7890）
	if isProxyAvailable("http://127.0.0.1:7890") {
		Info("Detected Clash proxy on port 7890")
		config.HTTPProxy = "http://127.0.0.1:7890"
		config.HTTPSProxy = "http://127.0.0.1:7890"
		return config
	}

	// 检测其他常见代理端口
	commonPorts := []string{"1080", "1081", "1087", "1088", "1089", "8080", "8888", "9050"}
	for _, port := range commonPorts {
		proxyURL := "http://127.0.0.1:" + port
		if isProxyAvailable(proxyURL) {
			Info("Detected proxy on port", zap.String("port", port))
			config.HTTPProxy = proxyURL
			config.HTTPSProxy = proxyURL
			return config
		}
	}

	return config
}

// detectSystemProxy 检测系统代理设置
func detectSystemProxy(defaultConfig *ProxyConfig) *ProxyConfig {
	config := &ProxyConfig{
		HTTPProxy:  defaultConfig.HTTPProxy,
		HTTPSProxy: defaultConfig.HTTPSProxy,
		NoProxy:    defaultConfig.NoProxy,
	}

	// 在Windows上，可以尝试从注册表读取系统代理设置
	// 这里可以添加注册表读取代码
	// 目前主要依赖环境变量和常见代理检测

	return config
}

// isProxyAvailable 检查代理是否可用
func isProxyAvailable(proxyURL string) bool {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return false
	}

	// 创建测试客户端
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
		},
		Timeout: 3 * time.Second,
	}

	// 测试连接
	resp, err := client.Get(ProxyTestURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 如果返回204状态码，说明代理可用
	return resp.StatusCode == http.StatusNoContent
}

// CreateProxyTransport 创建带代理的HTTP传输
func CreateProxyTransport() *http.Transport {
	proxyConfig := GetSystemProxy()
	transport := &http.Transport{}

	// 设置代理函数
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		// 检查是否在NO_PROXY列表中
		if isInNoProxy(req.URL.Host, proxyConfig.NoProxy) {
			return nil, nil
		}

		// 根据协议选择代理
		var proxyURL string
		switch req.URL.Scheme {
		case "https":
			proxyURL = proxyConfig.HTTPSProxy
		case "http":
			proxyURL = proxyConfig.HTTPProxy
		default:
			return nil, nil
		}

		if proxyURL == "" {
			return nil, nil
		}

		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			Error("Failed to parse proxy URL", zap.String("proxy_url", proxyURL), zap.Error(err))
			return nil, err
		}

		Info("Using proxy", zap.String("proxy", proxyURL), zap.String("target", req.URL.String()))
		return parsedURL, nil
	}

	return transport
}

// isInNoProxy 检查主机是否在NO_PROXY列表中
func isInNoProxy(host, noProxy string) bool {
	if noProxy == "" {
		return false
	}

	// 支持通配符和域名匹配
	host = strings.ToLower(host)
	noProxyList := strings.Split(strings.ToLower(noProxy), ",")

	for _, pattern := range noProxyList {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// 直接匹配
		if host == pattern {
			return true
		}

		// 通配符匹配
		if strings.HasPrefix(pattern, "*.") {
			domain := pattern[2:]
			if strings.HasSuffix(host, domain) {
				return true
			}
		}

		// 子域名匹配
		if strings.HasPrefix(host, ".") && strings.HasSuffix(host, pattern) {
			return true
		}
	}

	return false
}

// getLinuxProxy 获取Linux系统代理设置
func getLinuxProxy(defaultConfig *ProxyConfig) *ProxyConfig {
	// 首先检查环境变量
	if defaultConfig.HTTPProxy != "" || defaultConfig.HTTPSProxy != "" {
		return defaultConfig
	}

	// 检测常见的代理软件
	config := detectCommonProxies(defaultConfig)
	if config.HTTPProxy != "" || config.HTTPSProxy != "" {
		return config
	}

	// 检测系统代理设置
	config = detectLinuxSystemProxy(defaultConfig)
	if config.HTTPProxy != "" || config.HTTPSProxy != "" {
		return config
	}

	return defaultConfig
}

// detectLinuxSystemProxy 检测Linux系统代理设置
func detectLinuxSystemProxy(defaultConfig *ProxyConfig) *ProxyConfig {
	config := &ProxyConfig{
		HTTPProxy:  defaultConfig.HTTPProxy,
		HTTPSProxy: defaultConfig.HTTPSProxy,
		NoProxy:    defaultConfig.NoProxy,
	}

	// 检测 GNOME 系统代理设置
	if proxy := getGnomeProxy(); proxy != "" {
		Info("Detected GNOME system proxy")
		config.HTTPProxy = proxy
		config.HTTPSProxy = proxy
		return config
	}

	// 检测 KDE 系统代理设置
	if proxy := getKdeProxy(); proxy != "" {
		Info("Detected KDE system proxy")
		config.HTTPProxy = proxy
		config.HTTPSProxy = proxy
		return config
	}

	// 检测常见的Linux代理配置文件
	if proxy := getProxyFromConfigFiles(); proxy != "" {
		Info("Detected proxy from config files")
		config.HTTPProxy = proxy
		config.HTTPSProxy = proxy
		return config
	}

	return config
}

// getGnomeProxy 获取GNOME系统代理设置
func getGnomeProxy() string {
	// 尝试从gsettings获取代理设置
	// 这里可以添加gsettings命令调用
	// 目前主要依赖环境变量和常见代理检测
	return ""
}

// getKdeProxy 获取KDE系统代理设置
func getKdeProxy() string {
	// 尝试从KDE配置获取代理设置
	// 这里可以添加kreadconfig5命令调用
	// 目前主要依赖环境变量和常见代理检测
	return ""
}

// getProxyFromConfigFiles 从配置文件获取代理设置
func getProxyFromConfigFiles() string {
	// 检查常见的代理配置文件
	configFiles := []string{
		os.ExpandEnv("$HOME/.bashrc"),
		os.ExpandEnv("$HOME/.bash_profile"),
		os.ExpandEnv("$HOME/.zshrc"),
		os.ExpandEnv("$HOME/.profile"),
		"/etc/environment",
		"/etc/profile",
	}

	for _, file := range configFiles {
		if proxy := parseProxyFromFile(file); proxy != "" {
			return proxy
		}
	}

	return ""
}

// parseProxyFromFile 从文件解析代理设置
func parseProxyFromFile(filename string) string {
	// 这里可以添加文件解析逻辑
	// 目前主要依赖环境变量和常见代理检测
	return ""
}

// GetProxyInfo 获取代理信息（用于调试）
func GetProxyInfo() string {
	proxyConfig := GetSystemProxy()
	info := fmt.Sprintf("System: %s\n", runtime.GOOS)
	info += fmt.Sprintf("HTTP Proxy: %s\n", proxyConfig.HTTPProxy)
	info += fmt.Sprintf("HTTPS Proxy: %s\n", proxyConfig.HTTPSProxy)
	info += fmt.Sprintf("No Proxy: %s\n", proxyConfig.NoProxy)
	return info
}