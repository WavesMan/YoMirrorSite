package util

import (
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
)

// ProgressReader 带进度监控的Reader
type ProgressReader struct {
	Reader    io.Reader
	Total     int64
	BytesRead int64
	Label     string
	startTime time.Time
	lastPrint time.Time
	lastBytes int64
}

// NewProgressReader 创建新的进度监控Reader
func NewProgressReader(reader io.Reader, total int64, label string) *ProgressReader {
	return &ProgressReader{
		Reader:    reader,
		Total:     total,
		Label:     label,
		startTime: time.Now(),
		lastPrint: time.Now(),
	}
}

// Read 实现io.Reader接口
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.BytesRead += int64(n)
		pr.printProgress()
	}
	return n, err
}

// printProgress 打印下载进度
func (pr *ProgressReader) printProgress() {
	now := time.Now()

	// 每500ms更新一次进度，或者完成时
	elapsed := now.Sub(pr.lastPrint)
	if elapsed < 500*time.Millisecond && pr.BytesRead < pr.Total {
		return
	}

	// 计算下载速度
	bytesSinceLast := pr.BytesRead - pr.lastBytes
	speed := float64(bytesSinceLast) / elapsed.Seconds()

	// 计算进度百分比
	percent := float64(pr.BytesRead) / float64(pr.Total) * 100

	// 计算剩余时间
	var eta string
	if speed > 0 && pr.BytesRead < pr.Total {
		remainingBytes := pr.Total - pr.BytesRead
		remainingTime := time.Duration(float64(remainingBytes)/speed) * time.Second
		eta = formatDuration(remainingTime)
	} else {
		eta = "calculating..."
	}

	// 格式化速度
	speedStr := formatBytes(speed)

	// 创建进度条
	progressBar := createProgressBar(percent, 20)

	// 打印进度信息（单行刷新）
	fmt.Printf("\r%s | %s | %.1f%% | %s/s | ETA: %s",
		pr.Label, progressBar, percent, speedStr, eta)

	// 如果下载完成，换行
	if pr.BytesRead == pr.Total {
		fmt.Println()
		Info("Progress completed",
			zap.String("label", pr.Label),
			zap.Int64("total", pr.Total),
			zap.Float64("percent", 100.0),
		)
	}

	pr.lastPrint = now
	pr.lastBytes = pr.BytesRead
}

// createProgressBar 创建进度条
func createProgressBar(percent float64, width int) string {
	completed := int(percent * float64(width) / 100)
	remaining := width - completed

	bar := "["
	for i := 0; i < completed; i++ {
		bar += "="
	}
	for i := 0; i < remaining; i++ {
		bar += " "
	}
	bar += "]"

	return bar
}

// formatBytes 格式化字节大小
func formatBytes(bytes float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", bytes/KB)
	default:
		return fmt.Sprintf("%.0f B", bytes)
	}
}

// formatDuration 格式化时间
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}

// ProgressBar 进度条接口
type ProgressBar interface {
	Update(current, total int64)
	Finish()
}

// SimpleProgressBar 简单进度条实现
type SimpleProgressBar struct {
	Label     string
	Total     int64
	Current   int64
	startTime time.Time
}

// NewSimpleProgressBar 创建简单进度条
func NewSimpleProgressBar(label string, total int64) *SimpleProgressBar {
	return &SimpleProgressBar{
		Label:     label,
		Total:     total,
		startTime: time.Now(),
	}
}

// Update 更新进度
func (pb *SimpleProgressBar) Update(current, total int64) {
	pb.Current = current
	pb.Total = total

	percent := float64(current) / float64(total) * 100
	elapsed := time.Since(pb.startTime)

	var speed float64
	var eta string

	if elapsed.Seconds() > 0 {
		speed = float64(current) / elapsed.Seconds()
		if speed > 0 {
			remaining := total - current
			remainingTime := time.Duration(float64(remaining)/speed) * time.Second
			eta = formatDuration(remainingTime)
		} else {
			eta = "calculating..."
		}
	} else {
		eta = "calculating..."
	}

	speedStr := formatBytes(speed)
	progressBar := createProgressBar(percent, 20)

	fmt.Printf("\r%s | %s | %.1f%% | %s/s | ETA: %s",
		pb.Label, progressBar, percent, speedStr, eta)
}

// Finish 完成进度条
func (pb *SimpleProgressBar) Finish() {
	fmt.Println()
	Info("Progress completed",
		zap.String("label", pb.Label),
		zap.Int64("total", pb.Total),
		zap.Float64("percent", 100.0),
	)
}
