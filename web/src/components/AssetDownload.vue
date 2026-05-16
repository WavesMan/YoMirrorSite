<!--
  @fileoverview 可下载文件列表 — 展示版本包含的资源文件

  数据流：
    接收 props: { assets: AssetInfo[] }
    无 emit，下载通过 <a> 标签直接跳转 S3 预签名 URL

  视觉参考：Demo App.tsx 的文件下载行 —— 等宽文件名 + platform tag + 黑色下载按钮
-->
<script setup lang="ts">
import type { AssetInfo } from '@/types'

defineProps<{ assets: AssetInfo[] }>()
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="asset in assets"
      :key="asset.name"
      class="flex items-center justify-between p-4 bg-white border border-zinc-100 rounded-xl hover:border-zinc-300 hover:shadow-sm transition-all"
    >
      <!-- 左侧：文件信息 -->
      <div class="flex-1 min-w-0">
        <div class="font-mono text-sm truncate" :title="asset.name">
          {{ asset.name }}
        </div>
        <div class="flex items-center gap-2 mt-1.5">
          <!-- 平台标签 -->
          <span class="tag-pill">{{ asset.platform || 'unknown' }}</span>
          <!-- 文件大小 -->
          <span class="text-xs text-zinc-400">{{ asset.size_human }}</span>
          <!-- 下载次数 -->
          <span v-if="asset.downloads > 0" class="text-xs text-zinc-400">
            下载 {{ asset.downloads.toLocaleString() }} 次
          </span>
        </div>
      </div>

      <!-- 右侧：下载按钮（直接跳转 S3 预签名 URL） -->
      <a
        :href="asset.download_url"
        target="_blank"
        rel="noopener noreferrer"
        class="flex items-center gap-2 px-5 py-2.5 bg-zinc-900 text-white rounded-xl text-xs font-bold hover:bg-black transition-colors ml-4 flex-shrink-0"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-3.5 h-3.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
        </svg>
        下载
      </a>
    </div>

    <!-- 空态 -->
    <div v-if="assets.length === 0" class="text-center py-8 text-zinc-400 text-sm">
      暂无下载文件
    </div>
  </div>
</template>
