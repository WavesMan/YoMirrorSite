<!--
  @fileoverview 首页：镜像站简介 + 统计 + 热门软件

  数据流：
    onMounted → store.loadSoftwareList({ page_size: 6 }) → API
    无 props（页面组件通过路由和 store 获取数据）

  依赖：
    - @/stores/mirror (Pinia store)
    - @/api (fetch 函数)
    - @/types (接口定义)
    - 子组件：StatsBar, SoftwareCard

  路由：/ — home
-->
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
    <div class="text-center py-16">
      <h1 class="text-5xl font-display font-bold mb-4">
        <span class="text-zinc-900">YoMirror</span><span class="text-blue-500">Site</span>
      </h1>
      <p class="text-lg text-zinc-400 max-w-2xl mx-auto leading-relaxed">
        自由软件镜像站 — 为国内开发者提供 GitHub Release 的高速下载镜像，
        覆盖 VS Code、Obsidian、Neovim 等常用开发工具。
      </p>
    </div>

    <!-- 统计条 -->
    <StatsBar class="mb-10" />

    <!-- 热门软件 -->
    <div class="mb-6">
      <h2 class="text-2xl font-display font-bold mb-4">热门软件</h2>
      <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <div v-for="i in 3" :key="i" class="animate-pulse bg-zinc-100 rounded-2rem h-48"></div>
      </div>
      <div v-else-if="store.error" class="text-center py-8 text-zinc-400">
        {{ store.error }}
      </div>
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <SoftwareCard
          v-for="sw in store.softwareList"
          :key="sw.id"
          :software="sw"
        />
      </div>
    </div>

    <div class="text-center mt-8">
      <n-button @click="$router.push('/software')" size="large"
        class="!rounded-xl !px-8 !py-3 !font-bold !bg-zinc-900 !text-white hover:!bg-black">
        查看全部软件
      </n-button>
    </div>
  </div>
</template>
