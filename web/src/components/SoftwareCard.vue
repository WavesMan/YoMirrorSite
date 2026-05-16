<!--
  @fileoverview 软件卡片组件 — 展示软件图标、名称、描述、标签与版本信息

  数据流：
    接收 props: { software: Software }
    触发 emit:  无（点击卡片通过 router.push 跳转详情页）
    依赖：@/types (Software 接口)、vue-router (useRouter)

  状态：无内部状态
-->
<script setup lang="ts">
import type { Software } from '@/types'
import { useRouter } from 'vue-router'

const props = defineProps<{ software: Software }>()
const router = useRouter()

// 点击卡片跳转软件详情页
function goDetail() {
  router.push({ name: 'software-detail', params: { id: props.software.id } })
}
</script>

<template>
  <div
    class="group bg-white p-6 rounded-card border border-zinc-100 hover:border-zinc-300 card-hover flex flex-col cursor-pointer"
    @click="goDetail"
  >
    <!-- 图标方块 + Stars 徽章 -->
    <div class="flex items-start justify-between mb-4">
      <!-- 图标方块 -->
      <div class="icon-box group-hover:scale-110 transition-transform">
        <span
          v-if="software.icon_url"
          class="w-6 h-6 rounded-lg overflow-hidden"
        >
          <img :src="software.icon_url" class="w-full h-full object-cover" />
        </span>
        <span
          v-else
          class="text-zinc-400 text-lg font-bold group-hover:text-black transition-colors"
        >
          {{ software.category ? software.category.charAt(0).toUpperCase() : '📦' }}
        </span>
      </div>

      <!-- GitHub Stars 徽章 -->
      <div class="stars-badge">
        <span>⭐</span>
        <span>{{ (software.stars / 1000).toFixed(1) }}k</span>
      </div>
    </div>

    <!-- 标题 + 描述 -->
    <h3 class="text-lg font-bold mb-1 tracking-tight truncate">{{ software.name }}</h3>
    <p class="text-zinc-500 text-xs mb-4 line-clamp-2 leading-relaxed">{{ software.description }}</p>

    <!-- 标签 pills -->
    <div class="flex flex-wrap gap-2 mb-6 mt-auto">
      <span
        v-for="(tag, idx) in (software.tags || [])"
        :key="idx"
        class="tag-pill"
      >{{ tag }}</span>
    </div>

    <!-- 底部分隔线 + 版本号 + 下载按钮 -->
    <div class="flex items-center justify-between pt-4 border-t border-zinc-50">
      <div class="flex flex-col">
        <span class="text-caption">Latest Release</span>
        <span class="text-sm font-bold">{{ software.latest_ver || '—' }}</span>
      </div>
      <button class="btn-download" @click.stop>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-4 h-4"
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
      </button>
    </div>
  </div>
</template>
