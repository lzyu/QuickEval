<script setup lang="ts">
import { CircleCheck, Clock, CloseBold, Plus, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  EvaluationEnvironment,
  EvaluationRun,
  PageData,
  ResponseEnvelope,
  RunStatus,
} from '@/api/types'

const router = useRouter()
const loading = ref(false)
const runs = ref<EvaluationRun[]>([])
const filters = reactive({ keyword: '', status: '', environment: '' })

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

onMounted(load)
</script>

<template>
  <section class="evaluation-list-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">MY EVALUATIONS</p>
        <h1>我的评测</h1>
        <p>查看和继续你发起的独立人工评测。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="router.push('/datasets')">开始新评测</el-button>
    </div>

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

    <el-card shadow="never">
      <div class="run-filter-bar">
        <el-input
          v-model="filters.keyword"
          :prefix-icon="Search"
          placeholder="搜索评测集或 Agent 版本"
          clearable
          @keyup.enter="load"
        />
        <el-select v-model="filters.status" clearable placeholder="全部状态" @change="load">
          <el-option label="进行中" value="in_progress" />
          <el-option label="已完成" value="completed" />
          <el-option label="已作废" value="voided" />
        </el-select>
        <el-select v-model="filters.environment" clearable placeholder="全部环境" @change="load">
          <el-option label="测试" value="test" />
          <el-option label="预发布" value="staging" />
          <el-option label="生产" value="production" />
          <el-option label="其他" value="other" />
        </el-select>
        <el-button @click="load">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="runs" empty-text="暂无评测记录">
        <el-table-column label="评测集" min-width="220">
          <template #default="{ row }">
            <a class="dataset-name" @click="openRun(row)">{{ row.dataset_name }}</a>
            <div class="table-secondary">{{ row.evaluation_target_name }} / {{ row.scenario_name }}</div>
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
            <span>{{ row.progress.evaluated_count + row.progress.skipped_count }}/{{ row.progress.total_count }}</span>
            <el-progress
              :percentage="Math.round(row.progress.completion_rate * 100)"
              :show-text="false"
              :stroke-width="7"
            />
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
          <template #default="{ row }">{{ new Date(row.updated_at).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column label="操作" width="190" align="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'in_progress'"
              type="primary"
              @click="openRun(row)"
            >
              继续评测
            </el-button>
            <el-button v-else link type="primary" @click="openRun(row)">查看结果</el-button>
            <el-dropdown>
              <el-button link>更多</el-button>
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
                  <el-dropdown-item
                    v-if="row.status !== 'voided'"
                    class="danger-menu-item"
                    @click="voidRun(row)"
                  >
                    作废
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>
</template>
