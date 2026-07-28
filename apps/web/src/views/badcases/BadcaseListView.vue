<script setup lang="ts">
import { Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { Badcase, PageData, ResponseEnvelope, Scenario, Tag } from '@/api/types'

const router = useRouter()
const loading = ref(false)
const items = ref<Badcase[]>([])
const total = ref(0)
const scenarios = ref<Scenario[]>([])
const issueTags = ref<Tag[]>([])
const query = reactive({
  page: 1,
  page_size: 20,
  keyword: '',
  status: '',
  scenario_id: '',
  issue_tag_id: '',
})

async function loadOptions() {
  try {
    const [scenarioResponse, tagResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<ResponseEnvelope<{ items: Tag[] }>>('/api/v1/issue-tags'),
    ])
    scenarios.value = scenarioResponse.data.data.items
    issueTags.value = tagResponse.data.data.items
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<PageData<Badcase>>>(
      '/api/v1/badcases',
      { params: query },
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

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

onMounted(() => Promise.all([loadOptions(), load()]))
</script>

<template>
  <section class="badcase-list-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">问题沉淀</p>
        <h1>Badcase 中心</h1>
        <p>汇总评测中发现的问题，保留原始回答、截图证据和评测上下文。</p>
      </div>
    </div>

    <el-card shadow="never">
      <div class="badcase-filter-bar">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="搜索标题、描述或 Agent 回答"
          :prefix-icon="Search"
          @keyup.enter="search"
        />
        <el-select v-model="query.scenario_id" clearable placeholder="全部场景" @change="search">
          <el-option
            v-for="scenario in scenarios"
            :key="scenario.id"
            :label="scenario.name"
            :value="scenario.id"
          />
        </el-select>
        <el-select v-model="query.issue_tag_id" clearable placeholder="全部标签" @change="search">
          <el-option v-for="tag in issueTags" :key="tag.id" :label="tag.name" :value="tag.id" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="全部状态" @change="search">
          <el-option label="待处理" value="pending" />
          <el-option label="处理中" value="processing" />
          <el-option label="已解决" value="resolved" />
        </el-select>
        <el-button type="primary" @click="search">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="items" row-key="id">
        <el-table-column label="Badcase" min-width="280">
          <template #default="{ row }">
            <button class="table-primary-link" @click="router.push(`/badcases/${row.id}`)">
              {{ row.title }}
            </button>
            <small>{{ row.evaluation?.case_name || row.evaluation?.user_prompt || '-' }}</small>
          </template>
        </el-table-column>
        <el-table-column prop="scenario_name" label="场景" min-width="140" />
        <el-table-column label="问题标签" min-width="190">
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
        <el-table-column label="评分" width="80">
          <template #default="{ row }">{{ row.evaluation?.score || '-' }}</template>
        </el-table-column>
        <el-table-column label="评测人" width="120">
          <template #default="{ row }">{{ row.evaluation?.evaluator_name || row.creator_name }}</template>
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
</template>
