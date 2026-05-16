// 应用入口

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import NaiveUI from 'naive-ui'
import App from './App.vue'
import router from './router'

// Naive UI 全局样式
import 'uno.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(NaiveUI)

app.mount('#app')
