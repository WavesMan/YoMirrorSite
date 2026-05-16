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
    <h1 class="text-2xl font-bold mb-6">
      搜索: "{{ route.query.q }}"
    </h1>

    <div v-if="store.loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <n-skeleton v-for="i in 6" :key="i" height="120" />
    </div>
    <div v-else-if="store.softwareList.length === 0" class="text-center py-12">
      <n-empty description="未找到匹配的软件">
        <template #extra>
          <n-button @click="$router.push('/software')">浏览全部软件</n-button>
        </template>
      </n-empty>
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
