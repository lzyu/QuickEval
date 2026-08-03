<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const loading = ref(false)
const errorMessage = ref('')
const form = reactive({ current_password: '', new_password: '', confirm_password: '' })

async function submit() {
  if (form.new_password !== form.confirm_password) {
    errorMessage.value = '两次输入的新密码不一致'
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    await apiClient.post('/api/v1/auth/change-password', {
      current_password: form.current_password,
      new_password: form.new_password,
    })
    auth.clear()
    await router.replace({ name: 'login', query: { password_changed: '1' } })
  } catch (error) {
    errorMessage.value = apiErrorMessage(error)
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
        <h1>先保护好你的账号。</h1>
        <p>首次登录需要设置新密码，完成后即可继续使用评测与 Badcase 工作台。</p>
      </div>
      <small>内部评测系统 · 账号安全</small>
    </section>
    <section class="login-panel">
      <el-form class="login-form" label-position="top" @submit.prevent="submit">
        <h2>设置新密码</h2>
        <p>请使用初始密码验证身份，并设置至少 10 位的新密码。</p>
        <el-alert v-if="errorMessage" :title="errorMessage" type="error" show-icon :closable="false" />
        <el-form-item label="初始密码">
          <el-input v-model="form.current_password" size="large" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="form.new_password" size="large" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="确认新密码">
          <el-input v-model="form.confirm_password" size="large" type="password" show-password autocomplete="new-password" @keyup.enter="submit" />
        </el-form-item>
        <el-button type="primary" size="large" native-type="submit" :loading="loading" :disabled="!form.current_password || form.new_password.length < 10 || !form.confirm_password">
          设置并重新登录
        </el-button>
      </el-form>
    </section>
  </main>
</template>
