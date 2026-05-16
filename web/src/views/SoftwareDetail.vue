<!--
  @fileoverview 软件详情页：基本信息 + README + 版本列表 + 文件下载

  数据流：
    route.params.id → fetchSoftware + fetchVersion → API
    无 props（页面组件通过路由和 store 获取数据）

  依赖：
    - @/stores/mirror (Pinia store)
    - @/api (fetch 函数)
    - @/types (接口定义)
    - 子组件：VersionTimeline, AssetDownload

  路由：/software/:id — software-detail
-->
<script setup lang="ts">
// 软件详情页：基本信息 + README + 版本列表 + 文件下载
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { fetchSoftware, fetchVersion } from '@/api'
import type { SoftwareDetail, VersionDetail } from '@/types'
import VersionTimeline from '@/components/VersionTimeline.vue'
import AssetDownload from '@/components/AssetDownload.vue'
import type { VersionBrief } from '@/types'
import MarkdownIt from 'markdown-it'

const route = useRoute()
const softwareId = route.params.id as string

const detail = ref<SoftwareDetail | null>(null)
const selectedVersion = ref<VersionDetail | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const md = new MarkdownIt({ breaks: true, linkify: true })

onMounted(async () => {
  try {
    const res = await fetchSoftware(softwareId)
    if (res.success && res.data) {
      detail.value = res.data
      // 自动加载最新版本
      if (res.data.versions.length > 0) {
        await loadVersion(res.data.versions[0])
      }
    } else {
      error.value = res.error || '加载失败'
    }
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})

async function loadVersion(ver: VersionBrief) {
  try {
    const res = await fetchVersion(softwareId, ver.tag_name)
    if (res.success && res.data) {
      selectedVersion.value = res.data
    }
  } catch (e) {
    console.error('加载版本失败', e)
  }
}
</script>

<template>
  <div class="page-container">
    <!-- 加载/错误态 -->
    <div v-if="loading" class="space-y-4">
      <div class="animate-pulse bg-zinc-100 rounded-2rem h-32" />
      <div class="animate-pulse bg-zinc-100 rounded-2rem h-32" />
      <div class="animate-pulse bg-zinc-100 rounded-2rem h-32" />
    </div>
    <n-result v-else-if="error" status="error" :title="error" class="py-16" />

    <template v-else-if="detail">
      <!-- 软件头信息 -->
      <div class="flex items-start gap-6 mb-10">
        <!-- 图标方块 -->
        <div class="icon-box-lg flex-shrink-0">
          <img v-if="detail.icon_url" :src="detail.icon_url" class="w-10 h-10 rounded-xl object-cover" />
          <span v-else class="text-2xl">📦</span>
        </div>

        <div class="flex-1">
          <h1 class="text-4xl font-display font-bold mb-2">{{ detail.name }}</h1>
          <div class="flex items-center gap-3 text-zinc-500 text-sm mb-3">
            <span>⭐ {{ detail.stars.toLocaleString() }}</span>
            <span class="tag-pill">{{ detail.category }}</span>
            <span v-if="detail.license">{{ detail.license }}</span>
            <a v-if="detail.github_repo"
              :href="`https://github.com/${detail.github_repo}`"
              target="_blank"
              class="flex items-center gap-1.5 text-zinc-500 hover:text-zinc-900 transition-colors">
              <span>🔗</span> GitHub
            </a>
          </div>
          <p class="text-zinc-500 leading-relaxed">{{ detail.description }}</p>
        </div>
      </div>

      <!-- 分隔线 -->
      <div class="border-t border-zinc-100 my-8" />

      <!-- README + 版本 -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <!-- 左侧 README + 下载 -->
        <div class="lg:col-span-2 space-y-6">
          <!-- README -->
          <div v-if="detail.readme_md" class="bg-white p-8 rounded-2rem border border-zinc-100">
            <h3 class="text-lg font-bold mb-4">项目简介</h3>
            <div class="prose prose-sm max-w-none" v-html="md.render(detail.readme_md)" />
          </div>

          <!-- 文件下载 -->
          <div v-if="selectedVersion" class="bg-white p-8 rounded-2rem border border-zinc-100">
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-bold">文件下载</h3>
              <span :class="selectedVersion.prerelease ? 'bg-amber-50 text-amber-700 border-amber-200' : 'bg-emerald-50 text-emerald-700 border-emerald-200'"
                class="px-3 py-1 rounded-full text-xs font-bold border">
                {{ selectedVersion.tag_name }}
              </span>
            </div>
            <AssetDownload :assets="selectedVersion.assets" />
          </div>
        </div>

        <!-- 右侧版本时间线 -->
        <div>
          <div class="bg-white p-6 rounded-2rem border border-zinc-100">
            <h4 class="text-caption mb-4">版本历史</h4>
            <VersionTimeline :versions="detail.versions" @select="loadVersion" />
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
