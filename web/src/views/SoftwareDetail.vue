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
    <n-skeleton v-if="loading" :repeat="3" />
    <n-result v-else-if="error" status="error" :title="error" />

    <template v-else-if="detail">
      <!-- 软件头信息 -->
      <div class="flex items-start gap-4 mb-8">
        <n-avatar :src="detail.icon_url" :size="64" round v-if="detail.icon_url" />
        <div class="flex-1">
          <h1 class="text-3xl font-bold">{{ detail.name }}</h1>
          <div class="flex items-center gap-3 mt-2 text-gray-500">
            <span>⭐ {{ detail.stars.toLocaleString() }}</span>
            <n-tag>{{ detail.category }}</n-tag>
            <span v-if="detail.license">{{ detail.license }}</span>
            <a v-if="detail.github_repo" :href="`https://github.com/${detail.github_repo}`" target="_blank" class="text-blue-500">
              GitHub
            </a>
          </div>
          <p class="mt-3 text-gray-600">{{ detail.description }}</p>
        </div>
      </div>

      <n-divider />

      <!-- README + 版本 -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <!-- 左侧 README -->
        <div class="lg:col-span-2">
          <n-card title="项目简介" v-if="detail.readme_md">
            <div class="prose prose-sm max-w-none" v-html="md.render(detail.readme_md)" />
          </n-card>

          <!-- 选中版本的下载区 -->
          <n-card title="文件下载" class="mt-4" v-if="selectedVersion">
            <template #header-extra>
              <n-tag :type="selectedVersion.prerelease ? 'warning' : 'success'">
                {{ selectedVersion.tag_name }}
              </n-tag>
            </template>
            <AssetDownload :assets="selectedVersion.assets" />
          </n-card>
        </div>

        <!-- 右侧版本时间线 -->
        <div>
          <n-card title="版本历史" size="small">
            <VersionTimeline
              :versions="detail.versions"
              @select="loadVersion"
            />
          </n-card>
        </div>
      </div>
    </template>
  </div>
</template>
