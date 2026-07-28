<script setup lang="ts">
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { Badcase, ResponseEnvelope } from '@/api/types'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const item = ref<Badcase | null>(null)

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<Badcase>>(
      `/api/v1/pages/badcases/${String(route.params.badcaseId)}`,
    )
    item.value = response.data.data
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="badcase-detail-page">
    <template v-if="item">
      <div class="detail-back">
        <el-button :icon="ArrowLeft" text @click="router.push('/badcases')">返回 Badcase 中心</el-button>
      </div>
      <div class="badcase-detail-heading">
        <div>
          <p class="eyebrow">评测 Badcase</p>
          <h1>{{ item.title }}</h1>
          <p>{{ item.evaluation_target_name }} · {{ item.scenario_name }} · {{ formatTime(item.occurred_at) }}</p>
        </div>
        <el-tag type="danger" effect="light">{{ item.status === 'pending' ? '待处理' : item.status }}</el-tag>
      </div>

      <div class="badcase-detail-grid">
        <main>
          <el-card shadow="never">
            <template #header><strong>问题描述</strong></template>
            <p class="prewrap">{{ item.description || '-' }}</p>
            <div>
              <el-tag
                v-for="tag in item.issue_tags"
                :key="tag.id"
                type="danger"
                effect="plain"
              >
                {{ tag.name }}
              </el-tag>
            </div>
          </el-card>

          <el-card v-if="item.evaluation" shadow="never">
            <template #header><strong>原始评测上下文</strong></template>
            <h3>用户问题或任务指令</h3>
            <p class="prewrap">{{ item.evaluation.user_prompt || '-' }}</p>
            <h3>Agent 回答</h3>
            <p class="prewrap">{{ item.agent_response_text || '未粘贴文本，请查看截图证据。' }}</p>
            <h3>评测意见</h3>
            <p class="prewrap">{{ item.evaluation.comment || '-' }}</p>
            <div v-if="item.attachments.length" class="badcase-evidence-grid">
              <el-image
                v-for="(attachment, index) in item.attachments"
                :key="attachment.id"
                :src="attachment.content_url"
                :preview-src-list="item.attachments.map((entry) => entry.content_url)"
                :initial-index="index"
                fit="cover"
                preview-teleported
              />
            </div>
          </el-card>

          <el-card shadow="never">
            <template #header><strong>活动记录</strong></template>
            <el-timeline>
              <el-timeline-item
                v-for="activity in item.activities"
                :key="activity.id"
                :timestamp="formatTime(activity.created_at)"
              >
                {{ activity.activity_type === 'created' ? '创建 Badcase' : '重新激活 Badcase' }}
                · {{ activity.actor_name }}
                <p v-if="activity.note">{{ activity.note }}</p>
              </el-timeline-item>
            </el-timeline>
          </el-card>
        </main>

        <aside>
          <el-card shadow="never">
            <template #header><strong>追溯信息</strong></template>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="评测集">
                {{ item.evaluation?.dataset_name || '-' }} V{{ item.evaluation?.version_no || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="用例">{{ item.evaluation?.case_name || '-' }}</el-descriptions-item>
              <el-descriptions-item label="评测人">{{ item.evaluation?.evaluator_name || '-' }}</el-descriptions-item>
              <el-descriptions-item label="评分">{{ item.evaluation?.score || '未评分' }}</el-descriptions-item>
              <el-descriptions-item label="Agent 版本">{{ item.agent_version || '-' }}</el-descriptions-item>
              <el-descriptions-item label="运行环境">{{ item.environment }}</el-descriptions-item>
            </el-descriptions>
            <el-button
              v-if="item.evaluation"
              class="trace-link"
              type="primary"
              plain
              @click="router.push(`/evaluation-runs/${item.evaluation?.evaluation_run_id}/result`)"
            >
              查看完整评测结果
            </el-button>
          </el-card>
        </aside>
      </div>
    </template>
  </section>
</template>
