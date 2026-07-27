<script setup lang="ts">
import { AxiosError } from 'axios'
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ username: '', password: '' })

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
  <main class="login-page">
    <section class="login-hero">
      <div class="brand brand-on-dark">
        <span class="brand-mark">Q</span>
        <span>QuickEval</span>
      </div>
      <div>
        <p class="eyebrow light">QUICK, FOCUSED, TRACEABLE</p>
        <h1>让每一次 Agent 评测<br />都更快、更清晰。</h1>
        <p>面向云市场智慧助手与智能采购 Agent 的轻量黑盒评测平台。</p>
      </div>
      <small>内部评测系统 · V1</small>
    </section>
    <section class="login-panel">
      <el-form class="login-form" label-position="top" @submit.prevent="submit">
        <h2>欢迎回来</h2>
        <p>使用本地账号登录 QuickEval</p>
        <el-alert v-if="errorMessage" :title="errorMessage" type="error" show-icon :closable="false" />
        <el-form-item label="用户名或邮箱">
          <el-input v-model="form.username" size="large" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            size="large"
            type="password"
            show-password
            autocomplete="current-password"
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
