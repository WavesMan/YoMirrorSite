// 版本与资产 Repository
// 封装 version / asset 表的 GORM 操作
// 版本：(software_id + tag_name) 联合唯一
// 资产：s3_key 唯一，与版本一对多

package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yomirrorsite/internal/model"
)

// ============================================================
// 版本操作
// ============================================================

// UpsertVersion 插入或更新版本信息
// 按 software_id + tag_name 联合唯一键冲突时更新
// 返回版本 ID（插入或已存在的）
func (c *Client) UpsertVersion(ctx context.Context, v *model.VersionTable) (int, error) {
	err := c.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "software_id"}, {Name: "tag_name"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "prerelease", "published_at", "body",
			}),
		}).
		Create(v).Error
	if err != nil {
		return 0, err
	}
	return v.ID, nil
}

// ListVersionsBySoftware 按软件 ID 查版本列表
// 按发布时间倒序，limit<=0 时返回全部
func (c *Client) ListVersionsBySoftware(ctx context.Context, softwareID string, limit int) ([]model.VersionTable, error) {
	query := c.DB.WithContext(ctx).
		Where("software_id = ?", softwareID).
		Order("published_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var list []model.VersionTable
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetVersion 按软件 ID + tag 查询单版本
func (c *Client) GetVersion(ctx context.Context, softwareID, tagName string) (*model.VersionTable, bool, error) {
	var v model.VersionTable
	err := c.DB.WithContext(ctx).
		Where("software_id = ? AND tag_name = ?", softwareID, tagName).
		First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &v, true, nil
}

// ============================================================
// 资产操作
// ============================================================

// BatchInsertAssets 批量插入资产
// s3_key 冲突时跳过（幂等），忽略已存在的记录
// 返回成功插入的行数
func (c *Client) BatchInsertAssets(ctx context.Context, assets []model.AssetTable) (int, error) {
	if len(assets) == 0 {
		return 0, nil
	}
	err := c.DB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&assets).Error
	if err != nil {
		return 0, err
	}
	return len(assets), nil
}

// ListAssetsByVersion 按版本 ID 查询所有资产
func (c *Client) ListAssetsByVersion(ctx context.Context, versionID int) ([]model.AssetTable, error) {
	var list []model.AssetTable
	err := c.DB.WithContext(ctx).Where("version_id = ?", versionID).Find(&list).Error
	return list, err
}

// GetAssetByS3Key 按 S3 key 查询资产
func (c *Client) GetAssetByS3Key(ctx context.Context, s3Key string) (*model.AssetTable, bool, error) {
	var a model.AssetTable
	err := c.DB.WithContext(ctx).Where("s3_key = ?", s3Key).First(&a).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &a, true, nil
}

// UpdateAssetS3URL 更新资产的预签名下载 URL 及过期时间
func (c *Client) UpdateAssetS3URL(ctx context.Context, assetID int, url string, expires time.Time) error {
	return c.DB.WithContext(ctx).Model(&model.AssetTable{}).
		Where("id = ?", assetID).
		Updates(map[string]interface{}{
			"s3_url":     url,
			"s3_url_exp": expires,
		}).Error
}

// IncrementDownload 下载计数 +1
func (c *Client) IncrementDownload(ctx context.Context, assetID int) error {
	return c.DB.WithContext(ctx).Model(&model.AssetTable{}).
		Where("id = ?", assetID).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// ============================================================
// 对账辅助
// ============================================================

// ListS3KeysBySoftware 查询某软件下所有资产的 s3_key
// 用于与 S3 实际对象列表对账
func (c *Client) ListS3KeysBySoftware(ctx context.Context, softwareID string) ([]string, error) {
	var keys []string
	err := c.DB.WithContext(ctx).
		Model(&model.AssetTable{}).
		Joins("JOIN version ON version.id = asset.version_id").
		Where("version.software_id = ?", softwareID).
		Pluck("asset.s3_key", &keys).Error
	return keys, err
}

// DeleteAssetByS3Key 按 s3_key 删除资产记录（对账清理脏数据）
func (c *Client) DeleteAssetByS3Key(ctx context.Context, s3Key string) error {
	return c.DB.WithContext(ctx).Where("s3_key = ?", s3Key).Delete(&model.AssetTable{}).Error
}

// DeleteOldVersions 按软件 ID + tag 列表级联删除版本和资产记录
// 用于 latest_only 同步规则下的旧版本清理
func (c *Client) DeleteOldVersions(ctx context.Context, softwareID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	// 先删除资产（GORM 软删除或硬删除取决于 model 定义，此处为硬删除）
	err := c.DB.WithContext(ctx).
		Where("version_id IN (?)",
			c.DB.Table("version").Select("id").Where("software_id = ? AND tag_name IN ?", softwareID, tags)).
		Delete(&model.AssetTable{}).Error
	if err != nil {
		return err
	}
	// 再删除版本
	return c.DB.WithContext(ctx).
		Where("software_id = ? AND tag_name IN ?", softwareID, tags).
		Delete(&model.VersionTable{}).Error
}
