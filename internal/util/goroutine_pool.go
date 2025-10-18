package util

import (
	"sync"

	"github.com/panjf2000/ants/v2"
)

var (
	pool                    *ants.Pool
	poolOnce                sync.Once
	parallelDownloadThreads int
)

// InitGoroutinePool 初始化协程池
func InitGoroutinePool(size int) error {
	var err error
	poolOnce.Do(func() {
		pool, err = ants.NewPool(size, ants.WithNonblocking(false))
	})
	return err
}

// SetParallelDownloadThreads 设置并行下载线程数
func SetParallelDownloadThreads(threads int) {
	parallelDownloadThreads = threads
}

// GetParallelDownloadThreads 获取并行下载线程数
func GetParallelDownloadThreads() int {
	if parallelDownloadThreads <= 0 {
		return 5 // 默认值
	}
	return parallelDownloadThreads
}

// SubmitTask 提交任务到协程池
func SubmitTask(task func()) error {
	if pool == nil {
		return ants.ErrPoolClosed
	}
	return pool.Submit(task)
}

// ReleaseGoroutinePool 释放协程池
func ReleaseGoroutinePool() {
	if pool != nil {
		pool.Release()
	}
}

// Running 正在运行的协程数
func Running() int {
	if pool != nil {
		return pool.Running()
	}
	return 0
}

// Cap 协程池容量
func Cap() int {
	if pool != nil {
		return pool.Cap()
	}
	return 0
}

// Free 协程池空闲数
func Free() int {
	if pool != nil {
		return pool.Free()
	}
	return 0
}
