// API 请求封装层
// 使用 ofetch，自动 JSON 解析，统一错误处理

import { ofetch } from 'ofetch'
import type { APIResponse, Software, SoftwareDetail, VersionDetail, MirrorStats, SyncStatus, SoftwareListPage } from '@/types'

const api = ofetch.create({
  baseURL: '/api',
  onResponseError({ response }) {
    console.error(`API 请求失败: ${response.status}`, response._data)
  },
})

// ============================================================
// 软件镜像 API
// ============================================================

/** 获取软件列表（支持分类筛选、关键词搜索、分页） */
export const fetchSoftwareList = (params: {
  category?: string
  keyword?: string
  page?: number
  page_size?: number
}) => api<APIResponse<SoftwareListPage>>('/mirror/software', { query: params })

/** 获取软件详情（含版本列表） */
export const fetchSoftware = (id: string) =>
  api<APIResponse<SoftwareDetail>>(`/mirror/software/${id}`)

/** 获取版本详情（含下载 URL） */
export const fetchVersion = (id: string, tag: string, expires?: number) =>
  api<APIResponse<VersionDetail>>(`/mirror/software/${id}/versions/${tag}`, {
    query: expires ? { expires } : undefined,
  })

/** 获取镜像站统计 */
export const fetchStats = () =>
  api<APIResponse<MirrorStats>>('/mirror/stats')

// ============================================================
// 同步管理 API
// ============================================================

/** 获取同步状态 */
export const fetchSyncStatus = () =>
  api<APIResponse<SyncStatus>>('/sync/status')

/** 手动触发同步（softwareId 为空则全量同步） */
export const triggerSync = (softwareId?: string) =>
  api<APIResponse<{ message: string }>>('/sync/trigger', {
    method: 'POST',
    body: softwareId ? { software_id: softwareId } : {},
  })
