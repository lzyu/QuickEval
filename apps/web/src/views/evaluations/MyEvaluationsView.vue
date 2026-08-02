<script setup lang="ts">
import { ArrowDown, CircleCheck, Clock, CloseBold, Plus, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  EvaluationEnvironment,
  EvaluationRun,
  PageData,
  ResponseEnvelope,
  RunStatus,
} from '@/api/types'
import ActionableEmptyState from '@/components/app/ActionableEmptyState.vue'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const runs = ref<EvaluationRun[]>([])
const filters = reactive({
  keyword: String(route.query.keyword || ''),
  status: String(route.query.status || ''),
  environment: String(route.query.environment || ''),
})
const hasFilters = computed(() => Boolean(filters.keyword || filters.status || filters.environment))

const counts = computed(() => ({
  in_progress: runs.value.filter((item) => item.status === 'in_progress').length,
  completed: runs.value.filter((item) => item.status === 'completed').length,
  voided: runs.value.filter((item) => item.status === 'voided').length,
}))

const environmentLabels: Record<EvaluationEnvironment, string> = {
  test: '测试',
  staging: '预发布',
  production: '生产',
  other: '其他',
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<PageData<EvaluationRun>>>(
      '/api/v1/evaluation-runs',
      { params: { page_size: 100, ...filters } },
    )
    runs.value = response.data.data.items
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function openRun(run: EvaluationRun) {
  router.push(
    run.status === 'in_progress'
      ? `/evaluation-runs/${run.id}/workbench`
      : `/evaluation-runs/${run.id}/result`,
  )
}

async function reopen(run: EvaluationRun) {
  try {
    const { value } = await ElMessageBox.prompt('请填写重开原因', '重开评测', {
      inputType: 'textarea',
      inputValidator: (text) => Boolean(text.trim()) || '重开原因不能为空',
    })
    await apiClient.post(`/api/v1/evaluation-runs/${run.id}/reopen`, {
      reason: value,
      expected_lock_version: run.lock_version,
    })
    ElMessage.success('评测已重开')
    await router.push(`/evaluation-runs/${run.id}/workbench`)
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function voidRun(run: EvaluationRun) {
  try {
    const { value } = await ElMessageBox.prompt('作废后不能恢复，请填写原因', '作废评测', {
      type: 'warning',
      inputType: 'textarea',
      inputValidator: (text) => Boolean(text.trim()) || '作废原因不能为空',
    })
    await apiClient.post(`/api/v1/evaluation-runs/${run.id}/void`, {
      reason: value,
      expected_lock_version: run.lock_version,
    })
    ElMessage.success('评测已作废')
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function deleteRun(run: EvaluationRun) {
  try {
    await ElMessageBox.confirm('删除后评测记录和已保存结果都无法恢复。', '删除评测', {
      type: 'warning',
      confirmButtonText: '确认删除',
    })
    await apiClient.delete(`/api/v1/evaluation-runs/${run.id}`, {
      params: { expected_lock_version: run.lock_version },
    })
    ElMessage.success('评测已删除')
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

function statusLabel(status: RunStatus) {
  return { in_progress: '进行中', completed: '已完成', voided: '已作废' }[status]
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function resetFilters() {
  Object.assign(filters, { keyword: '', status: '', environment: '' })
  load()
}

onMounted(load)
</script>

<template>
  <section class="evaluation-list-page">
    <div class="run-summary-grid">
      <article>
        <el-icon><Clock /></el-icon>
        <div><span>进行中</span><strong>{{ counts.in_progress }}</strong></div>
      </article>
      <article>
        <el-icon><CircleCheck /></el-icon>
        <div><span>已完成</span><strong>{{ counts.completed }}</strong></div>
      </article>
      <article>
        <el-icon><CloseBold /></el-icon>
        <div><span>已作废</span><strong>{{ counts.voided }}</strong></div>
      </article>
    </div>

    <el-card class="evaluation-list-card" shadow="never">
      <div class="evaluation-list-toolbar">
        <el-button type="primary" :icon="Plus" @click="router.push('/datasets')">开始新评测</el-button>
      </div>
      <div class="run-filter-bar">
        <el-input
          v-model="filters.keyword"
          :prefix-icon="Search"
          placeholder="搜索评测集或 Agent 版本"
          aria-label="搜索评测记录"
          clearable
          @keyup.enter="load"
        />
        <el-select v-model="filters.status" clearable placeholder="全部状态" aria-label="按评测状态筛选" @change="load">
          <el-option label="进行中" value="in_progress" />
          <el-option label="已完成" value="completed" />
          <el-option label="已作废" value="voided" />
        </el-select>
        <el-select v-model="filters.environment" clearable placeholder="全部环境" aria-label="按运行环境筛选" @change="load">
          <el-option label="测试" value="test" />
          <el-option label="预发布" value="staging" />
          <el-option label="生产" value="production" />
          <el-option label="其他" value="other" />
        </el-select>
        <div
          class="filter-actions evaluation-filter-actions"
          :class="{ 'has-reset': hasFilters }"
        >
          <el-button type="primary" @click="load">查询</el-button>
          <el-button v-if="hasFilters" @click="resetFilters">重置</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="runs" row-key="id">
        <template #empty>
          <ActionableEmptyState
            :title="hasFilters ? '没有符合条件的评测记录' : '还没有人工评测记录'"
            :description="hasFilters ? '调整关键词、状态或环境后再试。' : '从已发布的评测集版本发起评测，完成结果会持续保留在这里。'"
            :action-label="hasFilters ? '清除筛选' : '选择评测集'"
            compact
            @action="hasFilters ? resetFilters() : router.push('/datasets')"
          />
        </template>
        <el-table-column label="评测集" min-width="220">
          <template #default="{ row }">
            <button class="table-primary-link" @click="openRun(row)">
              {{ row.dataset_name }}
            </button>
            <div class="table-secondary">{{ row.evaluation_target_name }}</div>
          </template>
        </el-table-column>
        <el-table-column label="版本" width="80">
          <template #default="{ row }">V{{ row.version_no }}</template>
        </el-table-column>
        <el-table-column prop="agent_version" label="Agent 版本" min-width="130" />
        <el-table-column label="环境" width="100">
          <template #default="{ row }">{{ environmentLabels[row.environment as EvaluationEnvironment] }}</template>
        </el-table-column>
        <el-table-column label="评测进度" min-width="190">
          <template #default="{ row }">
            <div class="run-progress-cell">
              <div>
                <span>
                  {{ row.progress.evaluated_count + row.progress.skipped_count }}/{{ row.progress.total_count }}
                </span>
                <small>{{ Math.round(row.progress.completion_rate * 100) }}%</small>
              </div>
              <el-progress
                :percentage="Math.round(row.progress.completion_rate * 100)"
                :show-text="false"
                :stroke-width="6"
              />
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag
              :type="row.status === 'completed' ? 'success' : row.status === 'voided' ? 'danger' : 'primary'"
            >
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220" align="right">
          <template #default="{ row }">
            <div class="evaluation-row-actions">
              <el-button
                :type="row.status === 'in_progress' ? 'primary' : 'default'"
                :plain="row.status !== 'in_progress'"
                @click="openRun(row)"
              >
                {{ row.status === 'in_progress' ? '继续评测' : '查看结果' }}
              </el-button>
              <el-dropdown v-if="row.status !== 'voided'" trigger="click">
                <el-button class="evaluation-more-action" aria-label="更多操作">
                  更多<el-icon><ArrowDown /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="row.status === 'completed'" @click="reopen(row)">
                      重开评测
                    </el-dropdown-item>
                    <el-dropdown-item
                      v-if="row.status === 'in_progress' && !row.first_completed_at"
                      @click="deleteRun(row)"
                    >
                      删除
                    </el-dropdown-item>
                    <el-dropdown-item class="danger-menu-item" @click="voidRun(row)">
                      作废
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>
</template>
