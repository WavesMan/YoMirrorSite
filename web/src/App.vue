<!--
  @fileoverview 根组件 — 全局布局 + NaiveUI 主题注入

  数据流：
    无需外部 props，通过 n-config-provider 向全局子树注入主题变量
    路由由 <router-view> 接管，各页面通过 Pinia store + API 层获取数据

  主题策略：
    基于 Demo（web/yomirrorsite-demo）设计语言重写 NaiveUI themeOverrides
    覆盖项：字体、主色、圆角、Card/Button/Input/Tag 样式
-->
<script setup lang="ts">
import AppHeader from '@/components/AppHeader.vue'
import { darkTheme, zhCN, dateZhCN } from 'naive-ui'
import { ref, computed } from 'vue'
import type { GlobalThemeOverrides } from 'naive-ui'

const isDark = ref(false)

// --- NaiveUI 主题覆盖（对齐 Demo 设计语言）---
const lightThemeOverrides: GlobalThemeOverrides = {
  common: {
    fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
    fontFamilyMono: 'JetBrains Mono, ui-monospace, SFMono-Regular, monospace',
    primaryColor: '#3b82f6',          // Demo: --color-accent
    primaryColorHover: '#2563eb',
    borderRadius: '12px',
    borderRadiusSmall: '8px',
  },
  Card: {
    borderRadius: '20px',
    borderColor: '#e4e4e7',           // zinc-200
    boxShadow: '0 1px 3px rgba(0,0,0,0.04)',
    titleFontWeight: '700',
  },
  Button: {
    borderRadiusMedium: '12px',
  },
  Input: {
    borderRadius: '9999px',            // 搜索框全圆角
  },
  Tag: {
    borderRadius: '8px',
  },
}

const darkThemeOverrides: GlobalThemeOverrides = {
  common: {
    fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
    fontFamilyMono: 'JetBrains Mono, ui-monospace, SFMono-Regular, monospace',
    primaryColor: '#3b82f6',
    primaryColorHover: '#2563eb',
  },
  Card: {
    borderRadius: '20px',
    titleFontWeight: '700',
  },
  Button: {
    borderRadiusMedium: '12px',
  },
  Input: {
    borderRadius: '9999px',
  },
  Tag: {
    borderRadius: '8px',
  },
}

const theme = computed(() => (isDark.value ? darkTheme : null))
const themeOverrides = computed(() =>
  isDark.value ? darkThemeOverrides : lightThemeOverrides,
)
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="themeOverrides"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-message-provider>
      <!-- 网格纹理背景 — 四角径向渐变（参考 Demo .mesh-bg） -->
      <div class="mesh-bg pointer-events-none">
        <div class="absolute inset-0" style="
          background-image:
            radial-gradient(at 0% 0%, rgba(59,130,246,0.04) 0px, transparent 50%),
            radial-gradient(at 100% 0%, rgba(139,92,246,0.04) 0px, transparent 50%),
            radial-gradient(at 100% 100%, rgba(20,184,166,0.04) 0px, transparent 50%),
            radial-gradient(at 0% 100%, rgba(244,63,94,0.04) 0px, transparent 50%);
        " />
      </div>
      <n-layout class="min-h-screen bg-zinc-50" :native-scrollbar="false">
        <n-layout-header bordered class="glass">
          <AppHeader :is-dark="isDark" @toggle-dark="isDark = !isDark" />
        </n-layout-header>

        <n-layout-content>
          <router-view v-slot="{ Component }">
            <Transition name="page" mode="out-in">
              <component :is="Component" />
            </Transition>
          </router-view>
        </n-layout-content>

        <n-layout-footer bordered class="text-center py-6 text-zinc-300 text-xs border-t border-zinc-100">
          YoMirrorSite — 自由软件镜像站 &copy; {{ new Date().getFullYear() }}
        </n-layout-footer>
      </n-layout>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
.page-enter-active,
.page-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.page-enter-from {
  opacity: 0;
  transform: translateX(10px);
}
.page-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}
</style>
