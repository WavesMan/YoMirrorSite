<!--
  @fileoverview 软件列表页：分类筛选 + 搜索 + 分页

  数据流：
    route.query.keyword → store.loadSoftwareList → API
    无 props（页面组件通过路由和 store 获取数据）

  依赖：
    - @/stores/mirror (Pinia store)
    - @/api (fetch 函数)
    - @/types (接口定义)
    - 子组件：SoftwareCard

  路由：/software — software-list
-->
<script setup lang="ts">
// 软件列表页：分类筛选 + 搜索 + 分页
import { useMirrorStore } from '@/stores/mirror'
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SoftwareCard from '@/components/SoftwareCard.vue'

const store = useMirrorStore()
const route = useRoute()
const router = useRouter()

const category = ref('')
const keyword = ref((route.query.keyword as string) || '')
const page = ref(1)

const categories = ['editor', 'dev-tool', 'media', 'system', 'network', 'database']

function load() {
  store.loadSoftwareList({
    category: category.value || undefined,
    keyword: keyword.value || undefined,
    page: page.value,
    page_size: 20,
  })
}

onMounted(load)
watch([category, keyword], () => { page.value = 1; load() })
</script>

<template>
  <div class="page-container">
    <h1 class="text-3xl font-display font-bold mb-6">软件列表</h1>

    <!-- 筛选栏 -->
    <div class="flex flex-wrap items-center gap-4 mb-8 p-4 bg-white/60 backdrop-blur-sm border border-zinc-100 rounded-2xl">
      <n-input
        v-model:value="keyword"
        placeholder="搜索软件..."
        clearable
        class="!max-w-xs"
        @keyup.enter="load"
      />
      <n-select
        v-model:value="category"
        :options="categories.map(c => ({ label: c, value: c }))"
        placeholder="分类筛选"
        clearable
        class="!w-40"
      />
    </div>

    <!-- 结果 -->
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div v-for="i in 3" :key="i" class="animate-pulse bg-zinc-100 rounded-2rem h-48" />
    </div>
    <div v-else-if="store.error" class="text-center py-8 text-zinc-400">{{ store.error }}</div>
    <div v-else-if="store.softwareList.length === 0" class="text-center py-8">
      <n-empty description="暂无软件" class="py-12" />
    </div>
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <SoftwareCard
        v-for="sw in store.softwareList"
        :key="sw.id"
        :software="sw"
      />
    </div>
  </div>
</template>
