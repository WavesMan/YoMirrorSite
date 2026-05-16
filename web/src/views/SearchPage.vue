<!--
  @fileoverview 搜索结果页：根据查询参数展示匹配软件

  数据流：
    route.query.q → store.loadSoftwareList({ keyword }) → API
    无 props（页面组件通过路由和 store 获取数据）

  依赖：
    - @/stores/mirror (Pinia store)
    - @/api (fetch 函数)
    - @/types (接口定义)
    - 子组件：SoftwareCard

  路由：/search — search
-->
<script setup lang="ts">
// 搜索结果页
import { useMirrorStore } from '@/stores/mirror'
import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import SoftwareCard from '@/components/SoftwareCard.vue'

const store = useMirrorStore()
const route = useRoute()

function search() {
  const q = route.query.q as string
  if (q) {
    store.loadSoftwareList({ keyword: q, page: 1, page_size: 20 })
  }
}

onMounted(search)
watch(() => route.query.q, search)
</script>

<template>
  <div class="page-container">
    <h1 class="text-2xl font-display font-bold mb-6">
      搜索: "{{ route.query.q }}"
    </h1>

    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <div v-for="i in 3" :key="i" class="animate-pulse bg-zinc-100 rounded-2rem h-48" />
    </div>
    <div v-else-if="store.softwareList.length === 0" class="text-center py-16">
      <p class="text-zinc-400 mb-4">未找到匹配 "{{ route.query.q }}" 的软件</p>
      <n-button @click="$router.push('/software')" class="!rounded-xl">浏览全部软件</n-button>
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
