// Pinia 镜像站全局状态管理

import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Software, MirrorStats, SyncStatus } from '@/types'
import { fetchSoftwareList, fetchStats, fetchSyncStatus } from '@/api'

export const useMirrorStore = defineStore('mirror', () => {
  // 状态
  const softwareList = ref<Software[]>([])
  const stats = ref<MirrorStats | null>(null)
  const syncStatus = ref<SyncStatus | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // 加载软件列表
  async function loadSoftwareList(params: {
    category?: string
    keyword?: string
    page?: number
    page_size?: number
  } = {}) {
    loading.value = true
    error.value = null
    try {
      const res = await fetchSoftwareList(params)
      if (res.success && res.data) {
        softwareList.value = res.data.items
      } else {
        error.value = res.error || '加载失败'
      }
    } catch (e: any) {
      error.value = e.message || '网络错误'
    } finally {
      loading.value = false
    }
  }

  // 加载统计信息
  async function loadStats() {
    try {
      const res = await fetchStats()
      if (res.success && res.data) {
        stats.value = res.data
      }
    } catch (e) {
      console.error('加载统计失败', e)
    }
  }

  // 加载同步状态
  async function loadSyncStatus() {
    try {
      const res = await fetchSyncStatus()
      if (res.success && res.data) {
        syncStatus.value = res.data
      }
    } catch (e) {
      console.error('加载同步状态失败', e)
    }
  }

  return {
    softwareList,
    stats,
    syncStatus,
    loading,
    error,
    loadSoftwareList,
    loadStats,
    loadSyncStatus,
  }
})
