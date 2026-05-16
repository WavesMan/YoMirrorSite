<!--
  @fileoverview 顶部导航栏 — Logo、搜索框、暗色模式切换与路由导航

  数据流：
    接收 props: { isDark: boolean }
    触发 emit:  { toggleDark: [] }
    依赖：vue-router (useRouter)、Naive UI (n-input, n-switch)

  状态：searchText ref（搜索框输入绑定）
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

defineProps<{ isDark: boolean }>()
const emit = defineEmits<{ 'toggleDark': [] }>()

const router = useRouter()
const searchText = ref('')

// 回车或点击搜索图标时跳转搜索结果页
function goSearch() {
  if (searchText.value.trim()) {
    router.push({ name: 'search', query: { q: searchText.value.trim() } })
  }
}
</script>

<template>
  <div class="flex items-center justify-between px-8 h-18">
    <!-- Logo 区域 -->
    <div class="flex items-center gap-3 cursor-pointer" @click="router.push('/')">
      <div class="w-10 h-10 bg-black rounded-xl flex items-center justify-center text-white">
        <span class="text-lg">⚡</span>
      </div>
      <h1 class="font-display text-xl font-bold tracking-tight">YoMirror</h1>
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
      <button
        @click="router.push('/software')"
        :class="$route.name === 'software-list'
          ? 'bg-zinc-900 text-white shadow-lg'
          : 'text-zinc-500 hover:bg-zinc-100'"
        class="flex items-center gap-3 px-4 py-3 rounded-xl transition-all"
      >
        全部软件
      </button>
      <a
        href="https://github.com/WavesMan/YoMirrorSite"
        target="_blank"
        rel="noopener noreferrer"
        class="flex items-center gap-3 px-4 py-3 rounded-xl transition-all text-zinc-500 hover:bg-zinc-100"
      >
        GitHub
      </a>
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
