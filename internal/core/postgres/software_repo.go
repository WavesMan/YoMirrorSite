// 软件仓库 Repository
// 封装 software / software_tag 表的 GORM 操作
// 方法签名：若出错返回 error，上层降级处理

package postgres

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"yomirrorsite/internal/model"
)

// ============================================================
// 软件基本信息操作
// ============================================================

// UpsertSoftware 插入或更新软件基本信息
// 按 id 冲突时更新除 created_at 外的全部字段
func (c *Client) UpsertSoftware(ctx context.Context, sw *model.SoftwareTable) error {
	return c.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "description", "github_repo", "homepage",
				"icon_url", "category", "license", "stars",
				"readme_md", "latest_ver", "total_size", "updated_at",
			}),
		}).
		Create(sw).Error
}

// GetSoftware 按 id 查询单条软件信息
// found=false 表示记录不存在
func (c *Client) GetSoftware(ctx context.Context, id string) (*model.SoftwareTable, bool, error) {
	var sw model.SoftwareTable
	err := c.DB.WithContext(ctx).Where("id = ?", id).First(&sw).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &sw, true, nil
}

// ListSoftware 分页列表查询
// 支持 category 筛选、keyword 模糊搜索、sortBy 排序
// sortBy 可选值："stars"（按星数降序）、"updated_at"（按更新时间降序）、""（默认按名称升序）
func (c *Client) ListSoftware(ctx context.Context, page, size int,
	category, keyword, sortBy string) ([]model.SoftwareTable, int64, error) {

	query := c.DB.WithContext(ctx).Model(&model.SoftwareTable{})

	// 分类筛选
	if category != "" {
		query = query.Where("category = ?", category)
	}
	// 关键词模糊搜索（名称 + 描述）
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", like, like)
	}

	// 排序
	switch sortBy {
	case "stars":
		query = query.Order("stars DESC")
	case "updated_at":
		query = query.Order("updated_at DESC")
	default:
		query = query.Order("name ASC")
	}

	// 计数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	if page < 1 { page = 1 }
	if size < 1 { size = 20 }
	offset := (page - 1) * size

	var list []model.SoftwareTable
	if err := query.Offset(offset).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ============================================================
// 软件标签操作
// ============================================================

// SaveTags 全量替换软件标签
// 先删后插（在同一事务中）
func (c *Client) SaveTags(ctx context.Context, softwareID string, tags []string) error {
	return c.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除旧标签
		if err := tx.Where("software_id = ?", softwareID).Delete(&model.SoftwareTagTable{}).Error; err != nil {
			return err
		}
		// 插入新标签
		if len(tags) == 0 {
			return nil
		}
		tagRows := make([]model.SoftwareTagTable, len(tags))
		for i, t := range tags {
			tagRows[i] = model.SoftwareTagTable{SoftwareID: softwareID, Tag: t}
		}
		return tx.Create(&tagRows).Error
	})
}

// GetTags 获取软件标签列表
func (c *Client) GetTags(ctx context.Context, softwareID string) ([]string, error) {
	var rows []model.SoftwareTagTable
	if err := c.DB.WithContext(ctx).Where("software_id = ?", softwareID).Find(&rows).Error; err != nil {
		return nil, err
	}
	tags := make([]string, len(rows))
	for i := range rows { tags[i] = rows[i].Tag }
	return tags, nil
}

// GetCategoryCounts 按分类聚合软件数量（首页统计用）
func (c *Client) GetCategoryCounts(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Category string
		Count    int64
	}
	var rows []row
	err := c.DB.WithContext(ctx).Model(&model.SoftwareTable{}).
		Select("category, COUNT(*) as count").
		Group("category").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for _, r := range rows {
		result[r.Category] = r.Count
	}
	return result, nil
}
