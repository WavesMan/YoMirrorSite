package util

import (
	"container/heap"
	"sync"
)

// CacheItem 表示 LFU 缓存中的一项
type CacheItem struct {
	Key       string
	Value     any
	Frequency int
	Index     int
}

// LFUCache 实现了一个简单的 LFU 缓存
type LFUCache struct {
	sync.Mutex
	Capacity int
	ItemMap  map[string]*CacheItem
	FreqHeap FreqHeap
}

// NewLFUCache 创建一个指定容量的 LFU 缓存
func NewLFUCache(capacity int) *LFUCache {
	cache := &LFUCache{
		Capacity: capacity,
		ItemMap:  make(map[string]*CacheItem),
		FreqHeap: make(FreqHeap, 0),
	}
	heap.Init(&cache.FreqHeap)
	return cache
}

// Get 返回缓存中的值，如果不存在则返回 nil
func (c *LFUCache) Get(key string) (any, bool) {
	c.Lock()
	defer c.Unlock()

	if item, exists := c.ItemMap[key]; exists {
		// 增加使用频率
		item.Frequency++
		heap.Fix(&c.FreqHeap, item.Index)
		return item.Value, true
	}
	return nil, false
}

// Set 插入一个新的键值对到缓存中，如果超过容量，则淘汰频率最低的键
func (c *LFUCache) Set(key string, value any) {
	c.Lock()
	defer c.Unlock()

	if item, exists := c.ItemMap[key]; exists {
		// 如果键已存在，仅更新值和频率
		item.Value = value
		item.Frequency++
		heap.Fix(&c.FreqHeap, item.Index)
		return
	}

	if len(c.ItemMap) >= c.Capacity {
		// 超出容量，移除频率最低的键
		evicted := heap.Pop(&c.FreqHeap).(*CacheItem)
		delete(c.ItemMap, evicted.Key)
	}

	// 添加新键
	item := &CacheItem{
		Key:       key,
		Value:     value,
		Frequency: 1,
	}
	heap.Push(&c.FreqHeap, item)
	c.ItemMap[key] = item
}

// Delete 删除指定的键
func (c *LFUCache) Delete(key string) bool {
	c.Lock()
	defer c.Unlock()

	if item, exists := c.ItemMap[key]; exists {
		heap.Remove(&c.FreqHeap, item.Index)
		delete(c.ItemMap, key)
		return true
	}
	return false
}

// Remove 删除指定的键（与 Delete 相同，为了接口兼容性）
func (c *LFUCache) Remove(key string) bool {
	return c.Delete(key)
}

// Contains 检查键是否存在
func (c *LFUCache) Contains(key string) bool {
	c.Lock()
	defer c.Unlock()
	_, exists := c.ItemMap[key]
	return exists
}

// Len 返回缓存中的项目数量
func (c *LFUCache) Len() int {
	c.Lock()
	defer c.Unlock()
	return len(c.ItemMap)
}

// Clear 清空缓存
func (c *LFUCache) Clear() {
	c.Lock()
	defer c.Unlock()
	c.ItemMap = make(map[string]*CacheItem)
	c.FreqHeap = make(FreqHeap, 0)
	heap.Init(&c.FreqHeap)
}

// Keys 返回所有键的列表
func (c *LFUCache) Keys() []string {
	c.Lock()
	defer c.Unlock()
	keys := make([]string, 0, len(c.ItemMap))
	for key := range c.ItemMap {
		keys = append(keys, key)
	}
	return keys
}

// FreqHeap 是一个基于频率的最小堆
type FreqHeap []*CacheItem

func (h FreqHeap) Len() int           { return len(h) }
func (h FreqHeap) Less(i, j int) bool { return h[i].Frequency < h[j].Frequency }
func (h FreqHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}

func (h *FreqHeap) Push(x any) {
	item := x.(*CacheItem)
	item.Index = len(*h)
	*h = append(*h, item)
}

func (h *FreqHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	item.Index = -1 // 避免被再次引用
	*h = old[0 : n-1]
	return item
}
