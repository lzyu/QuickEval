<script setup lang="ts">
import { ArrowLeft, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  EvaluationResultDetail,
  PageData,
  ResponseEnvelope,
} from '@/api/types'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const items = ref<EvaluationResultDetail[]>([])
const total = ref(0)
const completedRunCount = ref(0)
const query = reactive({
  page: Number(route.query.page || 1),
  page_size: 20,
  keyword: String(route.query.keyword || ''),
  result_status: String(route.query.result_status || ''),
  score: String(route.query.score || ''),
  skip_reason: String(route.query.skip_reason || ''),
  has_badcase: String(route.query.has_badcase || ''),
  scored: String(route.query.scored || ''),
})

const contextParams = computed(() => {
  const detailKeys = new Set([
    'page', 'page_size', 'keyword', 'result_status', 'score', 'skip_reason',
    'has_badcase', 'scored',
  ])
  return Object.fromEntries(
    Object.entries(route.query)
      .filter(([key, value]) => !detailKeys.has(key) && typeof value === 'string' && value)
      .map(([key, value]) => [key, String(value)]),
  )
})

function params() {
  return {
    ...contextParams.value,
    ...Object.fromEntries(
      Object.entries(query).filter(([, value]) => value !== ''),
    ),
  }
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<
      ResponseEnvelope<PageData<EvaluationResultDetail> & { completed_run_count: number }>
    >(
      '/api/v1/evaluation-results',
      { params: params() },
    )
    items.value = response.data.data.items
    total.value = response.data.data.total
    completedRunCount.value = response.data.data.completed_run_count
    await router.replace({ query: params() })
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

function clearSegment() {
  Object.assign(query, {
    page: 1,
    keyword: '',
    result_status: '',
    score: '',
    skip_reason: '',
    has_badcase: '',
    scored: '',
  })
  load()
}

function formatTime(value: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '-'
}

function openRow(row: EvaluationResultDetail) {
  router.push(row.result_detail_url)
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="evaluation-results-page">
    <div class="page-heading">
      <div>
        <h1>评测结果明细</h1>
        <p>仅展示已完成评测，保留数据看板的筛选条件和所点击的图表分段。</p>
      </div>
      <div class="page-actions">
        <el-button :icon="ArrowLeft" @click="router.push({ path: '/dashboard', query: contextParams })">
          返回看板
        </el-button>
        <el-button @click="clearSegment">清除明细条件</el-button>
      </div>
    </div>

    <el-card shadow="never">
      <div class="table-toolbar evaluation-result-toolbar">
        <el-input
          v-model="query.keyword"
          clearable
          :prefix-icon="Search"
          placeholder="搜索用例、问题、回答、评语或评测人"
          @keyup.enter="search"
          @clear="search"
        />
        <el-select v-model="query.result_status" clearable placeholder="结果状态" @change="search">
          <el-option label="已评" value="evaluated" />
          <el-option label="已跳过" value="skipped" />
        </el-select>
        <el-select v-model="query.score" clearable placeholder="评分" @change="search">
          <el-option v-for="score in 5" :key="score" :label="`${score} 分`" :value="String(score)" />
        </el-select>
        <el-select v-model="query.has_badcase" clearable placeholder="Badcase" @change="search">
          <el-option label="已标记 Badcase" value="1" />
          <el-option label="未标记 Badcase" value="0" />
        </el-select>
        <el-button type="primary" :icon="Search" @click="search">查询</el-button>
      </div>

      <div v-if="Object.keys(contextParams).length" class="detail-filter-context">
        <span>看板筛选：</span>
        <el-tag v-for="(value, key) in contextParams" :key="key" effect="plain">
          {{ key }} = {{ value }}
        </el-tag>
      </div>
      <p class="evaluation-result-summary">
        当前条件命中 <strong>{{ completedRunCount }}</strong> 次已完成评测、
        <strong>{{ total }}</strong> 条结果明细。
      </p>

      <el-table :data="items" row-key="id" @row-dblclick="openRow">
        <el-table-column label="用例 / 问题" min-width="280">
          <template #default="{ row }">
            <button class="table-primary-link" type="button" @click="router.push(row.result_detail_url)">
              {{ row.case_name || '未命名用例' }}
            </button>
            <p class="table-secondary-text">{{ row.user_prompt }}</p>
          </template>
        </el-table-column>
        <el-table-column label="评测上下文" min-width="220">
          <template #default="{ row }">
            <strong>{{ row.dataset_name }} V{{ row.version_no }}</strong>
            <p class="table-secondary-text">{{ row.evaluation_target_name }} / {{ row.scenario_name }}</p>
            <small>{{ row.evaluator_name }} · {{ row.agent_version }} · {{ row.environment }}</small>
          </template>
        </el-table-column>
        <el-table-column label="结果" min-width="300">
          <template #default="{ row }">
            <template v-if="row.result_status === 'skipped'">
              <el-tag type="warning">已跳过</el-tag>
              <span class="result-inline-note">{{ row.skip_reason }}</span>
            </template>
            <template v-else>
              <el-rate :model-value="row.score || 0" disabled />
              <p class="table-secondary-text">{{ row.answer_text || '仅上传了截图证据' }}</p>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="Badcase" width="170">
          <template #default="{ row }">
            <el-button
              v-if="row.badcase_id"
              link
              type="danger"
              @click="router.push(`/badcases/${row.badcase_id}`)"
            >
              {{ row.badcase_title }}
            </el-button>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="完成时间" width="180">
          <template #default="{ row }">{{ formatTime(row.completed_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="query.page"
        class="table-pagination"
        layout="total, prev, pager, next"
        :page-size="query.page_size"
        :total="total"
        @current-change="load"
      />
    </el-card>
  </section>
</template>
