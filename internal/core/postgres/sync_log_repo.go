// 同步日志 Repository
// 封装 sync_log 表的 GORM 操作
// 每次软件同步创建一条日志记录，记录新版本数、新资产数、错误信息等

package postgres

import (
	"context"
	"time"

	"yomirrorsite/internal/model"
)

// ============================================================
// 同步日志操作
// ============================================================

// CreateSyncLog 创建一条同步日志（status='running'）
// 返回日志 ID，供后续 FinishSyncLog 使用
func (c *Client) CreateSyncLog(ctx context.Context, softwareID string) (int, error) {
	log := model.SyncLogTable{
		SoftwareID: softwareID,
		Status:     "running",
		StartedAt:  time.Now(),
	}
	if err := c.DB.WithContext(ctx).Create(&log).Error; err != nil {
		return 0, err
	}
	return log.ID, nil
}

// FinishSyncLog 更新同步日志为成功或失败
// status: "success" 或 "failed"
func (c *Client) FinishSyncLog(ctx context.Context, id int, status string,
	newVersions, newAssets, skipped int, totalSize int64, errMsg string) error {

	now := time.Now()
	return c.DB.WithContext(ctx).Model(&model.SyncLogTable{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"new_versions": newVersions,
			"new_assets":   newAssets,
			"skipped":      skipped,
			"total_size":   totalSize,
			"error_msg":    errMsg,
			"finished_at":  now,
		}).Error
}

// GetLastSyncLog 获取某软件最近一次成功的同步日志
func (c *Client) GetLastSyncLog(ctx context.Context, softwareID string) (*model.SyncLogTable, bool, error) {
	var log model.SyncLogTable
	err := c.DB.WithContext(ctx).
		Where("software_id = ? AND status = ?", softwareID, "success").
		Order("finished_at DESC").
		First(&log).Error
	if err != nil {
		return nil, false, err
	}
	return &log, true, nil
}

// GetRecentSyncLogs 获取最近的同步日志列表（全局）
func (c *Client) GetRecentSyncLogs(ctx context.Context, limit int) ([]model.SyncLogTable, error) {
	var logs []model.SyncLogTable
	err := c.DB.WithContext(ctx).
		Order("started_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetSyncLogsBySoftware 获取某软件的所有同步历史（分页）
func (c *Client) GetSyncLogsBySoftware(ctx context.Context, softwareID string, limit int) ([]model.SyncLogTable, error) {
	var logs []model.SyncLogTable
	err := c.DB.WithContext(ctx).
		Where("software_id = ?", softwareID).
		Order("started_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
