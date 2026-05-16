// 镜像同步定时调度器
// 负责按配置的间隔周期性触发 GitHub Release 同步
// 启动时立即执行一次全量同步，后续按 interval 循环

package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/config"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// ============================================================
// 调度器结构
// ============================================================

// Scheduler 同步任务调度器
// 管理周期性的软件镜像同步任务
type Scheduler struct {
	syncer    *GitHubSyncer          // 同步器实例
	softwares []config.SoftwareConfig // 需要同步的软件列表
	interval  time.Duration          // 同步间隔
	maxConcurrent int                // 最大并发同步数

	ctx       context.Context        // 调度器上下文
	cancel    context.CancelFunc     // 取消函数
	wg        sync.WaitGroup         // 等待所有同步任务完成
	running   bool                   // 调度器是否运行中
	mu        sync.Mutex
}

// ============================================================
// 初始化
// ============================================================

// NewScheduler 创建同步调度器
// syncer：已初始化的 GitHubSyncer
// cfg：镜像站配置（读取软件列表和同步参数）
func NewScheduler(syncer *GitHubSyncer, cfg *config.MirrorConfig) *Scheduler {
	interval := time.Duration(cfg.Sync.IntervalMinutes) * time.Minute
	if interval <= 0 {
		interval = 30 * time.Minute // 默认 30 分钟
	}

	maxConcurrent := cfg.Sync.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3 // 默认同时同步 3 个软件
	}

	return &Scheduler{
		syncer:        syncer,
		softwares:     cfg.SoftwareList,
		interval:      interval,
		maxConcurrent: maxConcurrent,
	}
}

// ============================================================
// 启动与停止
// ============================================================

// Start 启动调度器
// 立即执行一次全量同步，然后按 interval 周期性触发
// 返回后调度器在后台运行，可通过 Stop 停止
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	util.Info("镜像同步调度器已启动",
		zap.Int("software_count", len(s.softwares)),
		zap.Duration("interval", s.interval),
		zap.Int("max_concurrent", s.maxConcurrent))

	// 立即执行首次同步（异步，不阻塞调用方）
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.syncAll(s.ctx)
	}()

	// 定时循环
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				util.Info("同步调度器收到停止信号，退出")
				return
			case <-ticker.C:
				util.Info("定时同步触发",
					zap.Time("trigger_time", time.Now()))
				s.syncAll(s.ctx)
			}
		}
	}()
}

// Stop 停止调度器
// 等待当前正在执行的同步完成后退出
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	util.Info("同步调度器已完全停止")
}

// ============================================================
// 批量同步
// ============================================================

// syncAll 同步所有配置的软件
// 使用信号量控制并发数，避免同时发起过多 API 请求
func (s *Scheduler) syncAll(ctx context.Context) {
	if len(s.softwares) == 0 {
		util.Warn("镜像站未配置任何软件，跳过同步")
		return
	}

	util.Info("开始全量同步",
		zap.Int("total_software", len(s.softwares)))

	// 使用 channel 作为信号量控制并发
	sem := make(chan struct{}, s.maxConcurrent)
	var wg sync.WaitGroup

	for i := range s.softwares {
		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			util.Info("同步被取消，停止提交新任务")
			wg.Wait()
			return
		default:
		}

		wg.Add(1)
		sw := s.softwares[i] // 拷贝，避免闭包引用问题

		go func(swCfg config.SoftwareConfig) {
			defer wg.Done()

			// 获取信号量
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// 执行同步
			result, err := s.syncer.SyncSoftware(ctx, swCfg)
			if err != nil {
				util.Error("同步软件失败",
					zap.String("software", swCfg.ID),
					zap.Error(err))
			} else if result != nil {
				util.Info("同步软件完成",
					zap.String("software", swCfg.ID),
					zap.Int("new_versions", result.NewVersions),
					zap.Int("new_assets", result.NewAssets),
					zap.Int("skipped", result.Skipped))
			}
		}(sw)
	}

	wg.Wait()
	util.Info("全量同步完成")
}

// ============================================================
// 手动触发
// ============================================================

// TriggerSync 手动触发指定软件的同步
// softwareID 为空时同步全部软件
func (s *Scheduler) TriggerSync(ctx context.Context, softwareID string) error {
	if softwareID == "" {
		go s.syncAll(ctx)
		return nil
	}

	// 查找指定软件配置
	for _, sw := range s.softwares {
		if sw.ID == softwareID {
			go func(cfg config.SoftwareConfig) {
				result, err := s.syncer.SyncSoftware(ctx, cfg)
				if err != nil {
					util.Error("手动同步失败",
						zap.String("software", softwareID),
						zap.Error(err))
				} else {
					util.Info("手动同步完成",
						zap.String("software", softwareID),
						zap.Int("new_versions", result.NewVersions),
						zap.Int("new_assets", result.NewAssets))
				}
			}(sw)
			return nil
		}
	}

	return fmt.Errorf("未找到软件: %s", softwareID)
}

// ============================================================
// 状态查询
// ============================================================

// GetStatus 获取调度器状态
func (s *Scheduler) GetStatus() *model.SyncStatus {
	return s.syncer.GetStatus()
}
