<script setup lang="ts">
// 版本时间线组件：展示软件版本发布历史
import type { VersionBrief } from '@/types'
import dayjs from 'dayjs'

defineProps<{ versions: VersionBrief[] }>()
const emit = defineEmits<{ select: [version: VersionBrief] }>()
</script>

<template>
  <n-timeline v-if="versions.length > 0">
    <n-timeline-item
      v-for="ver in versions"
      :key="ver.tag_name"
      :type="ver.prerelease ? 'warning' : 'success'"
      :title="ver.tag_name"
      :time="dayjs(ver.published_at).format('YYYY-MM-DD')"
      class="cursor-pointer hover:bg-gray-50 rounded p-2"
      @click="emit('select', ver)"
    >
      <div class="flex items-center gap-2">
        <span>{{ ver.name || ver.tag_name }}</span>
        <n-tag v-if="ver.prerelease" size="tiny" type="warning">预发布</n-tag>
        <n-tag size="tiny">{{ ver.asset_count }} 个文件</n-tag>
      </div>
    </n-timeline-item>
  </n-timeline>
  <n-empty v-else description="暂无版本记录" />
</template>
