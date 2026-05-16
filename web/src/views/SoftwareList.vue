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
    <h1 class="text-2xl font-bold mb-6">软件列表</h1>

    <!-- 筛选栏 -->
    <div class="flex flex-wrap items-center gap-4 mb-6">
      <n-input
        v-model:value="keyword"
        placeholder="搜索软件..."
        clearable
        class="max-w-xs"
        @keyup.enter="load"
      />
      <n-select
        v-model:value="category"
        :options="categories.map(c => ({ label: c, value: c }))"
        placeholder="分类筛选"
        clearable
        class="w-40"
      />
    </div>

    <!-- 结果 -->
    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <n-skeleton v-for="i in 6" :key="i" height="120" />
    </div>
    <div v-else-if="store.error" class="text-center py-8 text-gray-500">{{ store.error }}</div>
    <div v-else-if="store.softwareList.length === 0" class="text-center py-8">
      <n-empty description="暂无软件" />
    </div>
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <SoftwareCard
        v-for="sw in store.softwareList"
        :key="sw.id"
        :software="sw"
      />
    </div>
  </div>
</template>
