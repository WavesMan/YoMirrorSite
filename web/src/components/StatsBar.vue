<!--
  @fileoverview 镜像站统计条 — 展示已镜像软件数、版本总数、累计下载、总存储占用

  数据流：
    无 props/emit（纯展示组件，数据来自 Pinia store）
    依赖：@/stores/mirror (useMirrorStore → loadStats / stats)

  状态：通过 Pinia store 间接持有（store.stats）
-->
<script setup lang="ts">
import { useMirrorStore } from '@/stores/mirror'
import { onMounted } from 'vue'

const store = useMirrorStore()
// 挂载时异步拉取统计数据
onMounted(() => { store.loadStats() })
</script>

<template>
  <div v-if="store.stats" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
    <div class="stat-card">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-full bg-emerald-500" />
        <span class="text-caption">已镜像软件</span>
      </div>
      <p class="text-2xl font-display font-bold">{{ store.stats.total_software }}</p>
    </div>
    <div class="stat-card">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-full bg-blue-500" />
        <span class="text-caption">版本总数</span>
      </div>
      <p class="text-2xl font-display font-bold">{{ store.stats.total_versions || 0 }}</p>
    </div>
    <div class="stat-card">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-full bg-purple-500" />
        <span class="text-caption">累计下载</span>
      </div>
      <p class="text-2xl font-display font-bold">{{ (store.stats.total_downloads || 0).toLocaleString() }}</p>
    </div>
    <div class="stat-card">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-full bg-amber-500" />
        <span class="text-caption">总存储占用</span>
      </div>
      <p class="text-2xl font-display font-bold">{{ ((store.stats.total_size || 0) / 1024 / 1024 / 1024).toFixed(1) }} GB</p>
    </div>
  </div>
  <n-skeleton v-else :repeat="1" text />
</template>
