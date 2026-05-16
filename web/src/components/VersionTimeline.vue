<!--
  @fileoverview 版本时间线组件 — 展示软件版本发布历史

  数据流：
    接收 props: { versions: VersionBrief[] }
    触发 emit:  { select: [version: VersionBrief] }
    依赖：@/types (VersionBrief 接口)、dayjs (日期格式化)

  状态：无内部状态

  视觉参考：Demo App.tsx 的版本历史列表 —— 首个版本高亮 + Latest 标签
-->
<script setup lang="ts">
import type { VersionBrief } from '@/types'
import dayjs from 'dayjs'

defineProps<{ versions: VersionBrief[] }>()
const emit = defineEmits<{ select: [version: VersionBrief] }>()
</script>

<template>
  <!-- 版本列表 — Demo 风格条目 -->
  <div v-if="versions.length > 0" class="space-y-3">
    <div
      v-for="(ver, i) in versions"
      :key="ver.tag_name"
      :class="[
        'flex items-start justify-between p-4 rounded-xl cursor-pointer transition-all',
        i === 0
          ? 'bg-zinc-50 border border-zinc-200'
          : 'opacity-60 hover:opacity-100 hover:bg-zinc-50',
      ]"
      @click="emit('select', ver)"
    >
      <!-- 左侧：版本号 + 日期 + 标签 -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-3 mb-1">
          <span class="font-bold text-sm">{{ ver.tag_name }}</span>
          <!-- 首个版本标 Latest -->
          <span
            v-if="i === 0"
            class="px-2 py-0.5 rounded-full bg-black text-[8px] text-white font-bold uppercase tracking-widest"
          >
            Latest
          </span>
          <!-- 预发布标 Pre-release -->
          <span
            v-if="ver.prerelease"
            class="px-2 py-0.5 rounded-full bg-amber-100 text-[8px] text-amber-700 font-bold uppercase tracking-widest"
          >
            预发布
          </span>
        </div>
        <p class="text-xs text-zinc-400">
          {{ dayjs(ver.published_at).format('YYYY-MM-DD') }}
        </p>
        <!-- 文件数量 -->
        <p class="text-xs text-zinc-400 mt-1">
          {{ ver.asset_count }} 个文件
        </p>
      </div>

      <!-- 右侧：镜像下载按钮 -->
      <button
        class="flex items-center gap-2 px-4 py-2 bg-white border border-zinc-200 rounded-lg text-xs font-bold hover:bg-zinc-100 transition-colors ml-4 flex-shrink-0"
        @click.stop="emit('select', ver)"
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
        镜像下载
      </button>
    </div>
  </div>

  <!-- 空态 -->
  <div v-else class="text-center py-8 text-zinc-400 text-sm">
    暂无版本记录
  </div>
</template>
