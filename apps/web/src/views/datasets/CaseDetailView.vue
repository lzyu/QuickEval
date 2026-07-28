<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { ResponseEnvelope, VersionCase } from '@/api/types'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const item = ref<VersionCase | null>(null)

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<VersionCase>>(
      `/api/v1/version-cases/${route.params.caseId}`,
    )
    item.value = response.data.data
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="case-detail-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">CASE SNAPSHOT</p>
        <h1>{{ item?.name || '未命名用例' }}</h1>
        <p>搜索定位到的版本快照，只读展示当时发布的用例内容。</p>
      </div>
      <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
    </div>

    <el-card v-if="item" shadow="never">
      <el-descriptions :column="2" border>
        <el-descriptions-item label="用例 ID">{{ item.id }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="item.is_enabled ? 'success' : 'info'">
            {{ item.is_enabled ? '启用' : '停用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="问题标签" :span="2">
          <el-space wrap>
            <el-tag v-for="tag in item.tags" :key="tag.id" effect="plain">{{ tag.name }}</el-tag>
            <span v-if="item.tags.length === 0" class="muted">无</span>
          </el-space>
        </el-descriptions-item>
      </el-descriptions>

      <div class="case-detail-sections">
        <article>
          <h2>用户问题</h2>
          <p>{{ item.user_prompt }}</p>
        </article>
        <article v-if="item.precondition">
          <h2>前置条件</h2>
          <p>{{ item.precondition }}</p>
        </article>
        <article v-if="item.expected_result">
          <h2>期望结果</h2>
          <p>{{ item.expected_result }}</p>
        </article>
        <article v-if="item.judging_guide">
          <h2>判定提示</h2>
          <p>{{ item.judging_guide }}</p>
        </article>
      </div>
    </el-card>
  </section>
</template>
