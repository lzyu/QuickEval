<script setup lang="ts">
import { Download, Refresh, Search } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { DashboardData, DistributionItem, ResponseEnvelope } from '@/api/types'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const data = ref<DashboardData | null>(null)
const scoreChart = ref<HTMLElement | null>(null)
const tagChart = ref<HTMLElement | null>(null)
const statusChart = ref<HTMLElement | null>(null)
const skipChart = ref<HTMLElement | null>(null)
const versionChart = ref<HTMLElement | null>(null)
const charts: echarts.ECharts[] = []
const filters = reactive({
  evaluation_target_id: String(route.query.evaluation_target_id || ''),
  scenario_id: String(route.query.scenario_id || ''),
  dataset_id: String(route.query.dataset_id || ''),
  dataset_version_id: String(route.query.dataset_version_id || ''),
  evaluator_id: String(route.query.evaluator_id || ''),
  agent_version: String(route.query.agent_version || ''),
  environment: String(route.query.environment || ''),
  source_type: String(route.query.source_type || ''),
  badcase_status: String(route.query.badcase_status || ''),
  issue_tag_id: String(route.query.issue_tag_id || ''),
  dateRange:
    route.query.from && route.query.to
      ? [String(route.query.from).slice(0, 10), String(route.query.to).slice(0, 10)]
      : ([] as string[]),
})

const scenarios = computed(() =>
  (data.value?.options.scenarios || []).filter(
    (item) => !filters.evaluation_target_id || item.parent_id === filters.evaluation_target_id,
  ),
)
const datasets = computed(() =>
  (data.value?.options.datasets || []).filter(
    (item) => !filters.scenario_id || item.parent_id === filters.scenario_id,
  ),
)
const versions = computed(() =>
  (data.value?.options.dataset_versions || []).filter(
    (item) => !filters.dataset_id || item.parent_id === filters.dataset_id,
  ),
)

function requestParams() {
  return {
    ...Object.fromEntries(
      Object.entries(filters).filter(([key, value]) => key !== 'dateRange' && value),
    ),
    ...(filters.dateRange.length === 2
      ? {
          from: new Date(`${filters.dateRange[0]}T00:00:00`).toISOString(),
          to: new Date(`${filters.dateRange[1]}T23:59:59.999`).toISOString(),
        }
      : {}),
  }
}

async function load() {
  loading.value = true
  try {
    const params = requestParams()
    const response = await apiClient.get<ResponseEnvelope<DashboardData>>(
      '/api/v1/pages/dashboard',
      { params },
    )
    data.value = response.data.data
    await router.replace({ query: params })
    await nextTick()
    renderCharts()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function reset() {
  Object.assign(filters, {
    evaluation_target_id: '',
    scenario_id: '',
    dataset_id: '',
    dataset_version_id: '',
    evaluator_id: '',
    agent_version: '',
    environment: '',
    source_type: '',
    badcase_status: '',
    issue_tag_id: '',
    dateRange: [],
  })
  load()
}

function clearCharts() {
  while (charts.length) charts.pop()?.dispose()
}

function barOption(items: DistributionItem[], color: string, horizontal = false): echarts.EChartsOption {
  return {
    color: [color],
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: horizontal ? 110 : 42, right: 24, top: 22, bottom: 42 },
    xAxis: horizontal
      ? { type: 'value', minInterval: 1 }
      : { type: 'category', data: items.map((item) => item.label), axisLabel: { interval: 0 } },
    yAxis: horizontal
      ? { type: 'category', data: items.map((item) => item.label), axisLabel: { width: 95, overflow: 'truncate' } }
      : { type: 'value', minInterval: 1 },
    series: [
      {
        type: 'bar',
        data: items.map((item) => item.count),
        barMaxWidth: 38,
        label: { show: true, position: horizontal ? 'right' : 'top' },
      },
    ],
  }
}

function renderCharts() {
  if (!data.value) return
  clearCharts()
  const connect = (
    element: HTMLElement | null,
    option: echarts.EChartsOption,
    click?: (index: number) => void,
  ) => {
    if (!element) return
    const chart = echarts.init(element)
    chart.setOption(option)
    if (click) chart.on('click', (event) => click(event.dataIndex))
    charts.push(chart)
  }
  connect(scoreChart.value, barOption(data.value.score_distribution, '#2563eb'))
  connect(
    tagChart.value,
    barOption(data.value.issue_tag_distribution.slice(0, 10), '#ef4444', true),
    (index) => drillBadcase({ issue_tag_id: data.value?.issue_tag_distribution[index]?.key }),
  )
  connect(
    statusChart.value,
    {
      tooltip: { trigger: 'item' },
      legend: { bottom: 0 },
      series: [{
        type: 'pie',
        radius: ['42%', '68%'],
        data: data.value.status_distribution.map((item) => ({ name: item.label, value: item.count })),
        label: { formatter: '{b}\n{c}' },
      }],
    },
    (index) => drillBadcase({ status: data.value?.status_distribution[index]?.key }),
  )
  connect(skipChart.value, barOption(data.value.skip_reason_distribution.slice(0, 8), '#f59e0b', true))
  connect(
    versionChart.value,
    {
      tooltip: { trigger: 'axis' },
      legend: { data: ['平均分', 'Badcase 率'] },
      grid: { left: 44, right: 50, top: 42, bottom: 38 },
      xAxis: {
        type: 'category',
        data: data.value.version_comparison.map((item) => `V${item.version_no}`),
      },
      yAxis: [
        { type: 'value', name: '平均分', min: 0, max: 5 },
        { type: 'value', name: '比例', min: 0, max: 1, axisLabel: { formatter: '{value}' } },
      ],
      series: [
        {
          name: '平均分',
          type: 'line',
          connectNulls: false,
          data: data.value.version_comparison.map((item) => item.average_score),
        },
        {
          name: 'Badcase 率',
          type: 'line',
          yAxisIndex: 1,
          connectNulls: false,
          data: data.value.version_comparison.map((item) => item.evaluation_badcase_rate),
        },
      ],
    },
  )
}

function drillBadcase(extra: Record<string, string | undefined>) {
  const query = {
    scenario_id: filters.scenario_id || undefined,
    source_type: filters.source_type || undefined,
    ...extra,
  }
  router.push({ path: '/badcases', query })
}

function download(path: string) {
  const query = new URLSearchParams(
    Object.entries(requestParams()).reduce<Record<string, string>>((result, [key, value]) => {
      if (typeof value === 'string') result[key] = value
      return result
    }, {}),
  )
  window.location.href = `${path}?${query.toString()}`
}

function formatAverage(value: number | null) {
  return value == null ? '暂无样本' : value.toFixed(2)
}

function formatRate(value: number | null) {
  return value == null ? '暂无样本' : `${(value * 100).toFixed(1)}%`
}

function resizeCharts() {
  charts.forEach((chart) => chart.resize())
}

watch(() => filters.evaluation_target_id, () => {
  if (filters.scenario_id && !scenarios.value.some((item) => item.id === filters.scenario_id)) {
    filters.scenario_id = ''
  }
})
watch(() => filters.scenario_id, () => {
  if (filters.dataset_id && !datasets.value.some((item) => item.id === filters.dataset_id)) {
    filters.dataset_id = ''
  }
})
watch(() => filters.dataset_id, () => {
  if (
    filters.dataset_version_id &&
    !versions.value.some((item) => item.id === filters.dataset_version_id)
  ) {
    filters.dataset_version_id = ''
  }
})

onMounted(() => {
  window.addEventListener('resize', resizeCharts)
  load()
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts)
  clearCharts()
})
</script>

<template>
  <section v-loading="loading" class="dashboard-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">QUALITY INSIGHTS</p>
        <h1>数据看板</h1>
        <p>只统计已完成评测与有效 Badcase；所有指标都可追溯到当前筛选口径。</p>
      </div>
      <div class="page-actions">
        <el-dropdown>
          <el-button :icon="Download">导出当前筛选</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="download('/api/v1/exports/evaluation-results.csv')">
                评测结果 CSV
              </el-dropdown-item>
              <el-dropdown-item @click="download('/api/v1/exports/badcases.csv')">
                Badcase CSV
              </el-dropdown-item>
              <el-dropdown-item @click="download('/api/v1/exports/badcase-distribution.csv')">
                Badcase 分布 CSV
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button type="primary" :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <el-card v-if="data" class="dashboard-filter-card" shadow="never">
      <div class="dashboard-filter-grid">
        <el-select v-model="filters.evaluation_target_id" clearable placeholder="评测对象">
          <el-option v-for="item in data.options.evaluation_targets" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.scenario_id" clearable placeholder="场景">
          <el-option v-for="item in scenarios" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.dataset_id" clearable placeholder="评测集">
          <el-option v-for="item in datasets" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.dataset_version_id" clearable placeholder="版本">
          <el-option v-for="item in versions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.evaluator_id" clearable filterable placeholder="评测人员">
          <el-option v-for="item in data.options.evaluators" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="filters.agent_version" clearable filterable allow-create placeholder="Agent 版本">
          <el-option v-for="item in data.options.agent_versions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select v-model="filters.environment" clearable placeholder="运行环境">
          <el-option label="测试" value="test" /><el-option label="预发布" value="staging" />
          <el-option label="生产" value="production" /><el-option label="其他" value="other" />
        </el-select>
        <el-select v-model="filters.source_type" clearable placeholder="Badcase 来源">
          <el-option label="评测发现" value="evaluation" /><el-option label="业务登记" value="business" />
        </el-select>
        <el-select v-model="filters.badcase_status" clearable placeholder="Badcase 状态">
          <el-option label="待处理" value="pending" /><el-option label="处理中" value="processing" />
          <el-option label="已解决" value="resolved" /><el-option label="暂不处理" value="deferred" />
        </el-select>
        <el-select v-model="filters.issue_tag_id" clearable placeholder="问题标签">
          <el-option v-for="item in data.options.issue_tags" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-date-picker
          v-model="filters.dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
        />
        <div class="filter-actions">
          <el-button :icon="Search" type="primary" @click="load">应用筛选</el-button>
          <el-button @click="reset">重置</el-button>
        </div>
      </div>
    </el-card>

    <template v-if="data">
      <div class="dashboard-metric-grid">
        <article><span>已完成评测</span><strong>{{ data.metrics.completed_run_count }}</strong><small>次</small></article>
        <article><span>已评用例</span><strong>{{ data.metrics.evaluated_case_count }}</strong><small>条</small></article>
        <article><span>平均分</span><strong>{{ formatAverage(data.metrics.average_score) }}</strong><small>{{ data.metrics.scored_case_count }} 条已评分</small></article>
        <article><span>评测 Badcase 率</span><strong>{{ formatRate(data.metrics.evaluation_badcase_rate) }}</strong><small>{{ data.metrics.evaluation_badcase_count }} 个</small></article>
        <article><span>有效 Badcase</span><strong>{{ data.metrics.valid_badcase_count }}</strong><small>评测与业务来源</small></article>
        <article><span>跳过用例</span><strong>{{ data.metrics.skipped_case_count }}</strong><small>不进入评分分母</small></article>
      </div>

      <div class="dashboard-chart-grid">
        <el-card shadow="never">
          <template #header><strong>1～5 分分布</strong></template>
          <el-empty v-if="data.metrics.scored_case_count === 0" description="当前筛选没有已评分样本" />
          <div v-else ref="scoreChart" class="dashboard-chart"></div>
        </el-card>
        <el-card shadow="never">
          <template #header><strong>Badcase 状态分布</strong></template>
          <el-empty v-if="data.metrics.valid_badcase_count === 0" description="当前筛选没有有效 Badcase" />
          <div v-else ref="statusChart" class="dashboard-chart"></div>
        </el-card>
        <el-card shadow="never">
          <template #header><strong>问题标签分布</strong></template>
          <el-empty v-if="data.metrics.valid_badcase_count === 0" description="当前筛选没有有效 Badcase" />
          <div v-else ref="tagChart" class="dashboard-chart tall"></div>
        </el-card>
        <el-card shadow="never">
          <template #header><strong>跳过原因分布</strong></template>
          <el-empty v-if="data.metrics.skipped_case_count === 0" description="当前筛选没有跳过用例" />
          <div v-else ref="skipChart" class="dashboard-chart tall"></div>
        </el-card>
        <el-card v-if="filters.dataset_id" class="dashboard-version-card" shadow="never">
          <template #header><strong>评测集版本对比</strong></template>
          <el-empty v-if="data.version_comparison.length === 0" description="当前评测集暂无已完成评测样本" />
          <div v-else ref="versionChart" class="dashboard-chart tall"></div>
        </el-card>
      </div>
    </template>
  </section>
</template>
