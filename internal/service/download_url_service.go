package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// DownloadURLTask 下载URL生成任务
type DownloadURLTask struct {
	Key      string
	Expires  int64
	ResultCh chan *DownloadURLResult
}

// DownloadURLResult 任务结果
type DownloadURLResult struct {
	URL string
	Err error
}

// DownloadURLManager 下载URL管理器
type DownloadURLManager struct {
	taskQueue   chan *DownloadURLTask
	workerCount int
	cache       *FileService
	s3Client    *s3.Client
	mu          sync.RWMutex
	workers     []*worker
}

type worker struct {
	id    int
	queue chan *DownloadURLTask
	quit  chan bool
}

// NewDownloadURLManager 创建下载URL管理器
func NewDownloadURLManager(workerCount int, queueSize int, s3Client *s3.Client, cache *FileService) *DownloadURLManager {
	manager := &DownloadURLManager{
		taskQueue:   make(chan *DownloadURLTask, queueSize),
		workerCount: workerCount,
		cache:       cache,
		s3Client:    s3Client,
		workers:     make([]*worker, workerCount),
	}
	manager.startWorkers()
	return manager
}

// startWorkers 启动工作协程
func (m *DownloadURLManager) startWorkers() {
	for i := 0; i < m.workerCount; i++ {
		worker := &worker{
			id:    i,
			queue: m.taskQueue,
			quit:  make(chan bool),
		}
		m.workers[i] = worker
		go worker.start(m.s3Client, m.cache)
	}
}

// worker 工作协程
func (w *worker) start(s3Client *s3.Client, cache *FileService) {
	util.Info("Download URL worker started", zap.Int("worker_id", w.id))

	for {
		select {
		case task := <-w.queue:
			w.processTask(task, s3Client, cache)
		case <-w.quit:
			util.Info("Download URL worker stopped", zap.Int("worker_id", w.id))
			return
		}
	}
}

// processTask 处理任务
func (w *worker) processTask(task *DownloadURLTask, s3Client *s3.Client, cache *FileService) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 生成预签名URL
	url, err := s3Client.GetObjectURL(ctx, task.Key, task.Expires)
	if err != nil {
		util.Error("Failed to generate download URL in worker",
			zap.String("key", task.Key),
			zap.Int("worker_id", w.id),
			zap.Error(err))
		task.ResultCh <- &DownloadURLResult{URL: "", Err: err}
		return
	}

	// 写入缓存
	cacheKey := fmt.Sprintf("download_url:%s:%d", task.Key, task.Expires)
	err = util.SetJSON(ctx, cacheKey, url, time.Duration(task.Expires)*time.Second)
	if err != nil {
		util.Error("Failed to cache download URL", zap.String("key", cacheKey), zap.Error(err))
	}

	util.Info("Worker generated and cached download URL",
		zap.String("key", task.Key),
		zap.Int("worker_id", w.id))

	task.ResultCh <- &DownloadURLResult{URL: url, Err: nil}
}

// SubmitTask 提交任务到队列
func (m *DownloadURLManager) SubmitTask(ctx context.Context, key string, expiresIn int64) (string, error) {
	task := &DownloadURLTask{
		Key:      key,
		Expires:  expiresIn,
		ResultCh: make(chan *DownloadURLResult, 1),
	}

	// 尝试提交到队列
	select {
	case m.taskQueue <- task:
		util.Debug("Download URL task submitted to queue", zap.String("key", key))
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		// 队列已满，降级处理：直接生成URL但不缓存
		util.Warn("Download URL queue full, using fallback", zap.String("key", key))
		url, err := m.s3Client.GetObjectURL(ctx, key, expiresIn)
		if err != nil {
			return "", err
		}
		return url, nil
	}

	// 等待任务完成
	select {
	case result := <-task.ResultCh:
		return result.URL, result.Err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stop 停止管理器
func (m *DownloadURLManager) Stop() {
	for _, worker := range m.workers {
		worker.quit <- true
	}
	close(m.taskQueue)
}

// asyncRefreshDownloadURL 异步刷新下载URL缓存
func (s *FileService) asyncRefreshDownloadURL(key string, expiresIn int64) {
	cacheKey := fmt.Sprintf("download_url:%s:%d", key, expiresIn)
	lockKey := "lock:" + cacheKey
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查缓存新鲜度
	ttl, err := util.GetTTL(ctx, cacheKey)
	if err != nil {
		util.Debug("Failed to check cache TTL", zap.String("key", cacheKey), zap.Error(err))
		return
	}

	// 如果缓存还有超过1分钟的TTL，不需要刷新
	if ttl > 1*time.Minute {
		return
	}

	// 获取锁进行刷新
	locked, err := util.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil || !locked {
		return
	}
	defer util.ReleaseLock(ctx, lockKey)

	// 重新生成URL并更新缓存
	url, err := s.s3Client.GetObjectURL(ctx, key, expiresIn)
	if err != nil {
		util.Error("Failed to refresh download URL", zap.String("key", key), zap.Error(err))
		return
	}

	err = util.SetJSON(ctx, cacheKey, url, time.Duration(expiresIn)*time.Second)
	if err != nil {
		util.Error("Failed to update download URL cache", zap.String("key", cacheKey), zap.Error(err))
	} else {
		util.Info("Download URL cache refreshed", zap.String("key", key))
	}
}
