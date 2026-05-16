<script setup lang="ts">
// 软件卡片组件：用于列表和首页展示
import type { Software } from '@/types'
import { useRouter } from 'vue-router'

const props = defineProps<{ software: Software }>()
const router = useRouter()

function goDetail() {
  router.push({ name: 'software-detail', params: { id: props.software.id } })
}
</script>

<template>
  <n-card
    hoverable
    class="card-hover cursor-pointer"
    @click="goDetail"
  >
    <div class="flex items-start gap-4">
      <!-- 图标 -->
      <n-avatar
        v-if="software.icon_url"
        :src="software.icon_url"
        :size="48"
        round
      />
      <div v-else class="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center text-xl">
        📦
      </div>

      <!-- 信息 -->
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2 mb-1">
          <span class="font-semibold text-lg truncate">{{ software.name }}</span>
          <n-tag size="small" type="info" v-if="software.latest_ver">
            {{ software.latest_ver }}
          </n-tag>
        </div>
        <p class="text-sm text-gray-500 line-clamp-2 mb-2">{{ software.description }}</p>
        <div class="flex items-center gap-3 text-xs text-gray-400">
          <span>⭐ {{ software.stars.toLocaleString() }}</span>
          <n-tag size="tiny" :bordered="false">{{ software.category }}</n-tag>
          <span v-if="software.license">{{ software.license }}</span>
        </div>
      </div>
    </div>
  </n-card>
</template>
