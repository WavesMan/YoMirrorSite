import { defineConfig, presetUno } from 'unocss'

export default defineConfig({
  presets: [presetUno()],
  shortcuts: {
    'page-container': 'max-w-1400px mx-auto px-4 py-6',
    'card-hover': 'transition-all duration-200 hover:shadow-lg hover:-translate-y-1',
  },
})
