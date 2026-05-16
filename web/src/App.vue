<script setup lang="ts">
// 根组件：全局布局（导航栏 + 内容区）
import AppHeader from '@/components/AppHeader.vue'
import { darkTheme, zhCN, dateZhCN } from 'naive-ui'
import { ref } from 'vue'

const isDark = ref(false)
</script>

<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-layout class="min-h-screen">
        <n-layout-header bordered>
          <AppHeader :is-dark="isDark" @toggle-dark="isDark = !isDark" />
        </n-layout-header>
        <n-layout-content>
          <router-view />
        </n-layout-content>
        <n-layout-footer bordered class="text-center py-4 text-gray-500">
          YoMirrorSite — 自由软件镜像站 &copy; {{ new Date().getFullYear() }}
        </n-layout-footer>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>
