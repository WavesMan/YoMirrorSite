// Vue Router 配置

import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/views/HomePage.vue'),
    },
    {
      path: '/software',
      name: 'software-list',
      component: () => import('@/views/SoftwareList.vue'),
    },
    {
      path: '/software/:id',
      name: 'software-detail',
      component: () => import('@/views/SoftwareDetail.vue'),
    },
    {
      path: '/search',
      name: 'search',
      component: () => import('@/views/SearchPage.vue'),
    },
  ],
})

export default router
