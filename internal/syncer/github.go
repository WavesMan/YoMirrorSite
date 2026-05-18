// GitHub Release 同步器
// 负责将 GitHub Release 资产同步到 S3 对象存储
// 支持三种同步规则（按软件仓库分别控制）：
//   incremental:     增量同步 — 从上次同步的 tag 之后只拉取新版本（默认）
//   latest_only:     只保留最新 — 增量同步后清理旧版本，仅保留最近 N 个
//   full_historical: 全量历史 — 无视 last_synced_tag，拉取所有 Release（新→旧）
// 核心流程：
//   1. 获取分布式锁 lock:sync:{software_id} 防止多实例重复
//   2. 根据同步规则选择策略
//   3. 分页拉取 GitHub Releases
//   4. 逐个资产流式下载 → S3 PutObject（不落磁盘）
//   5. 写入版本/资产元数据到 Redis + S3
//   6. 更新 last_synced_tag
//   7. [latest_only] 清理超出保留个数的旧版本
//   8. 释放分布式锁

package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/config"
	"yomirrorsite/internal/core/github"
	"yomirrorsite/internal/core/postgres"
	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

)

// ============================================================
// 常量定义
// ============================================================

const (
	// mirrorsPathPrefix S3 中镜像文件存储路径前缀
	mirrorsPathPrefix = "mirrors/"
	// lockPrefix Redis 分布式锁 key 前缀（复用 redis_util.go 的 AcquireLock）
	lockPrefix = "lock:sync:"
	// lastSyncedKeyPrefix 记录每个软件最后同步的 tag
	lastSyncedKeyPrefix = "last_synced:software:"
	// softwareMetaKey 软件元数据 S3 key 模板
	softwareMetaKey = "mirrors/%s/meta.json"
	// versionMetaKey 版本元数据 S3 key 模板
	versionMetaKey = "mirrors/%s/versions/%s/meta.json"
	// latestTagKey 最新版本 tag 记录 key 模板
	latestTagKey = "mirrors/%s/versions/latest.txt"
	// lockTTL 分布式锁 TTL
	lockTTL = 30 * time.Minute
	// assetUploadTimeout 单个资产上传超时
	assetUploadTimeout = 20 * time.Minute
	// maxRetries 下载失败重试次数
	maxRetries = 3
)

// ============================================================
// 同步器结构
// ============================================================

// GitHubSyncer GitHub Release 同步器
// 将指定软件的 Release 资产镜像到 S3，支持增量检测和并发控制
type GitHubSyncer struct {
	ghClient    *github.Client        // GitHub API 客户端
	s3Client    *s3.Client            // S3 对象存储客户端
	config      *config.MirrorConfig  // 镜像站配置
	pgClient    *postgres.Client      // PG 持久化层（可选，nil 时跳过）

	// 同步状态（内存中维护，外部通过 SyncStatus API 查询）
	mu          sync.RWMutex
	inProgress  bool                  // 是否有同步进行中
	currentJob  string                // 当前同步的软件 ID
	lastSyncAt  time.Time             // 最近同步完成时间
	lastResult  *model.SyncResultBrief // 最近同步结果
}

// SyncResult 单次同步的完整结果
type SyncResult struct {
	SoftwareID  string
	NewVersions int      // 新发现的版本数
	NewAssets   int      // 成功上传的资产数
	Skipped     int      // 已存在跳过的资产数
	Failed      []string // 失败的资产名称列表
	TotalSize   int64    // 本次上传总大小（字节）
	Duration    time.Duration
	Errors      []string // 非致命错误信息
}

// ============================================================
// 初始化
// ============================================================

// NewGitHubSyncer 创建同步器
// 使用依赖注入：ghClient 负责 API 调用，s3Client 负责存储，pgClient 负责持久化
func NewGitHubSyncer(ghClient *github.Client, s3Client *s3.Client, cfg *config.MirrorConfig, pgClient *postgres.Client) *GitHubSyncer {
	return &GitHubSyncer{
		ghClient: ghClient,
		s3Client: s3Client,
		config:   cfg,
		pgClient: pgClient,
	}
}

// ============================================================
// 核心同步流程（策略分派入口）
// ============================================================

// SyncSoftware 同步单个软件（按配置的 sync_rule 选择策略）
// 三种策略：
//   incremental:     增量同步（默认，向后兼容）
//   latest_only:     增量 + 清理旧版本，仅保留最近 keep_versions 个
//   full_historical: 全量拉取所有 Release（新→旧），可选首次后切回增量
func (s *GitHubSyncer) SyncSoftware(ctx context.Context, swCfg config.SoftwareConfig) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{SoftwareID: swCfg.ID}

	// 1. 获取分布式锁，防止多实例同时同步同一软件
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

	// 更新同步状态
	s.setStatus(true, swCfg.ID)

	// 2. 根据同步规则选择策略
	switch swCfg.SyncRule {
	case config.SyncRuleIncremental, "":
		// 规则 1：增量同步（当前行为）
		result, err = s.syncIncremental(ctx, swCfg, result)

	case config.SyncRuleLatestOnly:
		// 规则 2：增量同步 + 后清理旧版本
		result, err = s.syncIncremental(ctx, swCfg, result)
		if err == nil {
			s.cleanupOldVersions(ctx, swCfg, swCfg.KeepVersions)
		}

	case config.SyncRuleFullHistorical:
		// 规则 3：全量历史同步（新→旧）
		result, err = s.syncFullHistorical(ctx, swCfg, result)
		// 如果配置了 full_sync_once，完成全量后更新标记切回增量模式
		if err == nil && swCfg.FullSyncOnce {
			newestTag := s.extractNewestTag(result)
			if newestTag != "" {
				_ = s.setLastSyncedTag(ctx, swCfg.ID, newestTag)
			}
		}

	default:
		// 未知规则，回退增量（防御）
		util.Warn("未知的同步规则，回退增量模式",
			zap.String("software", swCfg.ID),
			zap.String("sync_rule", string(swCfg.SyncRule)))
		result, err = s.syncIncremental(ctx, swCfg, result)
	}

	// 3. 后处理（PG 写入 + 状态更新）
	if err == nil {
		newestTag := s.extractNewestTag(result)
		s.writeToPG(ctx, swCfg, result, newestTag)
	}

	// 4. 更新最后同步时间
	s.mu.Lock()
	s.lastSyncAt = time.Now()
	s.mu.Unlock()

	result.Duration = time.Since(startTime)
	s.setStatus(false, "")

	// 保存同步结果
	s.lastResult = &model.SyncResultBrief{
		SoftwareID:  swCfg.ID,
		NewVersions: result.NewVersions,
		NewAssets:   result.NewAssets,
		Errors:      result.Errors,
		Duration:    result.Duration.Round(time.Second).String(),
	}

	util.Info("软件同步完成",
		zap.String("software", swCfg.ID),
		zap.String("sync_rule", string(swCfg.SyncRule)),
		zap.Int("new_versions", result.NewVersions),
		zap.Int("new_assets", result.NewAssets),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// ============================================================
// 策略 1：增量同步（原 SyncSoftware 核心逻辑）
// ============================================================

// syncIncremental 增量同步：从 last_synced_tag 之后只拉取新版本
func (s *GitHubSyncer) syncIncremental(ctx context.Context, swCfg config.SoftwareConfig, result *SyncResult) (*SyncResult, error) {
	// 获取已同步的最新 tag
	lastSyncedTag, err := s.getLastSyncedTag(ctx, swCfg.ID)
	if err != nil {
		util.Warn("获取已同步标记失败，将全量同步",
			zap.String("software", swCfg.ID), zap.Error(err))
	}

	// 拉取 GitHub Releases
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

	// 遍历 Release（GitHub 返回按时间倒序，需从旧到新处理以保证最新 tag 正确）
	// 先反转顺序
	for i, j := 0, len(releases)-1; i < j; i, j = i+1, j-1 {
		releases[i], releases[j] = releases[j], releases[i]
	}

	newestTag := lastSyncedTag
	for _, release := range releases {
		// 跳过已同步的版本（语义版本号比较，避免字典序误判如 1.9.0 > 1.10.0）
		if lastSyncedTag != "" && compareSemVer(release.TagName, lastSyncedTag) <= 0 {
			continue
		}

		// 跳过预发布版本（如果配置不要求同步）
		if release.Prerelease && !swCfg.SyncPrerelease {
			util.Debug("跳过预发布版本",
				zap.String("software", swCfg.ID),
				zap.String("tag", release.TagName))
			continue
		}

		// PG 持久化：先 upsert version 获取 ID，再同步资产时关联
		var versionID int
		if s.pgClient != nil && s.pgClient.IsEnabled() {
			vid, err := s.pgClient.UpsertVersion(ctx, &model.VersionTable{
				SoftwareID:  swCfg.ID,
				TagName:     release.TagName,
				Name:        release.Name,
				Prerelease:  release.Prerelease,
				PublishedAt: release.PublishedAt,
				Body:        release.Body,
			})
			if err != nil {
				util.Warn("UpsertVersion 失败，资产外键将为空",
					zap.String("software", swCfg.ID),
					zap.String("tag", release.TagName),
					zap.Error(err))
			}
			versionID = vid
		}

		// 同步该版本的资产
		assetsUploaded := s.syncReleaseAssets(ctx, swCfg, &release, result, versionID)
		if assetsUploaded > 0 {
			// 写入版本元数据到 Redis
			s.saveVersionMeta(ctx, swCfg.ID, &release)
			result.NewVersions++
			if compareSemVer(release.TagName, newestTag) > 0 {
				newestTag = release.TagName
			}
		}
	}

	// 更新已同步标记
	if newestTag > lastSyncedTag || (lastSyncedTag == "" && newestTag != "") {
		if err := s.setLastSyncedTag(ctx, swCfg.ID, newestTag); err != nil {
			util.Error("更新已同步标记失败",
				zap.String("software", swCfg.ID), zap.Error(err))
			result.Errors = append(result.Errors, "更新同步标记失败: "+err.Error())
		}

		// 更新最新版本标记
		_ = s.setLatestTag(ctx, swCfg.ID, newestTag)
	}

	return result, nil
}

// ============================================================
// 策略 3：全量历史同步
// ============================================================

// syncFullHistorical 全量历史同步：无视 last_synced_tag，拉取所有 Release
// 按 GitHub 原始返回顺序（新→旧）处理，但仍需从旧到新保证标记正确性
func (s *GitHubSyncer) syncFullHistorical(ctx context.Context, swCfg config.SoftwareConfig, result *SyncResult) (*SyncResult, error) {
	owner, repo := parseRepo(swCfg.GitHubRepo)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("无效的仓库路径: %s", swCfg.GitHubRepo)
	}

	releases, err := s.ghClient.ListAllReleases(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("获取 Releases 失败: %w", err)
	}

	util.Info("全量历史同步：获取到 Releases",
		zap.String("software", swCfg.ID),
		zap.Int("total", len(releases)),
		zap.String("mode", "full_historical"))

	// 从旧到新处理（与 incremental 一致，保证 tag 标记正确）
	for i, j := 0, len(releases)-1; i < j; i, j = i+1, j-1 {
		releases[i], releases[j] = releases[j], releases[i]
	}

	var newestTag string
	for _, release := range releases {
		// 跳过预发布版本（按配置）
		if release.Prerelease && !swCfg.SyncPrerelease {
			continue
		}

		// 不检查 last_synced_tag —— 全量同步所有非预发布版本

		var versionID int
		if s.pgClient != nil && s.pgClient.IsEnabled() {
			vid, err := s.pgClient.UpsertVersion(ctx, &model.VersionTable{
				SoftwareID:  swCfg.ID,
				TagName:     release.TagName,
				Name:        release.Name,
				Prerelease:  release.Prerelease,
				PublishedAt: release.PublishedAt,
				Body:        release.Body,
			})
			if err != nil {
				util.Warn("UpsertVersion 失败",
					zap.String("software", swCfg.ID),
					zap.String("tag", release.TagName),
					zap.Error(err))
			}
			versionID = vid
		}

		assetsUploaded := s.syncReleaseAssets(ctx, swCfg, &release, result, versionID)
		if assetsUploaded > 0 {
			s.saveVersionMeta(ctx, swCfg.ID, &release)
			result.NewVersions++
			if newestTag == "" || compareSemVer(release.TagName, newestTag) > 0 {
				newestTag = release.TagName
			}
		}
	}

	// 更新标记
	if newestTag != "" {
		_ = s.setLastSyncedTag(ctx, swCfg.ID, newestTag)
		_ = s.setLatestTag(ctx, swCfg.ID, newestTag)
	}

	return result, nil
}

// ============================================================
// syncReleaseAssets（共享：同步单个 Release 的资产）
// ============================================================

// syncReleaseAssets 同步一个 Release 的所有资产文件
// 返回成功上传的资产数量
func (s *GitHubSyncer) syncReleaseAssets(ctx context.Context, swCfg config.SoftwareConfig, release *github.Release, result *SyncResult, versionID int) int {
	uploaded := 0

	for _, asset := range release.Assets {
		// 状态检查：跳过未上传完成的资产
		if asset.State != "uploaded" {
			continue
		}

		// 过滤规则检查
		if !swCfg.MatchAssetName(asset.Name) {
			continue
		}

		// 构建 S3 存储路径
		s3Key := fmt.Sprintf("mirrors/%s/versions/%s/%s",
			swCfg.ID, release.TagName, asset.Name)

		// 检查是否已存在于 S3
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

		// 下载资产
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

		// 流式上传到 S3（不落磁盘）
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

		// PG 持久化：每条资产上传后立即 INSERT（幂等：s3_key 唯一）
		if s.pgClient != nil && s.pgClient.IsEnabled() {
			_, _ = s.pgClient.BatchInsertAssets(ctx, []model.AssetTable{{
				VersionID:   versionID,
				Name:        asset.Name,
				Size:        asset.Size,
				ContentType: asset.ContentType,
				Platform:    swCfg.GetAssetFilter(asset.Name),
				S3Key:       s3Key,
				Checksum:    "", // TODO: 流式上传后无法回算，对账时补
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

// ============================================================
// 策略 2：版本清理（latest_only）
// ============================================================

// cleanupOldVersions 清理旧版本，仅保留最近 keepN 个版本
// 在增量同步完成后调用，分布式锁内执行（安全）
// 清理范围：S3 对象 + Redis 元数据 + PG 记录
func (s *GitHubSyncer) cleanupOldVersions(ctx context.Context, swCfg config.SoftwareConfig, keepN int) {
	if keepN <= 0 {
		keepN = 1 // 防御：至少保留 1 个版本
	}

	// 1. 列举 S3 中该软件的所有版本目录
	versionTags, err := s.listVersionsFromS3(ctx, swCfg.ID)
	if err != nil {
		util.Warn("列举 S3 版本目录失败，跳过清理",
			zap.String("software", swCfg.ID), zap.Error(err))
		return
	}

	if len(versionTags) <= keepN {
		util.Debug("版本数未超过保留上限，无需清理",
			zap.String("software", swCfg.ID),
			zap.Int("versions", len(versionTags)),
			zap.Int("keep", keepN))
		return
	}

	// 2. 按语义版本排序（降序，最新在前）
	sort.Slice(versionTags, func(i, j int) bool {
		return compareSemVer(versionTags[i], versionTags[j]) > 0
	})

	// 3. 保留最新的 keepN 个，删除其余
	toDelete := versionTags[keepN:]

	util.Info("开始清理旧版本",
		zap.String("software", swCfg.ID),
		zap.Int("total_versions", len(versionTags)),
		zap.Int("keep", keepN),
		zap.Int("delete", len(toDelete)),
		zap.Strings("to_delete", toDelete))

	prefix := fmt.Sprintf("mirrors/%s/versions/", swCfg.ID)

	for _, tag := range toDelete {
		// 3a. 删除 S3 中该版本目录下所有对象
		versionPrefix := prefix + tag + "/"
		objects, err := s.s3Client.ListObjects(ctx, versionPrefix)
		if err != nil {
			util.Warn("列举 S3 版本对象失败",
				zap.String("software", swCfg.ID),
				zap.String("tag", tag),
				zap.Error(err))
			continue
		}

		// 批量删除 S3 对象
		var objectIds []s3types.ObjectIdentifier
		for _, obj := range objects {
			objectIds = append(objectIds, s3types.ObjectIdentifier{Key: obj.Key})
		}
		if len(objectIds) > 0 {
			_, err := s.s3Client.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
				Bucket: aws.String(s.s3Client.BucketName()),
				Delete: &s3types.Delete{Objects: objectIds, Quiet: aws.Bool(true)},
			})
			if err != nil {
				util.Warn("批量删除 S3 对象失败",
					zap.String("software", swCfg.ID),
					zap.String("tag", tag),
					zap.Error(err))
			}
		}

		// 3b. 清理 Redis 版本元数据
		redisKey := fmt.Sprintf("software:%s:version:%s", swCfg.ID, tag)
		_ = util.Delete(ctx, redisKey)

		// 3c. 清理 PG 版本 + 资产记录（如果启用）
		if s.pgClient != nil && s.pgClient.IsEnabled() {
			_ = s.pgClient.DeleteOldVersions(ctx, swCfg.ID, []string{tag})
		}
	}

	util.Info("旧版本清理完成",
		zap.String("software", swCfg.ID),
		zap.Int("deleted", len(toDelete)))
}

// listVersionsFromS3 列举 S3 中某软件的所有版本 tag
// 通过列举 mirrors/{softwareID}/versions/ 前缀的一级子目录获取
func (s *GitHubSyncer) listVersionsFromS3(ctx context.Context, softwareID string) ([]string, error) {
	prefix := fmt.Sprintf("mirrors/%s/versions/", softwareID)
	objects, err := s.s3Client.ListObjects(ctx, prefix)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var tags []string
	for _, obj := range objects {
		// Key 格式: mirrors/{softwareID}/versions/{tag}/filename
		// 截取 versions/ 和下一个 / 之间的部分作为 tag
		key := obj.Key
		idx := strings.Index(key, "versions/")
		if idx < 0 {
			continue
		}
		rest := key[idx+len("versions/"):]
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		tag := rest[:slashIdx]
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// ============================================================
// 元数据管理
// ============================================================

// saveVersionMeta 保存版本元数据到 Redis
// Key: software:vscode:version:v1.85.0
// Value: JSON 格式的版本详情
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

// getLastSyncedTag 从 Redis 读取已同步的最新 tag
func (s *GitHubSyncer) getLastSyncedTag(ctx context.Context, softwareID string) (string, error) {
	key := lastSyncedKeyPrefix + softwareID
	tag, err := util.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return tag, nil
}

// setLastSyncedTag 更新 Redis 中的已同步 tag
func (s *GitHubSyncer) setLastSyncedTag(ctx context.Context, softwareID, tag string) error {
	key := lastSyncedKeyPrefix + softwareID
	return util.SetWithExpiration(ctx, key, tag, 0) // 0 = 永不过期
}

// setLatestTag 更新最新版本标记
func (s *GitHubSyncer) setLatestTag(ctx context.Context, softwareID, tag string) error {
	key := fmt.Sprintf(latestTagKey, softwareID)
	return util.SetWithExpiration(ctx, key, tag, 0)
}

// ============================================================
// 状态管理
// ============================================================

// setStatus 更新同步状态（线程安全）
func (s *GitHubSyncer) setStatus(inProgress bool, job string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inProgress = inProgress
	s.currentJob = job
}

// GetStatus 获取当前同步状态
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

// ============================================================
// 辅助函数
// ============================================================

// parseRepo 将 "owner/repo" 格式拆分为 owner 和 repo
func parseRepo(repo string) (owner, repoName string) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// ComputeChecksum 计算数据流的 SHA256 校验和
// 用于文件完整性验证
func ComputeChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// compareSemVer 语义版本号比较
// 解析 "1.85.0" / "v1.85.0" 格式，逐段数值对比
// 返回 -1（a<b）、0（a==b）、1（a>b）
// 避免字典序比较导致的 "1.9.0" > "1.10.0" 问题
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

// MirrorPath 构建 S3 中的镜像文件路径
func MirrorPath(softwareID, tagName, assetName string) string {
	return path.Join(mirrorsPathPrefix, softwareID, "versions", tagName, assetName)
}

// extractNewestTag 从 Syncer 内部提取当前同步周期中最新的 tag
// 用于 SyncSoftware 后处理统一获取
func (s *GitHubSyncer) extractNewestTag(result *SyncResult) string {
	if result == nil {
		return ""
	}
	// 从 Redis 读取最新标记（syncIncremental/syncFullHistorical 已更新）
	tag, err := s.getLastSyncedTag(context.Background(), result.SoftwareID)
	if err != nil {
		return ""
	}
	return tag
}

// ============================================================
// PG 写入（YoMirrorSite 新增）
// ============================================================

// writeToPG 将同步结果写入 PostgreSQL
// 每资产上传时已立即 INSERT asset，此处汇总 upsert software + 写 sync_log
// 强一致性：S3 上传成功 = PG asset 记录存在（逐资产紧耦合）
func (s *GitHubSyncer) writeToPG(ctx context.Context, swCfg config.SoftwareConfig, result *SyncResult, newestTag string) {
	if s.pgClient == nil || !s.pgClient.IsEnabled() {
		return
	}

	// 创建同步日志
	logID, err := s.pgClient.CreateSyncLog(ctx, swCfg.ID)
	if err != nil {
		util.Warn("创建同步日志失败", zap.String("software", swCfg.ID), zap.Error(err))
	}

	// 更新软件基本信息
	if result.NewVersions > 0 || result.NewAssets > 0 {
		sw := &model.SoftwareTable{
			ID:         swCfg.ID,
			Name:       swCfg.Name,
			GitHubRepo: swCfg.GitHubRepo,
			Category:   swCfg.Category,
			LatestVer:  newestTag,
			TotalSize:  0, // TODO: 从 PG 聚合计算
		}
		if err := s.pgClient.UpsertSoftware(ctx, sw); err != nil {
			util.Warn("PG 更新软件信息失败", zap.String("software", swCfg.ID), zap.Error(err))
		}
		// 保存标签
		if len(swCfg.Tags) > 0 {
			_ = s.pgClient.SaveTags(ctx, swCfg.ID, swCfg.Tags)
		}
	}

	// 完成同步日志
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
