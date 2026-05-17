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

const (
	softwareListKey        = "mirror:software_list"
	softwareDetailKeyPrefix = "software:detail:"
	versionDetailKeyPrefix = "software:%s:version:%s"
	downloadCounterKey     = "mirror:downloads"
	softwareListCacheTTL   = 10 * time.Minute
	softwareDetailCacheTTL = 30 * time.Minute
	defaultPageSize        = 20
)

type SoftwareService struct {
	s3Client     *s3.Client
	cacheManager *util.CacheManager
	pgClient     *postgres.Client
}

func NewSoftwareService(s3Client *s3.Client, cacheManager *util.CacheManager, pgClient *postgres.Client) *SoftwareService {
	return &SoftwareService{
		s3Client:     s3Client,
		cacheManager: cacheManager,
		pgClient:     pgClient,
	}
}

func (s *SoftwareService) ListSoftware(ctx context.Context, category, keyword string, page, pageSize int) (*model.SoftwareListPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = defaultPageSize
	}

	if s.pgClient != nil && s.pgClient.IsEnabled() {
		list, total, err := s.pgClient.ListSoftware(ctx, page, pageSize, category, keyword, "stars")
		if err == nil {
			items := make([]model.Software, len(list))
			for i := range list { items[i] = list[i].ToAPI() }
			util.Debug("软件列表查询成功", util.Module("service"), zap.Int("count", len(items)), zap.Int64("total", total), util.Action("list"))
			return &model.SoftwareListPage{Items: items, Page: page, PageSize: pageSize, TotalCount: int(total)}, nil
		}
		util.Warn("PG 查询软件列表失败，降级到 Redis/S3", zap.Error(err))
	}

	var allSoftware []model.Software
	found, err := util.GetJSON(ctx, softwareListKey, &allSoftware)
	if err != nil {
		util.Warn("从 Redis 读取软件列表失败，降级到 S3", zap.Error(err))
		found = false
	}
	if !found {
		allSoftware, err = s.loadSoftwareListFromS3(ctx)
		if err != nil {
			return nil, fmt.Errorf("加载软件列表失败: %w", err)
		}
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = util.SetJSON(bgCtx, softwareListKey, allSoftware, softwareListCacheTTL)
		}()
	}

	filtered := filterSoftware(allSoftware, category, keyword)

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

func (s *SoftwareService) GetSoftware(ctx context.Context, softwareID string) (*model.SoftwareDetail, error) {
	if s.pgClient != nil && s.pgClient.IsEnabled() {
		sw, found, err := s.pgClient.GetSoftware(ctx, softwareID)
		if err == nil && found {
			tags, _ := s.pgClient.GetTags(ctx, softwareID)
			versions, _ := s.pgClient.ListVersionsBySoftware(ctx, softwareID, 0)
			detail := s.assembleDetail(sw, tags, versions)
			go func() {
				cacheKey := softwareDetailKeyPrefix + softwareID
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = util.SetJSON(bgCtx, cacheKey, detail, softwareDetailCacheTTL)
			}()
			util.Debug("PG 查询软件详情成功", util.Module("service"), util.Software(softwareID), util.Action("detail"))
			return &detail, nil
		}
		if err != nil {
			util.Warn("PG 查询软件详情失败，降级到 Redis/S3", zap.String("id", softwareID), zap.Error(err))
		}
	}

	cacheKey := softwareDetailKeyPrefix + softwareID
	var detail model.SoftwareDetail
	found, err := util.GetJSON(ctx, cacheKey, &detail)
	if err != nil {
		util.Warn("从 Redis 读取软件详情失败", zap.String("id", softwareID), zap.Error(err))
	}
	if found {
		util.Debug("Redis 缓存命中", util.Module("service"), util.Software(softwareID), util.Action("cache"))
		return &detail, nil
	}

	detail, err = s.loadSoftwareFromS3(ctx, softwareID)
	if err != nil {
		return nil, fmt.Errorf("加载软件详情失败: %w", err)
	}

	versions, err := s.listVersions(ctx, softwareID)
	if err != nil {
		util.Warn("获取版本列表失败", zap.String("id", softwareID), zap.Error(err))
	}
	detail.Versions = versions
	detail.TotalVersions = len(versions)

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = util.SetJSON(bgCtx, cacheKey, detail, softwareDetailCacheTTL)
	}()

	return &detail, nil
}

func (s *SoftwareService) GetVersion(ctx context.Context, softwareID, tagName string, expiresIn int64) (*model.VersionDetail, error) {
	key := fmt.Sprintf(versionDetailKeyPrefix, softwareID, tagName)
	var detail model.VersionDetail
	found, err := util.GetJSON(ctx, key, &detail)
	if err != nil {
		util.Warn("从 Redis 读取版本详情失败", zap.String("key", key), zap.Error(err))
	}
	if !found {
		return nil, fmt.Errorf("版本不存在: %s/%s", softwareID, tagName)
	}

	for i := range detail.Assets {
		asset := &detail.Assets[i]
		s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s", softwareID, tagName, asset.Name)
		url, err := s.s3Client.GetObjectURL(ctx, s3Key, expiresIn)
		if err != nil {
			util.Warn("生成下载 URL 失败", zap.String("key", s3Key), zap.Error(err))
			continue
		}
		asset.DownloadURL = url
	}

	return &detail, nil
}

func (s *SoftwareService) GetDownloadURL(ctx context.Context, softwareID, tagName, assetName string, expiresIn int64) (string, error) {
	s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s", softwareID, tagName, assetName)
	url, err := s.s3Client.GetObjectURL(ctx, s3Key, expiresIn)
	if err != nil {
		return "", fmt.Errorf("生成下载 URL 失败: %w", err)
	}
	return url, nil
}

func (s *SoftwareService) GetStats(ctx context.Context) (*model.MirrorStats, error) {
	if s.pgClient != nil && s.pgClient.IsEnabled() {
		stats, err := s.pgClient.GetStats(ctx)
		if err == nil {
			return stats, nil
		}
		util.Warn("PG 查询统计失败，降级", zap.Error(err))
	}

	stats := &model.MirrorStats{}
	allSoftware, _, _ := s.loadSoftwareListWithMeta(ctx)
	stats.TotalSoftware = len(allSoftware)

	for _, sw := range allSoftware {
		versions, _ := s.listVersions(ctx, sw.ID)
		stats.TotalVersions += len(versions)
		stats.TotalAssets += sw.TotalAssets
	}

	return stats, nil
}

func filterSoftware(list []model.Software, category, keyword string) []model.Software {
	var result []model.Software
	for _, sw := range list {
		if category != "" && sw.Category != category {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(sw.Name), strings.ToLower(keyword)) {
			continue
		}
		result = append(result, sw)
	}
	return result
}

func (s *SoftwareService) loadSoftwareListFromS3(ctx context.Context) ([]model.Software, error) {
	list, _, err := s.loadSoftwareListWithMeta(ctx)
	return list, err
}

func (s *SoftwareService) loadSoftwareListWithMeta(ctx context.Context) ([]model.Software, []string, error) {
	objects, err := s.s3Client.ListObjects(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	var allSoftware []model.Software
	var metaKeys []string
	for _, obj := range objects {
		key := *obj.Key
		if strings.HasSuffix(key, "meta.json") {
			metaKeys = append(metaKeys, key)
			data, err := s.s3Client.GetObject(ctx, key)
			if err != nil {
				util.Warn("读取 S3 元数据失败", zap.String("key", key), zap.Error(err))
				continue
			}
			var sw model.Software
			if err := json.Unmarshal(data, &sw); err != nil {
				util.Warn("解析 S3 元数据失败", zap.String("key", key), zap.Error(err))
				continue
			}
			allSoftware = append(allSoftware, sw)
		}
	}
	return allSoftware, metaKeys, nil
}

func (s *SoftwareService) loadSoftwareFromS3(ctx context.Context, softwareID string) (model.SoftwareDetail, error) {
	key := fmt.Sprintf("mirrors/%s/meta.json", softwareID)
	data, err := s.s3Client.GetObject(ctx, key)
	if err != nil {
		return model.SoftwareDetail{}, fmt.Errorf("读取软件元数据失败: %w", err)
	}
	var detail model.SoftwareDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return model.SoftwareDetail{}, fmt.Errorf("解析软件元数据失败: %w", err)
	}
	return detail, nil
}

func (s *SoftwareService) listVersions(ctx context.Context, softwareID string) ([]model.VersionBrief, error) {
	key := softwareDetailKeyPrefix + softwareID
	var detail model.SoftwareDetail
	found, err := util.GetJSON(ctx, key, &detail)
	if err != nil || !found {
		return nil, fmt.Errorf("版本列表不存在: %s", softwareID)
	}
	return detail.Versions, nil
}

func (s *SoftwareService) assembleDetail(sw model.SoftwareTable, tags []string, versions []model.VersionBrief) model.SoftwareDetail {
	return model.SoftwareDetail{
		Software: model.Software{
			ID:          sw.ID,
			Name:        sw.Name,
			GitHubRepo:  sw.GitHubRepo,
			Category:    sw.Category,
			Description: sw.Description,
			Stars:       sw.Stars,
			LatestVer:   sw.LatestVer,
			TotalAssets: len(versions),
			UpdatedAt:   sw.UpdatedAt,
		},
		Tags:          tags,
		Versions:      versions,
		TotalVersions: len(versions),
	}
}