import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// UnoCSS 0.59+ 为 ESM-only，Vite 5 的 esbuild 无法 require ESM-only 包。
// 使用异步工厂函数消除 esbuild 对配置文件的 CJS 打包需求。
// 等价于原来的 import UnoCSS from 'unocss/vite'
export default defineConfig(async () => {
  const UnoCSS = (await import('unocss/vite')).default
  return {
    plugins: [
      vue(),
      UnoCSS(),
    ],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
    server: {
      proxy: {
        // 开发时 API 请求代理到 Go 后端
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
    build: {
      // 构建输出到 web/dist/，Go Fiber 的静态资源目录
      outDir: 'dist',
      emptyOutDir: true,
    },
  }
})
