<script setup lang="ts">
import { ArrowDown, Plus, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { Badcase, CatalogItem, PageData, ResponseEnvelope, Scenario } from '@/api/types'
import EvaluationTargetDialog from '@/components/badcases/EvaluationTargetDialog.vue'
import ActionableEmptyState from '@/components/app/ActionableEmptyState.vue'
import { badcaseDisplayTitle } from '@/features/badcases/display'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const loading = ref(false)
const targetPickerOpen = ref(false)
const items = ref<Badcase[]>([])
const total = ref(0)
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const assignees = ref<Array<{ id: string; display_name: string }>>([])
const query = reactive({
  page: 1,
  page_size: 20,
  keyword: String(route.query.keyword || ''),
  source_type: String(route.query.source_type || ''),
  status: String(route.query.status || ''),
  validity: '',
  evaluation_target_id: String(route.query.evaluation_target_id || ''),
  scenario_id: String(route.query.scenario_id || ''),
  dataset_id: String(route.query.dataset_id || ''),
  dataset_version_id: String(route.query.dataset_version_id || ''),
  evaluator_id: String(route.query.evaluator_id || ''),
  assignee_id: String(route.query.assignee_id || ''),
  issue_tag_id: String(route.query.issue_tag_id || ''),
  open: route.query.open === '1' ? '1' : '',
  agent_version: String(route.query.agent_version || ''),
  environment: String(route.query.environment || ''),
  occurred_from: String(route.query.occurred_from || ''),
  occurred_to: String(route.query.occurred_to || ''),
})
const advancedFiltersOpen = ref(Boolean(query.scenario_id || query.assignee_id || query.validity))
const visibleFilterCount = computed(
  () =>
    [
      query.keyword,
      query.evaluation_target_id,
      query.source_type,
      query.scenario_id,
      query.assignee_id,
      query.status,
      query.validity,
    ]
      .filter(Boolean).length,
)
const visibleScenarios = computed(() =>
  query.evaluation_target_id
    ? scenarios.value.filter((item) => item.evaluation_target_id === query.evaluation_target_id)
    : [],
)
async function loadOptions() {
  try {
    const [targetResponse, scenarioResponse, optionResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>('/api/v1/evaluation-targets', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<
        ResponseEnvelope<{
          assignees: Array<{ id: string; display_name: string }>
          issue_tags: unknown[]
        }>
      >('/api/v1/badcase-options'),
    ])
    targets.value = targetResponse.data.data.items
    scenarios.value = scenarioResponse.data.data.items
    assignees.value = optionResponse.data.data.assignees
    if (query.scenario_id && !query.evaluation_target_id) {
      query.evaluation_target_id =
        scenarios.value.find((item) => item.id === query.scenario_id)?.evaluation_target_id || ''
    }
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<PageData<Badcase>>>(
      '/api/v1/badcases',
      {
        params: {
          ...query,
          assignee_id:
            query.assignee_id || (route.query.assigned_to_me === '1' ? auth.user?.id : ''),
        },
      },
    )
    items.value = response.data.data.items
    total.value = response.data.data.total
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function search() {
  query.page = 1
  load()
}

function changeTarget() {
  query.scenario_id = ''
  search()
}

function resetFilters() {
  Object.assign(query, {
    page: 1,
    keyword: '',
    source_type: '',
    status: '',
    validity: '',
    evaluation_target_id: '',
    scenario_id: '',
    dataset_id: '',
    dataset_version_id: '',
    evaluator_id: '',
    assignee_id: '',
    issue_tag_id: '',
    open: '',
    agent_version: '',
    environment: '',
    occurred_from: '',
    occurred_to: '',
  })
  load()
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function statusLabel(status: Badcase['status']) {
  return {
    pending: '待处理',
    processing: '处理中',
    resolved: '已解决',
    deferred: '暂不处理',
  }[status]
}

function statusType(row: Badcase) {
  if (row.invalidated_at) return 'info'
  if (row.status === 'resolved') return 'success'
  return undefined
}

function openRegistration(targetId: string) {
  router.push({ name: 'badcase-register', query: { evaluation_target_id: targetId } })
}

onMounted(async () => {
  await loadOptions()
  await load()
})
</script>

<template>
  <section class="badcase-list-page">
    <el-card shadow="never">
      <div class="content-primary-actions">
        <el-button type="primary" :icon="Plus" @click="targetPickerOpen = true">
          主动登记 Badcase
        </el-button>
      </div>
      <div class="badcase-filter-bar">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="搜索原始输入、问题描述或 Agent 回答"
          :prefix-icon="Search"
          aria-label="搜索 Badcase"
          @keyup.enter="search"
        />
        <el-select
          v-model="query.evaluation_target_id"
          filterable
          clearable
          placeholder="全部评测对象"
          aria-label="按评测对象筛选"
          @change="changeTarget"
        >
          <el-option v-for="target in targets" :key="target.id" :label="target.name" :value="target.id" />
        </el-select>
        <el-select v-model="query.source_type" clearable placeholder="全部来源" aria-label="按来源筛选" @change="search">
          <el-option label="评测发现" value="evaluation" />
          <el-option label="业务登记" value="business" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="全部状态" aria-label="按状态筛选" @change="search">
          <el-option label="待处理" value="pending" />
          <el-option label="处理中" value="processing" />
          <el-option label="已解决" value="resolved" />
          <el-option label="暂不处理" value="deferred" />
        </el-select>
        <div class="filter-actions badcase-filter-actions">
          <el-button type="primary" @click="search">查询</el-button>
          <el-button
            class="advanced-filter-toggle"
            :class="{ open: advancedFiltersOpen }"
            :aria-expanded="advancedFiltersOpen"
            aria-controls="badcase-advanced-filters"
            @click="advancedFiltersOpen = !advancedFiltersOpen"
          >
            更多筛选
            <el-icon><ArrowDown /></el-icon>
          </el-button>
        </div>
      </div>

      <div v-show="advancedFiltersOpen" id="badcase-advanced-filters" class="badcase-advanced-filters">
        <el-select
          v-model="query.scenario_id"
          clearable
          :disabled="!query.evaluation_target_id"
          :placeholder="query.evaluation_target_id ? '全部评测场景' : '请先选择评测对象'"
          aria-label="按评测场景筛选"
          @change="search"
        >
          <el-option
            v-for="scenario in visibleScenarios"
            :key="scenario.id"
            :label="scenario.name"
            :value="scenario.id"
          />
        </el-select>
        <el-select v-model="query.assignee_id" clearable placeholder="全部负责人" aria-label="按负责人筛选" @change="search">
          <el-option
            v-for="user in assignees"
            :key="user.id"
            :label="user.display_name"
            :value="user.id"
          />
        </el-select>
        <el-select v-model="query.validity" clearable placeholder="有效记录" aria-label="按有效性筛选" @change="search">
          <el-option label="有效记录" value="" />
          <el-option label="无效记录" value="invalid" />
          <el-option label="全部记录" value="all" />
        </el-select>
      </div>

      <div v-if="visibleFilterCount" class="active-filter-summary">
        <span>已应用 {{ visibleFilterCount }} 项筛选</span>
        <el-button link type="primary" @click="resetFilters">清除全部</el-button>
      </div>

      <el-table v-loading="loading" :data="items" row-key="id">
        <template #empty>
          <ActionableEmptyState
            :title="visibleFilterCount ? '没有符合条件的 Badcase' : '还没有 Badcase'"
            :description="visibleFilterCount ? '调整或清除筛选条件后再试。' : '主动登记真实业务问题，或在人工评测中将问题沉淀为 Badcase。'"
            :action-label="visibleFilterCount ? '清除筛选' : '登记第一条 Badcase'"
            compact
            @action="visibleFilterCount ? resetFilters() : (targetPickerOpen = true)"
          />
        </template>
        <el-table-column label="Badcase" min-width="300">
          <template #default="{ row }">
            <button class="table-primary-link" @click="router.push(`/badcases/${row.id}`)">
              {{ badcaseDisplayTitle(row) }}
            </button>
            <small v-if="row.source_type === 'business' && row.description">问题描述：{{ row.title }}</small>
            <small v-else>{{ row.description || row.evaluation?.user_prompt || '-' }}</small>
          </template>
        </el-table-column>
        <el-table-column label="来源" width="100">
          <template #default="{ row }">
            <el-tag :type="row.source_type === 'business' ? 'warning' : 'primary'" effect="plain">
              {{ row.source_type === 'business' ? '业务登记' : '评测发现' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="评测对象" min-width="170">
          <template #default="{ row }">{{ row.evaluation_target_name }}</template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row)">
              {{ row.invalidated_at ? '无效' : statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="问题标签" min-width="180">
          <template #default="{ row }">
            <el-tag
              v-for="tag in row.issue_tags"
              :key="tag.id"
              type="danger"
              effect="plain"
              size="small"
            >
              {{ tag.name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="负责人" width="120">
          <template #default="{ row }">{{ row.assignee_name || '未分配' }}</template>
        </el-table-column>
        <el-table-column label="发现时间" width="180">
          <template #default="{ row }">{{ formatTime(row.occurred_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        class="table-pagination"
        layout="total, prev, pager, next"
        :total="total"
        @current-change="load"
      />
    </el-card>
  </section>

  <EvaluationTargetDialog v-model="targetPickerOpen" @select="openRegistration" />
</template>
