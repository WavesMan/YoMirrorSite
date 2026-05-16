<script setup lang="ts">
// 首页：镜像站简介 + 统计 + 热门软件
import { useMirrorStore } from '@/stores/mirror'
import { onMounted } from 'vue'
import StatsBar from '@/components/StatsBar.vue'
import SoftwareCard from '@/components/SoftwareCard.vue'

const store = useMirrorStore()

onMounted(() => {
  store.loadSoftwareList({ page: 1, page_size: 6 })
})
</script>

<template>
  <div class="page-container">
    <!-- 镜像站简介 -->
    <div class="text-center py-12">
      <h1 class="text-4xl font-bold text-blue-600 mb-4">
        YoMirror<span class="text-gray-800">Site</span>
      </h1>
      <p class="text-lg text-gray-500 max-w-2xl mx-auto">
        自由软件镜像站 — 为国内开发者提供 GitHub Release 的高速下载镜像，
        覆盖 VS Code、Obsidian、Neovim 等常用开发工具。
      </p>
    </div>

    <!-- 统计条 -->
    <StatsBar class="mb-10" />

    <!-- 热门软件 -->
    <div class="mb-6">
      <h2 class="text-xl font-semibold mb-4">软件列表</h2>
      <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <n-skeleton v-for="i in 6" :key="i" height="120" />
      </div>
      <div v-else-if="store.error" class="text-center py-8 text-gray-500">
        {{ store.error }}
      </div>
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <SoftwareCard
          v-for="sw in store.softwareList"
          :key="sw.id"
          :software="sw"
        />
      </div>
    </div>

    <div class="text-center mt-8">
      <n-button type="primary" @click="$router.push('/software')" size="large">
        查看全部软件
      </n-button>
    </div>
  </div>
</template>
