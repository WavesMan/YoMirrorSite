// 与后端 api/model/software.go 一一对应
// 用于前端 TypeScript 类型检查

/** 软件条目概要（列表页、卡片展示） */
export interface Software {
  id: string
  name: string
  description: string
  homepage: string
  github_repo: string
  icon_url: string
  category: string
  tags: string[]
  license: string
  stars: number
  latest_ver: string
  total_assets: number
  updated_at: string
}

/** 软件详情（含版本列表） */
export interface SoftwareDetail extends Software {
  versions: VersionBrief[]
  total_versions: number
  total_size: number
  readme_md: string
}

/** 版本概要 */
export interface VersionBrief {
  tag_name: string
  name: string
  prerelease: boolean
  published_at: string
  asset_count: number
}

/** 版本详情（含资产列表和下载 URL） */
export interface VersionDetail {
  tag_name: string
  name: string
  body: string
  prerelease: boolean
  published_at: string
  assets: AssetInfo[]
}

/** 可下载资产文件 */
export interface AssetInfo {
  name: string
  size: number
  size_human: string
  platform: string
  content_type: string
  download_url: string
  checksum: string
  downloads: number
}

/** 镜像站统计 */
export interface MirrorStats {
  total_software: number
  total_versions: number
  total_assets: number
  total_size: number
  total_downloads: number
  last_sync_at: string
  sync_in_progress: boolean
}

/** 同步状态 */
export interface SyncStatus {
  in_progress: boolean
  current_job: string
  last_sync_at: string
  last_result: SyncResultBrief | null
}

export interface SyncResultBrief {
  software_id: string
  new_versions: number
  new_assets: number
  errors: string[]
  duration: string
}

/** 统一 API 响应包装 */
export interface APIResponse<T = unknown> {
  success: boolean
  data?: T
  error?: string
  total?: number
}

/** 分页列表 */
export interface SoftwareListPage {
  items: Software[]
  page: number
  page_size: number
  total_count: number
}
