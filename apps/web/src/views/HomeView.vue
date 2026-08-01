<script setup lang="ts">
import { ArrowRight, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { HomePageData, ResponseEnvelope } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import ActionableEmptyState from '@/components/app/ActionableEmptyState.vue'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const data = ref<HomePageData | null>(null)

const activityLabels: Record<string, string> = {
  created: '创建了 Badcase',
  status_changed: '更新了处理状态',
  assignee_changed: '调整了负责人',
  note_added: '添加了处理备注',
  invalidated: '将 Badcase 标记为无效',
  reactivated: '重新激活了 Badcase',
}

const statusLabels: Record<string, string> = {
  pending: '待处理',
  processing: '处理中',
  resolved: '已解决',
  deferred: '暂不处理',
}

const greeting = computed(() => {
  const hour = new Date().getHours()
  const prefix = hour < 12 ? '上午好' : hour < 18 ? '下午好' : '晚上好'
  return `${prefix}，${auth.user?.display_name || ''}`
})

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<HomePageData>>('/api/v1/pages/home')
    data.value = response.data.data
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function progress(done: number, total: number) {
  return total ? Math.round((done / total) * 100) : 0
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="home-page m6-home">
    <div class="page-heading home-welcome">
      <div>
        <h1>{{ greeting }}</h1>
        <p>继续最近的评测，处理分配给你的问题，并快速进入最新评测集。</p>
      </div>
      <div class="page-actions">
        <el-button :icon="Refresh" @click="load">刷新</el-button>
        <el-button type="primary" @click="router.push('/datasets')">开始新评测</el-button>
      </div>
    </div>

    <div v-if="data" class="home-metric-grid">
      <button
        v-for="metric in data.metrics"
        :key="metric.key"
        class="metric-card metric-card-button"
        type="button"
        @click="router.push(metric.url)"
      >
        <span>{{ metric.label }}</span>
        <strong>{{ metric.value }}</strong>
        <small>查看明细 <el-icon><ArrowRight /></el-icon></small>
      </button>
    </div>

    <div v-if="data" class="home-work-grid">
      <el-card shadow="never">
        <template #header>
          <div class="card-header-actions">
            <strong>继续评测</strong>
            <el-button text type="primary" @click="router.push('/evaluations')">全部评测</el-button>
          </div>
        </template>
        <ActionableEmptyState
          v-if="data.continue_evaluations.length === 0"
          title="没有进行中的评测"
          description="从已发布的评测集开始一次人工评测，进度会自动出现在这里。"
          action-label="选择评测集"
          compact
          @action="router.push('/datasets')"
        />
        <button
          v-for="run in data.continue_evaluations"
          v-else
          :key="run.id"
          class="home-list-row"
          type="button"
          @click="router.push(`/evaluation-runs/${run.id}/workbench`)"
        >
          <div class="home-list-main">
            <strong>{{ run.dataset_name }} V{{ run.version_no }}</strong>
            <span>{{ run.scenario_name }} · {{ run.agent_version }} · {{ run.environment }}</span>
            <el-progress
              :percentage="progress(run.completed_count, run.total_count)"
              :stroke-width="6"
              :show-text="false"
            />
          </div>
          <span>{{ run.completed_count }}/{{ run.total_count }}</span>
        </button>
      </el-card>

      <el-card shadow="never">
        <template #header>
          <div class="card-header-actions">
            <strong>分配给我的 Badcase</strong>
            <el-button text type="primary" @click="router.push('/badcases?assigned_to_me=1&open=1')">查看全部</el-button>
          </div>
        </template>
        <ActionableEmptyState
          v-if="data.assigned_badcases.length === 0"
          title="当前没有待处理问题"
          description="没有分配给你的有效 Badcase；遇到线上问题时也可以主动登记。"
          action-label="主动登记 Badcase"
          compact
          @action="router.push('/badcases/register')"
        />
        <button
          v-for="item in data.assigned_badcases"
          v-else
          :key="item.id"
          class="home-list-row"
          type="button"
          @click="router.push(`/badcases/${item.id}`)"
        >
          <div class="home-list-main">
            <strong>{{ item.title }}</strong>
            <span>{{ item.scenario_name }} · {{ item.source_type === 'business' ? '业务登记' : '评测发现' }}</span>
          </div>
          <el-tag :type="item.status === 'processing' ? 'primary' : 'warning'" effect="light">
            {{ statusLabels[item.status] }}
          </el-tag>
        </button>
      </el-card>

      <el-card shadow="never">
        <template #header>
          <div class="card-header-actions">
            <strong>最近发布的评测集</strong>
            <el-button text type="primary" @click="router.push('/datasets')">浏览评测集</el-button>
          </div>
        </template>
        <ActionableEmptyState
          v-if="data.recent_datasets.length === 0"
          :title="auth.isAdmin ? '还没有已发布的评测集' : '暂无可用的评测集'"
          :description="auth.isAdmin ? '创建评测集并发布首个版本后，团队即可开始人工评测。' : '管理员发布评测集版本后，最新内容会出现在这里。'"
          :action-label="auth.isAdmin ? '前往创建' : ''"
          compact
          @action="router.push('/datasets')"
        />
        <button
          v-for="dataset in data.recent_datasets"
          v-else
          :key="dataset.version_id"
          class="home-list-row"
          type="button"
          @click="router.push(`/datasets/${dataset.dataset_id}`)"
        >
          <div class="home-list-main">
            <strong>{{ dataset.dataset_name }} V{{ dataset.version_no }}</strong>
            <span>{{ dataset.evaluation_target_name }} · {{ dataset.scenario_name }}</span>
          </div>
          <span>{{ dataset.case_count }} 条用例</span>
        </button>
      </el-card>

      <el-card shadow="never">
        <template #header><strong>近期活动</strong></template>
        <ActionableEmptyState
          v-if="data.recent_activities.length === 0"
          title="暂无与你相关的活动"
          description="Badcase 的分配、状态和处理备注更新会汇总在这里。"
          compact
        />
        <button
          v-for="activity in data.recent_activities"
          v-else
          :key="activity.id"
          class="activity-row"
          type="button"
          @click="router.push(`/badcases/${activity.badcase_id}`)"
        >
          <span class="activity-dot"></span>
          <div>
            <strong>{{ activity.actor_name }}{{ activityLabels[activity.activity_type] || '更新了 Badcase' }}</strong>
            <p>{{ activity.badcase_title }}<template v-if="activity.note"> · {{ activity.note }}</template></p>
            <small>{{ formatTime(activity.created_at) }}</small>
          </div>
        </button>
      </el-card>
    </div>
  </section>
</template>
