// GitHub Release 同步器
// 负责将 GitHub Release 资产增量同步到 S3 对象存储
// 核心流程：
//   1. 获取分布式锁 lock:sync:{software_id} 防止多实例重复
//   2. 从 Redis 读取已同步的最新 tag (last_synced_tag)
//   3. 分页拉取 GitHub Releases，仅处理新版本
//   4. 逐个资产流式下载 → S3 PutObject（不落磁盘）
//   5. 写入版本/资产元数据到 Redis + S3
//   6. 更新 last_synced_tag
//   7. 释放分布式锁

package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/config"
	"yomirrorsite/internal/core/github"
	"yomirrorsite/internal/core/postgres"
	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

)

const (
	mirrorsPathPrefix = "mirrors/"
	lockPrefix = "lock:sync:"
	lastSyncedKeyPrefix = "last_synced:software:"
	softwareMetaKey = "mirrors/%s/meta.json"
	versionMetaKey = "mirrors/%s/versions/%s/meta.json"
	latestTagKey = "mirrors/%s/versions/latest.txt"
	lockTTL = 30 * time.Minute
	assetUploadTimeout = 20 * time.Minute
	maxRetries = 3
)

type GitHubSyncer struct {
	ghClient    *github.Client
	s3Client    *s3.Client
	config      *config.MirrorConfig
	pgClient    *postgres.Client
	mu          sync.RWMutex
	inProgress  bool
	currentJob  string
	lastSyncAt  time.Time
	lastResult  *model.SyncResultBrief
}

type SyncResult struct {
	SoftwareID  string
	NewVersions int
	NewAssets   int
	Skipped     int
	Failed      []string
	TotalSize   int64
	Duration    time.Duration
	Errors      []string
}

func NewGitHubSyncer(ghClient *github.Client, s3Client *s3.Client, cfg *config.MirrorConfig, pgClient *postgres.Client) *GitHubSyncer {
	return &GitHubSyncer{
		ghClient: ghClient,
		s3Client: s3Client,
		config:   cfg,
		pgClient: pgClient,
	}
}

func (s *GitHubSyncer) SyncSoftware(ctx context.Context, swCfg config.SoftwareConfig) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{SoftwareID: swCfg.ID}

	lockKey := lockPrefix + swCfg.ID
	locked, err := util.AcquireLock(ctx, lockKey, lockTTL)
	if err != nil {
		return nil, fmt.Errorf("获取同步锁失败: %w", err)
	}
	if !locked {
		util.Info("同步已被其他实例执行中，跳过",
			zap.String("software", swCfg.ID))
		return result, nil
	}
	defer util.ReleaseLock(ctx, lockKey)

	s.setStatus(true, swCfg.ID)

	lastSyncedTag, err := s.getLastSyncedTag(ctx, swCfg.ID)
	if err != nil {
		util.Warn("获取已同步标记失败，将全量同步",
			zap.String("software", swCfg.ID), zap.Error(err))
	}

	owner, repo := parseRepo(swCfg.GitHubRepo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("无效的仓库路径: %s", swCfg.GitHubRepo)
	}

	releases, err := s.ghClient.ListAllReleases(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("获取 Releases 失败: %w", err)
	}

	util.Info("获取到 Releases",
		zap.String("software", swCfg.ID),
		zap.Int("total", len(releases)),
		zap.String("last_synced_tag", lastSyncedTag))

	for i, j := 0, len(releases)-1; i < j; i, j = i+1, j-1 {
		releases[i], releases[j] = releases[j], releases[i]
	}

	newestTag := lastSyncedTag
	for _, release := range releases {
		if lastSyncedTag != "" && compareSemVer(release.TagName, lastSyncedTag) <= 0 {
			continue
		}

		if release.Prerelease && !swCfg.SyncPrerelease {
			util.Debug("跳过预发布版本",
				zap.String("software", swCfg.ID),
				zap.String("tag", release.TagName))
			continue
		}

		var versionID int
		if s.pgClient != nil && s.pgClient.IsEnabled() {
			versionID, _ = s.pgClient.UpsertVersion(ctx, &model.VersionTable{
				SoftwareID:  swCfg.ID,
				TagName:     release.TagName,
				Name:        release.Name,
				Prerelease:  release.Prerelease,
				PublishedAt: release.PublishedAt,
				Body:        release.Body,
			})
		}

		assetsUploaded := s.syncReleaseAssets(ctx, swCfg, &release, result, versionID)
		if assetsUploaded > 0 {
			s.saveVersionMeta(ctx, swCfg.ID, &release)
			result.NewVersions++
			if compareSemVer(release.TagName, newestTag) > 0 {
				newestTag = release.TagName
			}
		}
	}

	if newestTag > lastSyncedTag || (lastSyncedTag == "" && newestTag != "") {
		if err := s.setLastSyncedTag(ctx, swCfg.ID, newestTag); err != nil {
			util.Error("更新已同步标记失败",
				zap.String("software", swCfg.ID), zap.Error(err))
			result.Errors = append(result.Errors, "更新同步标记失败: "+err.Error())
		}
		_ = s.setLatestTag(ctx, swCfg.ID, newestTag)
	}

	s.mu.Lock()
	s.lastSyncAt = time.Now()
	s.mu.Unlock()

	s.writeToPG(ctx, swCfg, result, newestTag)

	result.Duration = time.Since(startTime)
	s.setStatus(false, "")

	s.lastResult = &model.SyncResultBrief{
		SoftwareID:  swCfg.ID,
		NewVersions: result.NewVersions,
		NewAssets:   result.NewAssets,
		Errors:      result.Errors,
		Duration:    result.Duration.Round(time.Second).String(),
	}

	util.Info("软件同步完成",
		zap.String("software", swCfg.ID),
		zap.Int("new_versions", result.NewVersions),
		zap.Int("new_assets", result.NewAssets),
		zap.Duration("duration", result.Duration))

	return result, nil
}

func (s *GitHubSyncer) syncReleaseAssets(ctx context.Context, swCfg config.SoftwareConfig, release *github.Release, result *SyncResult, versionID int) int {
	uploaded := 0

	for _, asset := range release.Assets {
		if asset.State != "uploaded" {
			continue
		}

		if !swCfg.MatchAssetName(asset.Name) {
			continue
		}

		s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s",
			swCfg.ID, release.TagName, asset.Name)

		exists, err := s.s3Client.ObjectExists(ctx, s3Key)
		if err != nil {
			util.Warn("检查文件存在性失败",
				zap.String("key", s3Key), zap.Error(err))
		}
		if exists {
			result.Skipped++
			util.Debug("文件已存在，跳过",
				zap.String("key", s3Key))
			continue
		}

		uploadCtx, cancel := context.WithTimeout(ctx, assetUploadTimeout)
		resp, err := s.ghClient.DownloadAsset(uploadCtx, asset.DownloadURL)
		if err != nil {
			result.Failed = append(result.Failed, asset.Name)
			cancel()
			util.Error("下载资产失败",
				zap.String("asset", asset.Name),
				zap.Error(err))
			continue
		}

		err = s.s3Client.UploadObjectWithSize(
			uploadCtx,
			s3Key,
			resp.Body,
			asset.Size,
			asset.ContentType,
		)
		resp.Body.Close()
		cancel()

		if err != nil {
			result.Failed = append(result.Failed, asset.Name)
			util.Error("上传资产到 S3 失败",
				zap.String("asset", asset.Name),
				zap.String("key", s3Key),
				zap.Error(err))
			continue
		}

		result.NewAssets++
		result.TotalSize += asset.Size
		uploaded++

		if s.pgClient != nil && s.pgClient.IsEnabled() {
			_, _ = s.pgClient.BatchInsertAssets(ctx, []model.AssetTable{{
				VersionID:   versionID,
				Name:        asset.Name,
				Size:        asset.Size,
				ContentType: asset.ContentType,
				Platform:    swCfg.GetAssetFilter(asset.Name),
				S3Key:       s3Key,
				Checksum:    "",
			}})
		}

		util.Info("资产同步成功",
			zap.String("software", swCfg.ID),
			zap.String("version", release.TagName),
			zap.String("asset", asset.Name),
			zap.Int64("size", asset.Size))
	}

	return uploaded
}

func (s *GitHubSyncer) saveVersionMeta(ctx context.Context, softwareID string, release *github.Release) {
	key := fmt.Sprintf("software:%s:version:%s", softwareID, release.TagName)

	assets := make([]model.AssetInfo, 0, len(release.Assets))
	for _, a := range release.Assets {
		assets = append(assets, model.AssetInfo{
			Name:        a.Name,
			Size:        a.Size,
			SizeHuman:   model.FormatSize(a.Size),
			ContentType: a.ContentType,
		})
	}

	detail := model.VersionDetail{
		TagName:     release.TagName,
		Name:        release.Name,
		Body:        release.Body,
		Prerelease:  release.Prerelease,
		PublishedAt: release.PublishedAt.Format(time.RFC3339),
		Assets:      assets,
	}

	if err := util.SetJSON(ctx, key, detail, 24*time.Hour); err != nil {
		util.Warn("保存版本元数据失败",
			zap.String("key", key), zap.Error(err))
	}
}

func (s *GitHubSyncer) getLastSyncedTag(ctx context.Context, softwareID string) (string, error) {
	key := lastSyncedKeyPrefix + softwareID
	tag, err := util.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return tag, nil
}

func (s *GitHubSyncer) setLastSyncedTag(ctx context.Context, softwareID, tag string) error {
	key := lastSyncedKeyPrefix + softwareID
	return util.SetWithExpiration(ctx, key, tag, 0)
}

func (s *GitHubSyncer) setLatestTag(ctx context.Context, softwareID, tag string) error {
	key := fmt.Sprintf(latestTagKey, softwareID)
	return util.SetWithExpiration(ctx, key, tag, 0)
}

func (s *GitHubSyncer) setStatus(inProgress bool, job string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inProgress = inProgress
	s.currentJob = job
}

func (s *GitHubSyncer) GetStatus() *model.SyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &model.SyncStatus{
		InProgress: s.inProgress,
		CurrentJob: s.currentJob,
		LastSyncAt: s.lastSyncAt,
		LastResult: s.lastResult,
	}
}

func parseRepo(repo string) (owner, repoName string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func ComputeChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func compareSemVer(a, b string) int {
	parse := func(s string) []int {
		s = strings.TrimPrefix(s, "v")
		parts := strings.Split(s, ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			nums[i], _ = strconv.Atoi(p)
		}
		return nums
	}
	va, vb := parse(a), parse(b)
	maxLen := len(va)
	if len(vb) > maxLen {
		maxLen = len(vb)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(va) {
			av = va[i]
		}
		if i < len(vb) {
			bv = vb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func MirrorPath(softwareID, tagName, assetName string) string {
	return path.Join(mirrorsPathPrefix, softwareID, "versions", tagName, assetName)
}

func (s *GitHubSyncer) writeToPG(ctx context.Context, swCfg config.SoftwareConfig, result *SyncResult, newestTag string) {
	if s.pgClient == nil || !s.pgClient.IsEnabled() {
		return
	}

	logID, err := s.pgClient.CreateSyncLog(ctx, swCfg.ID)
	if err != nil {
		util.Warn("创建同步日志失败", zap.String("software", swCfg.ID), zap.Error(err))
	}

	if result.NewVersions > 0 || result.NewAssets > 0 {
		sw := &model.SoftwareTable{
			ID:         swCfg.ID,
			Name:       swCfg.Name,
			GitHubRepo: swCfg.GitHubRepo,
			Category:   swCfg.Category,
			LatestVer:  newestTag,
			TotalSize:  0,
		}
		if err := s.pgClient.UpsertSoftware(ctx, sw); err != nil {
			util.Warn("PG 更新软件信息失败", zap.String("software", swCfg.ID), zap.Error(err))
		}
		if len(swCfg.Tags) > 0 {
			_ = s.pgClient.SaveTags(ctx, swCfg.ID, swCfg.Tags)
		}
	}

	if logID > 0 {
		status := "success"
		errMsg := ""
		if len(result.Failed) > 0 {
			status = "partial"
			errMsg = fmt.Sprintf("部分资产失败: %v", result.Failed)
		}
		if len(result.Errors) > 0 {
			status = "failed"
			errMsg = strings.Join(result.Errors, "; ")
		}
		_ = s.pgClient.FinishSyncLog(ctx, logID, status,
			result.NewVersions, result.NewAssets, result.Skipped, result.TotalSize, errMsg)
	}
}