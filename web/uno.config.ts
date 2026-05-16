/**
 * @fileoverview UnoCSS 配置 — 基于 Demo 设计语言扩展
 *
 * 设计 Token 来源：web/yomirrorsite-demo/src/index.css + App.tsx
 * 映射方式：Tailwind → UnoCSS（类名几乎一致，UnoCSS 解析器兼容 Tailwind 语法）
 *
 * 分层：
 *   theme.fontFamily  → 字体系统（Inter / Space Grotesk / JetBrains Mono）
 *   shortcuts         → 语义化组合类（glass、mesh-bg、card-hover 等）
 */
import { defineConfig, presetUno } from 'unocss'

export default defineConfig({
  presets: [presetUno()],

  // --- 字体系统 ---
  theme: {
    fontFamily: {
      sans: ['Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
      display: ['Space Grotesk', 'sans-serif'],
      mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace'],
    },
  },

  // --- 语义化 Shortcuts（对应 Demo Tailwind 组合类）---
  shortcuts: {
    // 布局容器
    'page-container': 'max-w-1400px mx-auto px-6 py-8',

    // 毛玻璃材质（Demo: .glass）
    'glass':
      'bg-white/70 backdrop-blur-md border border-white/20',

    // 毛玻璃卡片（Demo: bg-white + backdrop-blur 组合）
    'glass-card':
      'bg-white/80 backdrop-blur-md border border-zinc-100 rounded-2rem',

    // 卡片悬停效果（Demo: hover:shadow-2xl hover:shadow-zinc-200/50）
    'card-hover':
      'transition-all duration-300 hover:shadow-2xl hover:shadow-zinc-200/50 hover:-translate-y-1',

    // 纹理网格背景（Demo: .mesh-bg 四角径向渐变）
    'mesh-bg': 'fixed inset-0 -z-1',

    // 展示标题（Demo: font-display font-bold tracking-tight）
    'text-display': 'font-display font-bold tracking-tight',

    // 等宽文本（Demo: font-mono font-medium）
    'text-mono': 'font-mono font-medium',

    // 小号全大写标签（Demo: text-[10px] font-bold uppercase tracking-widest text-zinc-400）
    'text-caption':
      'text-10px font-bold uppercase tracking-widest text-zinc-400',

    // 超圆角（Demo: rounded-[2rem]）
    'rounded-card': 'rounded-2rem',

    // 图标方块容器（Demo: w-12 h-12 rounded-2xl bg-zinc-50）
    'icon-box':
      'w-12 h-12 rounded-2xl bg-zinc-50 flex items-center justify-center border border-zinc-100',
    'icon-box-lg':
      'w-16 h-16 rounded-2xl bg-zinc-50 flex items-center justify-center border border-zinc-100',

    // 标签 pill（Demo: px-2.5 py-1 rounded-lg bg-zinc-50 uppercase tracking-wider text-[10px]）
    'tag-pill':
      'px-2.5 py-1 rounded-lg bg-zinc-50 border border-zinc-100 text-10px font-bold text-zinc-500 uppercase tracking-wider',

    // Stars 徽章（Demo: px-3 py-1.5 rounded-full bg-zinc-50 border border-zinc-100）
    'stars-badge':
      'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-zinc-50 border border-zinc-100 text-10px font-bold tracking-tight',

    // 下载按钮（Demo: bg-zinc-900 text-white rounded-xl hover:bg-black）
    'btn-download':
      'p-3 bg-zinc-900 text-white rounded-xl hover:bg-black transition-colors',

    // 暗色面板（Demo: bg-zinc-950 text-white rounded-[2rem]）
    'dark-panel': 'bg-zinc-950 text-white rounded-2rem',

    // 统计卡片（Demo: bg-white p-6 rounded-3xl border border-zinc-100 shadow-sm）
    'stat-card':
      'bg-white p-6 rounded-3xl border border-zinc-100 shadow-sm',
  },
})
