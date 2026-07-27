import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

import App from './App.vue'
import { setUnauthorizedHandler } from './api/client'
import router from './router'
import { useAuthStore } from './stores/auth'
import './styles/main.css'

const pinia = createPinia()
setUnauthorizedHandler(() => {
  useAuthStore(pinia).clear()
  if (router.currentRoute.value.name !== 'login') {
    void router.replace({
      name: 'login',
      query: { redirect: router.currentRoute.value.fullPath },
    })
  }
})

createApp(App).use(pinia).use(router).use(ElementPlus).mount('#app')
