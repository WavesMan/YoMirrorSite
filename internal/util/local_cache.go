package util

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
)

// CacheManager 本地缓存管理器
type CacheManager struct {
	LRUCache *lru.Cache[string, any]
	LFUCache *LFUCache
	mutex    sync.RWMutex
	mode     string // "lru" 或 "lfu"
	stats    *LocalCacheStats
}

// LocalCacheStats 本地缓存统计信息
type LocalCacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	HitRate     float64   `json:"hit_rate"`
	Size        int       `json:"size"`
	LastUpdated time.Time `json:"last_updated"`
	mu          sync.RWMutex
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(size int, mode string) *CacheManager {
	stats := &LocalCacheStats{
		LastUpdated: time.Now(),
	}

	if mode == "lru" {
		cache, err := lru.New[string, any](size)
		if err != nil {
			panic(err)
		}
		return &CacheManager{
			LRUCache: cache,
			mode:     mode,
			stats:    stats,
		}
	} else if mode == "lfu" {
		return &CacheManager{
			LFUCache: NewLFUCache(size),
			mode:     mode,
			stats:    stats,
		}
	} else {
		panic("Invalid cache mode. Only 'lru' or 'lfu' are supported.")
	}
}

// Set 设置键值对
func (m *CacheManager) Set(key string, value any) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mode == "lru" {
		m.LRUCache.Add(key, value)
	} else if m.mode == "lfu" {
		m.LFUCache.Set(key, value)
	}

	m.updateStats()
}

// Get 获取值
func (m *CacheManager) Get(key string) (any, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var value any
	var found bool

	if m.mode == "lru" {
		value, found = m.LRUCache.Get(key)
	} else if m.mode == "lfu" {
		value, found = m.LFUCache.Get(key)
	}

	if found {
		m.stats.mu.Lock()
		m.stats.Hits++
		m.stats.LastUpdated = time.Now()
		m.stats.mu.Unlock()
	} else {
		m.stats.mu.Lock()
		m.stats.Misses++
		m.stats.LastUpdated = time.Now()
		m.stats.mu.Unlock()
	}

	m.updateStats()
	return value, found
}

// Delete 删除键
func (m *CacheManager) Delete(key string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var removed bool
	if m.mode == "lru" {
		removed = m.LRUCache.Remove(key)
	} else if m.mode == "lfu" {
		removed = m.LFUCache.Remove(key)
	}

	m.updateStats()
	return removed
}

// Contains 检查键是否存在
func (m *CacheManager) Contains(key string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.mode == "lru" {
		return m.LRUCache.Contains(key)
	} else if m.mode == "lfu" {
		return m.LFUCache.Contains(key)
	}
	return false
}

// Len 获取缓存大小
func (m *CacheManager) Len() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.mode == "lru" {
		return m.LRUCache.Len()
	} else if m.mode == "lfu" {
		return m.LFUCache.Len()
	}
	return 0
}

// Clear 清空缓存
func (m *CacheManager) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mode == "lru" {
		m.LRUCache.Purge()
	} else if m.mode == "lfu" {
		m.LFUCache.Clear()
	}

	m.stats.mu.Lock()
	m.stats.Hits = 0
	m.stats.Misses = 0
	m.stats.HitRate = 0
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()
}

// SetMode 设置缓存模式（LRU 或 LFU）
func (m *CacheManager) SetMode(mode string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if mode != m.mode && (mode == "lru" || mode == "lfu") {
		m.mode = mode
		Info("Cache mode changed", zap.String("mode", mode))
	}
}

// GetMode 获取当前缓存模式
func (m *CacheManager) GetMode() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.mode
}

// GetStats 获取缓存统计信息
func (m *CacheManager) GetStats() *LocalCacheStats {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	// 计算命中率
	total := m.stats.Hits + m.stats.Misses
	if total > 0 {
		m.stats.HitRate = float64(m.stats.Hits) / float64(total) * 100
	}

	// 获取当前大小
	m.stats.Size = m.Len()

	return &LocalCacheStats{
		Hits:        m.stats.Hits,
		Misses:      m.stats.Misses,
		HitRate:     m.stats.HitRate,
		Size:        m.stats.Size,
		LastUpdated: m.stats.LastUpdated,
	}
}

// updateStats 更新统计信息
func (m *CacheManager) updateStats() {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()

	// 计算命中率
	total := m.stats.Hits + m.stats.Misses
	if total > 0 {
		m.stats.HitRate = float64(m.stats.Hits) / float64(total) * 100
	}

	// 获取当前大小
	if m.mode == "lru" {
		m.stats.Size = m.LRUCache.Len()
	} else if m.mode == "lfu" {
		m.stats.Size = m.LFUCache.Len()
	}
}

// Keys 获取所有键
func (m *CacheManager) Keys() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.mode == "lru" {
		return m.LRUCache.Keys()
	} else if m.mode == "lfu" {
		return m.LFUCache.Keys()
	}
	return []string{}
}

// HotDataManager 热点数据管理器
type HotDataManager struct {
	cacheManager *CacheManager
	accessCount  map[string]int
	mu           sync.RWMutex
	threshold    int
}

// NewHotDataManager 创建热点数据管理器
func NewHotDataManager(cacheManager *CacheManager, threshold int) *HotDataManager {
	return &HotDataManager{
		cacheManager: cacheManager,
		accessCount:  make(map[string]int),
		threshold:    threshold,
	}
}

// RecordAccess 记录访问
func (h *HotDataManager) RecordAccess(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.accessCount[key]++
	if h.accessCount[key] >= h.threshold {
		// 如果访问次数达到阈值，标记为热点数据
		Debug("Key marked as hot data", zap.String("key", key), zap.Int("access_count", h.accessCount[key]))
	}
}

// GetHotKeys 获取热点键
func (h *HotDataManager) GetHotKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var hotKeys []string
	for key, count := range h.accessCount {
		if count >= h.threshold {
			hotKeys = append(hotKeys, key)
		}
	}
	return hotKeys
}

// ResetStats 重置统计
func (h *HotDataManager) ResetStats() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.accessCount = make(map[string]int)
	Info("Hot data statistics reset")
}
