<script setup lang="ts">
// 可下载文件列表
import type { AssetInfo } from '@/types'

defineProps<{ assets: AssetInfo[] }>()
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="asset in assets"
      :key="asset.name"
      class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
    >
      <div class="flex-1 min-w-0">
        <div class="font-mono text-sm truncate" :title="asset.name">
          {{ asset.name }}
        </div>
        <div class="flex items-center gap-2 text-xs text-gray-500 mt-1">
          <n-tag size="tiny" :bordered="false">{{ asset.platform || 'unknown' }}</n-tag>
          <span>{{ asset.size_human }}</span>
          <span v-if="asset.downloads > 0">下载 {{ asset.downloads }} 次</span>
        </div>
      </div>
      <n-button
        tag="a"
        :href="asset.download_url"
        type="primary"
        size="small"
        secondary
      >
        下载
      </n-button>
    </div>
    <n-empty v-if="assets.length === 0" description="暂无文件" />
  </div>
</template>
