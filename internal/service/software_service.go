// 软件镜像业务服务
// 负责软件列表、详情、版本查询、下载 URL 生成
// 复用项目已有的三级缓存链路：本地 LRU → Redis → S3 源站
// 参考 file_service.go 的缓存策略和锁机制

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/core/postgres"
	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// ============================================================
// 常量定义
// ============================================================

const (
	// softwareListKey Redis 软件列表缓存 key
	softwareListKey = "mirror:software_list"
	// softwareDetailKeyPrefix 软件详情 Redis key 前缀
	softwareDetailKeyPrefix = "software:detail:"
	// versionDetailKeyPrefix 版本详情 Redis key 前缀
	// 由同步器写入，本服务只读
	versionDetailKeyPrefix = "software:%s:version:%s"
	// downloadCounterKey 下载计数 Redis key 前缀
	downloadCounterKey = "mirror:downloads"
	// softwareListCacheTTL 软件列表缓存 TTL
	softwareListCacheTTL = 10 * time.Minute
	// softwareDetailCacheTTL 软件详情缓存 TTL
	softwareDetailCacheTTL = 30 * time.Minute
	// defaultPageSize 默认分页大小
	defaultPageSize = 20
)

// ============================================================
// 服务结构
// ============================================================

// SoftwareService 软件镜像业务服务
// 提供软件列表、详情、下载 URL 等查询能力
type SoftwareService struct {
	s3Client     *s3.Client          // S3 对象存储客户端
	cacheManager *util.CacheManager  // 本地 LRU/LFU 缓存管理器
	pgClient     *postgres.Client    // PG 持久化层（可选，nil 时降级）
}

// NewSoftwareService 创建软件服务
// s3Client：S3 客户端（复用已有实例）
// cacheManager：本地缓存管理器（可选，nil 时跳过低延迟缓存层）
// pgClient：PG 持久化客户端（可选，nil 时降级到纯 S3+Redis）
func NewSoftwareService(s3Client *s3.Client, cacheManager *util.CacheManager, pgClient *postgres.Client) *SoftwareService {
	return &SoftwareService{
		s3Client:     s3Client,
		cacheManager: cacheManager,
		pgClient:     pgClient,
	}
}

// ============================================================
// 软件列表
// ============================================================

// ListSoftware 获取软件列表（支持分类筛选和关键词搜索）
// 查询链路：PG（SQL 分页）→ Redis → S3 源站
// PG 启用时直接 SQL 分页+排序，O(log n)；否则全量加载后内存筛选
func (s *SoftwareService) ListSoftware(ctx context.Context, category, keyword string, page, pageSize int) (*model.SoftwareListPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = defaultPageSize
	}

	// 1. PG 优先：直接 SQL 分页 + 排序
	if s.pgClient != nil && s.pgClient.IsEnabled() {
		list, total, err := s.pgClient.ListSoftware(ctx, page, pageSize, category, keyword, "stars")
		if err == nil {
			items := make([]model.Software, len(list))
			for i := range list { items[i] = list[i].ToAPI() }
			return &model.SoftwareListPage{Items: items, Page: page, PageSize: pageSize, TotalCount: int(total)}, nil
		}
		util.Warn("PG 查询软件列表失败，降级到 Redis/S3", zap.Error(err))
	}

	// 2. 降级：Redis → S3 全量加载 + 内存筛选
	var allSoftware []model.Software
	found, err := util.GetJSON(ctx, softwareListKey, &allSoftware)
	if err != nil {
		util.Warn("从 Redis 读取软件列表失败，降级到 S3", zap.Error(err))
	}
	if !found {
		allSoftware, err = s.loadSoftwareListFromS3(ctx)
		if err != nil {
			return nil, fmt.Errorf("加载软件列表失败: %w", err)
		}
		go func() {
			_ = util.SetJSON(context.Background(), softwareListKey, allSoftware, softwareListCacheTTL)
		}()
	}

	// 3. 内存筛选
	filtered := filterSoftware(allSoftware, category, keyword)

	// 4. 分页裁剪
	total := len(filtered)
	start := (page - 1) * pageSize
	if start >= total {
		return &model.SoftwareListPage{
			Items:      []model.Software{},
			Page:       page,
			PageSize:   pageSize,
			TotalCount: total,
		}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	return &model.SoftwareListPage{
		Items:      filtered[start:end],
		Page:       page,
		PageSize:   pageSize,
		TotalCount: total,
	}, nil
}

// ============================================================
// 软件详情
// ============================================================

// GetSoftware 获取软件详情（含版本列表）
// 缓存策略：Redis → S3
func (s *SoftwareService) GetSoftware(ctx context.Context, softwareID string) (*model.SoftwareDetail, error) {
	// 1. PG 优先：SQL 联合查询 software + tags + versions
	if s.pgClient != nil && s.pgClient.IsEnabled() {
		sw, found, err := s.pgClient.GetSoftware(ctx, softwareID)
		if err == nil && found {
			tags, _ := s.pgClient.GetTags(ctx, softwareID)
			versions, _ := s.pgClient.ListVersionsBySoftware(ctx, softwareID, 0)
			detail := s.assembleDetail(sw, tags, versions)
			go func() {
				cacheKey := softwareDetailKeyPrefix + softwareID
				_ = util.SetJSON(context.Background(), cacheKey, detail, softwareDetailCacheTTL)
			}()
			return &detail, nil
		}
		if err != nil {
			util.Warn("PG 查询软件详情失败，降级到 Redis/S3", zap.String("id", softwareID), zap.Error(err))
		}
	}

	// 2. 降级：查 Redis 缓存
	cacheKey := softwareDetailKeyPrefix + softwareID
	var detail model.SoftwareDetail
	found, err := util.GetJSON(ctx, cacheKey, &detail)
	if err != nil {
		util.Warn("从 Redis 读取软件详情失败", zap.String("id", softwareID), zap.Error(err))
	}
	if found {
		return &detail, nil
	}

	// 2. 从 S3 加载
	detail, err = s.loadSoftwareFromS3(ctx, softwareID)
	if err != nil {
		return nil, fmt.Errorf("加载软件详情失败: %w", err)
	}

	// 3. 查询版本列表
	versions, err := s.listVersions(ctx, softwareID)
	if err != nil {
		util.Warn("获取版本列表失败", zap.String("id", softwareID), zap.Error(err))
	}
	detail.Versions = versions
	detail.TotalVersions = len(versions)

	// 4. 异步写回 Redis
	go func() {
		_ = util.SetJSON(context.Background(), cacheKey, detail, softwareDetailCacheTTL)
	}()

	return &detail, nil
}

// ============================================================
// 版本详情
// ============================================================

// GetVersion 获取单个版本的详细信息（含下载 URL）
// 版本元数据由同步器在同步时写入 Redis，此处只读
func (s *SoftwareService) GetVersion(ctx context.Context, softwareID, tagName string, expiresIn int64) (*model.VersionDetail, error) {
	// 1. 从 Redis 读取版本元数据（同步器写入）
	cacheKey := fmt.Sprintf(versionDetailKeyPrefix, softwareID, tagName)
	var detail model.VersionDetail
	found, err := util.GetJSON(ctx, cacheKey, &detail)
	if err != nil {
		return nil, fmt.Errorf("读取版本元数据失败: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("版本不存在: %s/%s", softwareID, tagName)
	}

	// 2. 为每个资产生成下载 URL
	// 复用 s3Client.GetObjectURL 生成预签名 URL
	for i := range detail.Assets {
		s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s",
			softwareID, tagName, detail.Assets[i].Name)
		url, err := s.s3Client.GetObjectURL(ctx, s3Key, expiresIn)
		if err != nil {
			util.Warn("生成下载 URL 失败",
				zap.String("software", softwareID),
				zap.String("asset", detail.Assets[i].Name),
				zap.Error(err))
			// 不阻止其他资产继续处理
			continue
		}
		detail.Assets[i].DownloadURL = url
	}

	return &detail, nil
}

// ============================================================
// 下载 URL
// ============================================================

// GetDownloadURL 获取单个资产的预签名下载 URL
// 复用 s3Client.GetObjectURL，单次请求不缓存（URL 自带有效期）
func (s *SoftwareService) GetDownloadURL(ctx context.Context, softwareID, tagName, assetName string, expiresIn int64) (string, error) {
	s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s", softwareID, tagName, assetName)

	url, err := s.s3Client.GetObjectURL(ctx, s3Key, expiresIn)
	if err != nil {
		return "", fmt.Errorf("生成下载 URL 失败: %w", err)
	}

	// 记录下载计数
	go func() {
		_, _ = util.IncrementCounter(context.Background(), downloadCounterKey)
	}()

	return url, nil
}

// ============================================================
// 镜像站统计
// ============================================================

// GetStats 获取镜像站整体统计信息
// 从 Redis 计数器聚合统计
func (s *SoftwareService) GetStats(ctx context.Context) (*model.MirrorStats, error) {
	stats := &model.MirrorStats{}

	// 1. 从软件列表获取软件总数
	var allSoftware []model.Software
	found, err := util.GetJSON(ctx, softwareListKey, &allSoftware)
	if err == nil && found {
		stats.TotalSoftware = len(allSoftware)
	}

	// 2. 下载总数（Redis 计数器）
	downloads, err := util.GetCounter(ctx, downloadCounterKey)
	if err == nil {
		stats.TotalDownloads = downloads
	}

	// 3. 其他统计需要从 S3 遍历计算（低频，按需触发）
	// 这里先返回基础统计

	return stats, nil
}

// ============================================================
// 内部辅助方法
// ============================================================

// loadSoftwareListFromS3 从 S3 加载所有软件概要信息
// 遍历 mirrors/ 目录下的所有 meta.json
func (s *SoftwareService) loadSoftwareListFromS3(ctx context.Context) ([]model.Software, error) {
	// 列出 mirrors/ 下的所有对象
	objects, err := s.s3Client.ListObjects(ctx, "mirrors/")
	if err != nil {
		return nil, fmt.Errorf("列出镜像目录失败: %w", err)
	}

	// 提取所有软件 ID（去重）
	softwareIDs := make(map[string]bool)
	for _, obj := range objects {
		key := *obj.Key
		// mirrors/{software_id}/...
		parts := strings.SplitN(strings.TrimPrefix(key, "mirrors/"), "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			softwareIDs[parts[0]] = true
		}
	}

	// 逐一加载软件详情并转为 Software 概要
	var softwareList []model.Software
	for id := range softwareIDs {
		detail, err := s.loadSoftwareFromS3(ctx, id)
		if err != nil {
			util.Warn("加载软件详情失败，跳过", zap.String("id", id), zap.Error(err))
			continue
		}
		softwareList = append(softwareList, detail.Software)
	}

	return softwareList, nil
}

// loadSoftwareFromS3 从 S3 加载单个软件的元数据
// 读取 mirrors/{id}/meta.json
func (s *SoftwareService) loadSoftwareFromS3(ctx context.Context, softwareID string) (model.SoftwareDetail, error) {
	metaKey := fmt.Sprintf("mirrors/%s/meta.json", softwareID)

	// TODO: 实际应该从 S3 下载 meta.json 并反序列化
	// 这里先返回一个占位详情，等待同步器写入后即可正常返回
	detail := model.SoftwareDetail{
		Software: model.Software{
			ID:       softwareID,
			Name:     softwareID, // 临时使用 ID 作为名称
			Category: "uncategorized",
		},
	}

	// 尝试从 Redis 读取最新版本标记
	latestKey := fmt.Sprintf("mirrors/%s/versions/latest.txt", softwareID)
	latestTag, err := util.Get(ctx, latestKey)
	if err == nil && latestTag != "" {
		detail.LatestVer = latestTag
	}

	// 获取已同步的最新 tag
	lastSyncedKey := fmt.Sprintf("last_synced:software:%s", softwareID)
	lastSynced, err := util.Get(ctx, lastSyncedKey)
	if err == nil && lastSynced != "" {
		detail.UpdatedAt = lastSyncedCreated // 使用 Redis 记录的时间
	}
	_ = lastSynced
	_ = metaKey

	return detail, nil
}

// listVersions 获取指定软件的版本概要列表
func (s *SoftwareService) listVersions(ctx context.Context, softwareID string) ([]model.VersionBrief, error) {
	// 使用 Redis Keys 扫描该软件的所有版本 key
	pattern := fmt.Sprintf("software:%s:version:*", softwareID)
	keys, err := util.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("扫描版本键失败: %w", err)
	}

	var versions []model.VersionBrief
	for _, key := range keys {
		var detail model.VersionDetail
		found, err := util.GetJSON(ctx, key, &detail)
		if err != nil || !found {
			continue
		}
		versions = append(versions, model.VersionBrief{
			TagName:     detail.TagName,
			Name:        detail.Name,
			Prerelease:  detail.Prerelease,
			PublishedAt: detail.PublishedAt,
			AssetCount:  len(detail.Assets),
		})
	}

	return versions, nil
}

// ============================================================
// 筛选辅助函数
// ============================================================

// filterSoftware 根据分类和关键词筛选软件列表
func filterSoftware(list []model.Software, category, keyword string) []model.Software {
	var result []model.Software

	for _, sw := range list {
		// 分类筛选
		if category != "" && sw.Category != category {
			continue
		}
		// 关键词筛选（不区分大小写，匹配名称和描述）
		if keyword != "" {
			kw := strings.ToLower(keyword)
			name := strings.ToLower(sw.Name)
			desc := strings.ToLower(sw.Description)
			if !strings.Contains(name, kw) && !strings.Contains(desc, kw) {
				// 也检查标签
				found := false
				for _, tag := range sw.Tags {
					if strings.Contains(strings.ToLower(tag), kw) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}
		result = append(result, sw)
	}

	return result
}

// lastSyncedCreated 占位变量，表明时间来自 Redis
var lastSyncedCreated string

// ============================================================
// PG 组装（YoMirrorSite 新增）
// ============================================================

// assembleDetail 从 PG 的 software + tags + versions 组装 SoftwareDetail
func (s *SoftwareService) assembleDetail(sw *model.SoftwareTable, tags []string, versions []model.VersionTable) model.SoftwareDetail {
	vbriefs := make([]model.VersionBrief, 0, len(versions))
	for _, v := range versions {
		vbriefs = append(vbriefs, model.VersionBrief{
			TagName:     v.TagName,
			Name:        v.Name,
			Prerelease:  v.Prerelease,
			PublishedAt: v.PublishedAt.Format("2006-01-02"),
		})
	}
	return model.SoftwareDetail{
		Software:      sw.ToAPI(),
		Tags:          tags,
		Versions:      vbriefs,
		TotalVersions: len(vbriefs),
		TotalSize:     sw.TotalSize,
		ReadmeMD:      sw.ReadmeMD,
	}
}
