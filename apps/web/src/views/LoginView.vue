<script setup lang="ts">
import { AxiosError } from 'axios'
import { ChatDotRound, Cpu, FolderOpened, Lock, TrendCharts, User } from '@element-plus/icons-vue'
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '' })
const passwordChanged = computed(() => route.query.password_changed === '1')
const benefits = [
  { icon: FolderOpened, text: '管理评测场景与评测集' },
  { icon: ChatDotRound, text: '记录人工评分与真实交互证据' },
  { icon: TrendCharts, text: '统一跟踪和分析 Badcase' },
  { icon: Cpu, text: '支持智能化、自动化评测流程' },
]

async function submit() {
  loading.value = true
  errorMessage.value = ''
  try {
    await auth.login(form.username, form.password)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (error) {
    const axiosError = error as AxiosError<{ error?: { message?: string } }>
    errorMessage.value = axiosError.response?.data.error?.message || '登录失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login-page login-page--sign-in">
    <section class="login-hero login-hero--sign-in">
      <div class="brand login-brand">
        <span class="brand-mark">Q</span>
        <span>KooEval</span>
      </div>
      <div class="login-benefits" aria-label="KooEval 核心能力">
        <div v-for="item in benefits" :key="item.text" class="login-benefit">
          <el-icon :size="38"><component :is="item.icon" /></el-icon>
          <span>{{ item.text }}</span>
        </div>
      </div>
    </section>
    <section class="login-panel login-panel--sign-in">
      <el-form class="login-form login-card" label-position="top" @submit.prevent="submit">
        <h2>欢迎回来 KooEval</h2>
        <el-alert v-if="passwordChanged" title="密码已设置，请使用新密码登录" type="success" show-icon :closable="false" />
        <el-alert v-if="errorMessage" :title="errorMessage" type="error" show-icon :closable="false" />
        <el-form-item>
          <el-input v-model="form.username" size="large" autocomplete="username" placeholder="用户名 / 邮箱" :prefix-icon="User" aria-label="用户名或邮箱" />
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="form.password"
            size="large"
            type="password"
            show-password
            autocomplete="current-password"
            placeholder="密码"
            :prefix-icon="Lock"
            aria-label="密码"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-button
          type="primary"
          size="large"
          native-type="submit"
          :loading="loading"
          :disabled="!form.username || !form.password"
        >
          登录
        </el-button>
      </el-form>
    </section>
  </main>
</template>
