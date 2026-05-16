<script setup lang="ts">
// 顶部导航栏：Logo + 搜索框 + 暗色模式切换
import { ref } from 'vue'
import { useRouter } from 'vue-router'

defineProps<{ isDark: boolean }>()
const emit = defineEmits<{ 'toggleDark': [] }>()

const router = useRouter()
const searchText = ref('')

function goSearch() {
  if (searchText.value.trim()) {
    router.push({ name: 'search', query: { q: searchText.value.trim() } })
  }
}
</script>

<template>
  <div class="flex items-center justify-between px-6 h-16">
    <!-- Logo 区域 -->
    <div class="flex items-center gap-3 cursor-pointer" @click="router.push('/')">
      <div class="text-2xl font-bold text-blue-600">
        YoMirror<span class="text-gray-700">Site</span>
      </div>
    </div>

    <!-- 搜索框 -->
    <div class="flex-1 max-w-md mx-8">
      <n-input
        v-model:value="searchText"
        placeholder="搜索软件..."
        clearable
        @keyup.enter="goSearch"
      >
        <template #suffix>
          <n-button text @click="goSearch">
            <template #icon>
              <span class="i-ion-search text-lg" />
            </template>
          </n-button>
        </template>
      </n-input>
    </div>

    <!-- 右侧操作 -->
    <div class="flex items-center gap-3">
      <n-button text @click="router.push('/software')">
        全部软件
      </n-button>
      <n-switch :value="isDark" @update:value="emit('toggleDark')">
        <template #checked-icon>
          <span class="i-ion-moon" />
        </template>
        <template #unchecked-icon>
          <span class="i-ion-sunny" />
        </template>
      </n-switch>
    </div>
  </div>
</template>
