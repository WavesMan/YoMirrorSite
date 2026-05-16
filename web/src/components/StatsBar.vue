<script setup lang="ts">
// 镜像站统计条
import { useMirrorStore } from '@/stores/mirror'
import { onMounted } from 'vue'

const store = useMirrorStore()
onMounted(() => { store.loadStats() })
</script>

<template>
  <n-grid :cols="4" :x-gap="12" v-if="store.stats">
    <n-gi>
      <n-statistic label="已镜像软件" :value="store.stats.total_software" />
    </n-gi>
    <n-gi>
      <n-statistic label="版本总数" :value="store.stats.total_versions || 0" />
    </n-gi>
    <n-gi>
      <n-statistic label="累计下载" :value="store.stats.total_downloads || 0" />
    </n-gi>
    <n-gi>
      <n-statistic label="总存储占用">
        {{ ((store.stats.total_size || 0) / 1024 / 1024 / 1024).toFixed(1) }} GB
      </n-statistic>
    </n-gi>
  </n-grid>
  <n-skeleton v-else :repeat="1" text />
</template>
