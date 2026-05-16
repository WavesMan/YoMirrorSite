package util

import (
	"crypto/tls"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var RedisClient *redis.Client

// InitRedisClient 初始化 Redis 客户端
func InitRedisClient(addr, password string, db int) {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		TLSConfig:    &tls.Config{InsecureSkipVerify: true},
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		Error("Failed to connect to Redis", zap.String("addr", addr), zap.Error(err))
	} else {
		Info("Redis connected successfully", zap.String("addr", addr))
	}
}

// CloseRedisClient 关闭 Redis 客户端
func CloseRedisClient() {
	if RedisClient != nil {
		err := RedisClient.Close()
		if err != nil {
			Error("Failed to close Redis client", zap.Error(err))
		} else {
			Info("Redis client closed")
		}
	}
}

// ToJSON 将对象转换为 JSON 字符串
func ToJSON(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从 JSON 字符串解析对象
func FromJSON(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

// SetJSON 设置 JSON 格式缓存
func SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := ToJSON(value)
	if err != nil {
		return err
	}
	return RedisClient.Set(ctx, key, jsonData, expiration).Err()
}

// GetJSON 获取 JSON 格式缓存
func GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := RedisClient.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := FromJSON(data, dest); err != nil {
		return false, err
	}
	return true, nil
}

// Delete 删除缓存
func Delete(ctx context.Context, key string) error {
	return RedisClient.Del(ctx, key).Err()
}

// Exists 检查缓存是否存在
func Exists(ctx context.Context, key string) (bool, error) {
	count, err := RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetWithExpiration 设置缓存并指定过期时间
func SetWithExpiration(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return RedisClient.Set(ctx, key, value, expiration).Err()
}

// Get 获取缓存
func Get(ctx context.Context, key string) (string, error) {
	return RedisClient.Get(ctx, key).Result()
}

// HSet 设置哈希字段
func HSet(ctx context.Context, key string, values ...interface{}) error {
	return RedisClient.HSet(ctx, key, values...).Err()
}

// HGet 获取哈希字段
func HGet(ctx context.Context, key, field string) (string, error) {
	return RedisClient.HGet(ctx, key, field).Result()
}

// HGetAll 获取所有哈希字段
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return RedisClient.HGetAll(ctx, key).Result()
}

// Keys 根据模式获取所有键
func Keys(ctx context.Context, pattern string) ([]string, error) {
	return RedisClient.Keys(ctx, pattern).Result()
}

// FlushDB 清空当前数据库
func FlushDB(ctx context.Context) error {
	return RedisClient.FlushDB(ctx).Err()
}

// HealthCheck 健康检查
func HealthCheck(ctx context.Context) error {
	_, err := RedisClient.Ping(ctx).Result()
	return err
}

// AcquireLock 获取分布式锁
func AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	result, err := RedisClient.SetNX(ctx, lockKey, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

// ReleaseLock 释放分布式锁
func ReleaseLock(ctx context.Context, lockKey string) error {
	_, err := RedisClient.Del(ctx, lockKey).Result()
	return err
}

// TryLock 尝试获取锁（非阻塞）
func TryLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	return AcquireLock(ctx, lockKey, ttl)
}

// GetLockTTL 获取锁的剩余时间
func GetLockTTL(ctx context.Context, lockKey string) (time.Duration, error) {
	return RedisClient.TTL(ctx, lockKey).Result()
}

// GetTTL 获取键的剩余时间
func GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return RedisClient.TTL(ctx, key).Result()
}

// LockStats 锁统计信息
type LockStats struct {
	LockKey     string        `json:"lock_key"`
	IsLocked    bool          `json:"is_locked"`
	RemainingTT time.Duration `json:"remaining_ttl"`
}

// GetLockStats 获取锁状态信息
func GetLockStats(ctx context.Context, lockKey string) (*LockStats, error) {
	ttl, err := GetLockTTL(ctx, lockKey)
	if err != nil {
		return nil, err
	}

	isLocked := ttl > 0
	return &LockStats{
		LockKey:     lockKey,
		IsLocked:    isLocked,
		RemainingTT: ttl,
	}, nil
}

// CacheStats 缓存统计信息
type CacheStats struct {
	TotalKeys    int64   `json:"total_keys"`
	MemoryUsage  int64   `json:"memory_usage"`
	HitRate      float64 `json:"hit_rate"`
	MissRate     float64 `json:"miss_rate"`
	KeyspaceHits int64   `json:"keyspace_hits"`
	KeyspaceMiss int64   `json:"keyspace_misses"`
}

// DownloadURLCacheStats 下载URL缓存统计
type DownloadURLCacheStats struct {
	TotalCachedURLs int64   `json:"total_cached_urls"`
	HitCount        int64   `json:"hit_count"`
	MissCount       int64   `json:"miss_count"`
	HitRate         float64 `json:"hit_rate"`
	QueueSize       int     `json:"queue_size"`
	ActiveWorkers   int     `json:"active_workers"`
}

// SearchCacheStats 搜索缓存统计
type SearchCacheStats struct {
	TotalSearches   int64   `json:"total_searches"`
	CacheHitCount   int64   `json:"cache_hit_count"`
	CacheMissCount  int64   `json:"cache_miss_count"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	S3FallbackCount int64   `json:"s3_fallback_count"`
}

// GetCacheStats 获取缓存统计信息
func GetCacheStats(ctx context.Context) (*CacheStats, error) {
	info, err := RedisClient.Info(ctx, "stats", "memory").Result()
	if err != nil {
		return nil, err
	}

	stats := &CacheStats{}

	// 解析Redis INFO命令输出
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "used_memory":
			stats.MemoryUsage, _ = strconv.ParseInt(value, 10, 64)
		case "keyspace_hits":
			stats.KeyspaceHits, _ = strconv.ParseInt(value, 10, 64)
		case "keyspace_misses":
			stats.KeyspaceMiss, _ = strconv.ParseInt(value, 10, 64)
		}
	}

	// 计算命中率
	total := stats.KeyspaceHits + stats.KeyspaceMiss
	if total > 0 {
		stats.HitRate = float64(stats.KeyspaceHits) / float64(total) * 100
		stats.MissRate = float64(stats.KeyspaceMiss) / float64(total) * 100
	}

	// 获取总键数
	keys, err := RedisClient.Keys(ctx, "*").Result()
	if err == nil {
		stats.TotalKeys = int64(len(keys))
	}

	return stats, nil
}

// IncrementCounter 增加计数器
func IncrementCounter(ctx context.Context, key string) (int64, error) {
	return RedisClient.Incr(ctx, key).Result()
}

// GetCounter 获取计数器值
func GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := RedisClient.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// ResetCounter 重置计数器
func ResetCounter(ctx context.Context, key string) error {
	return RedisClient.Del(ctx, key).Err()
}