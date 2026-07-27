<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { fetchReadiness } from '@/features/system/health'

type ReadinessState = 'loading' | 'ready' | 'unavailable'

const state = ref<ReadinessState>('loading')
const requestId = ref('')

const statusText = computed(() => {
  if (state.value === 'ready') return '后端、MySQL 与 Redis 已就绪'
  if (state.value === 'unavailable') return '服务依赖尚未就绪'
  return '正在检查服务状态'
})

async function checkReadiness() {
  state.value = 'loading'
  try {
    const result = await fetchReadiness()
    requestId.value = result.meta.request_id
    state.value = 'ready'
  } catch {
    requestId.value = ''
    state.value = 'unavailable'
  }
}

onMounted(checkReadiness)
</script>

<template>
  <section class="home-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">QuickEval V1</p>
        <h1>Agent 人工评测平台</h1>
        <p>登录、用户、基础目录与问题标签已经就绪，可以开始准备评测数据。</p>
      </div>
      <el-button :loading="state === 'loading'" @click="checkReadiness">重新检查</el-button>
    </div>

    <el-card class="health-card" shadow="never">
      <div class="health-content">
        <span class="health-indicator" :class="state" aria-hidden="true"></span>
        <div>
          <h2>服务健康状态</h2>
          <p>{{ statusText }}</p>
          <small v-if="requestId">Request ID：{{ requestId }}</small>
        </div>
      </div>
    </el-card>

    <div class="foundation-grid">
      <el-card shadow="never">
        <strong>Go API</strong>
        <p>Cookie Session、CSRF、固定角色权限与审计</p>
      </el-card>
      <el-card shadow="never">
        <strong>MySQL</strong>
        <p>用户、评测目录与 17 张 V1 业务表</p>
      </el-card>
      <el-card shadow="never">
        <strong>Redis</strong>
        <p>会话、登录限流与用户会话统一失效</p>
      </el-card>
    </div>
  </section>
</template>
