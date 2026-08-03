<script setup lang="ts">
import { Plus, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  CatalogItem,
  Dataset,
  DatasetDetail,
  DatasetVersion,
  PageData,
  ResponseEnvelope,
  Scenario,
  VersionCase,
} from '@/api/types'
import { caseDisplayName } from '@/features/datasets/case-display'
import { useAuthStore } from '@/stores/auth'
import ActionableEmptyState from '@/components/app/ActionableEmptyState.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const datasets = ref<Dataset[]>([])
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const expandedDatasetIds = ref<string[]>([])
const previewDatasetId = ref('')
const previewVersion = ref<DatasetVersion | null>(null)
const previewCases = ref<VersionCase[]>([])
const previewTotal = ref(0)
const previewLoading = ref(false)
const createDialog = ref(false)
const includeDisabledCatalog = ref(false)
const filters = reactive({
  evaluation_target_id: String(route.query.evaluation_target_id || ''),
  scenario_id: String(route.query.scenario_id || ''),
  status: '',
  keyword: '',
})
const form = reactive({ evaluation_target_id: '', name: '', description: '' })
let previewRequest = 0

const activeTargetIds = computed(
  () => new Set(targets.value.filter((item) => item.status === 'active').map((item) => item.id)),
)
const selectableTargets = computed(() =>
  targets.value.filter((item) => item.status === 'active'),
)
const selectableScenarios = computed(() =>
  scenarios.value.filter(
    (item) => item.status === 'active' && activeTargetIds.value.has(item.evaluation_target_id),
  ),
)
const catalogTargets = computed(() =>
  includeDisabledCatalog.value ? targets.value : selectableTargets.value,
)
const catalogScenarios = computed(() =>
  includeDisabledCatalog.value ? scenarios.value : selectableScenarios.value,
)
const visibleScenarios = computed(() =>
  filters.evaluation_target_id
    ? catalogScenarios.value.filter(
        (item) => item.evaluation_target_id === filters.evaluation_target_id,
      )
    : [],
)
const visibleDatasets = computed(() =>
  includeDisabledCatalog.value
    ? datasets.value
    : datasets.value.filter((item) => datasetOwnershipActive(item)),
)
const hasFilters = computed(() => Boolean(
  filters.evaluation_target_id || filters.scenario_id || filters.status || filters.keyword ||
  includeDisabledCatalog.value,
))

function datasetOwnershipActive(item: Dataset) {
  return item.evaluation_target_status !== 'disabled'
}

async function loadCatalog() {
  const [targetResponse, scenarioResponse] = await Promise.all([
    apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>(
      '/api/v1/evaluation-targets?page_size=100',
    ),
    apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios?page_size=100'),
  ])
  targets.value = targetResponse.data.data.items
  scenarios.value = scenarioResponse.data.data.items
  if (filters.scenario_id && !filters.evaluation_target_id) {
    filters.evaluation_target_id =
      scenarios.value.find((item) => item.id === filters.scenario_id)?.evaluation_target_id || ''
  }
  if (
    filters.scenario_id &&
    !selectableScenarios.value.some((item) => item.id === filters.scenario_id)
  ) {
    filters.scenario_id = ''
  }
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<PageData<Dataset>>>('/api/v1/datasets', {
      params: { page_size: 100, ...filters },
    })
    datasets.value = response.data.data.items
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, {
    evaluation_target_id:
      selectableTargets.value.find((item) => item.id === filters.evaluation_target_id)?.id ||
      selectableTargets.value[0]?.id ||
      '',
    name: '',
    description: '',
  })
  createDialog.value = true
}

function resetFilters() {
  Object.assign(filters, { evaluation_target_id: '', scenario_id: '', status: '', keyword: '' })
  includeDisabledCatalog.value = false
  load()
}

function changeTarget() {
  filters.scenario_id = ''
  load()
}

async function loadPreview(row: Dataset) {
  const request = ++previewRequest
  previewDatasetId.value = row.id
  previewVersion.value = null
  previewCases.value = []
  previewTotal.value = 0
  previewLoading.value = true
  try {
    const detailResponse = await apiClient.get<ResponseEnvelope<DatasetDetail>>(
      `/api/v1/datasets/${row.id}`,
    )
    const latestVersion = detailResponse.data.data.versions
      .filter((version) => version.status === 'published')
      .sort((left, right) => (right.version_no || 0) - (left.version_no || 0))[0]
    if (!latestVersion) return
    const casesResponse = await apiClient.get<ResponseEnvelope<PageData<VersionCase>>>(
      `/api/v1/dataset-versions/${latestVersion.id}/cases`,
      { params: { page: 1, page_size: 8 } },
    )
    if (request !== previewRequest) return
    previewVersion.value = latestVersion
    previewCases.value = casesResponse.data.data.items
    previewTotal.value = casesResponse.data.data.total
  } catch (error) {
    if (request !== previewRequest) return
    ElMessage.error(apiErrorMessage(error))
  } finally {
    if (request === previewRequest) previewLoading.value = false
  }
}

function handlePreviewChange(row: Dataset, expandedRows: Dataset[]) {
  const expanded = expandedRows.some((item) => item.id === row.id)
  if (!expanded) {
    if (previewDatasetId.value === row.id) {
      previewRequest += 1
      previewDatasetId.value = ''
      previewLoading.value = false
    }
    return
  }
  expandedDatasetIds.value = [row.id]
  if (previewDatasetId.value !== row.id) void loadPreview(row)
}

function togglePreview(row: Dataset) {
  const expanded = expandedDatasetIds.value.includes(row.id)
  expandedDatasetIds.value = expanded ? [] : [row.id]
  if (!expanded) {
    void loadPreview(row)
    return
  }
  if (previewDatasetId.value === row.id) {
    previewRequest += 1
    previewDatasetId.value = ''
    previewLoading.value = false
  }
}

async function createDataset() {
  try {
    const response = await apiClient.post<
      ResponseEnvelope<{ dataset: Dataset; draft: { id: string } }>
    >('/api/v1/datasets', {
      evaluation_target_id: form.evaluation_target_id,
      name: form.name,
      description: form.description || null,
    })
    createDialog.value = false
    ElMessage.success('评测集及首个草稿已创建')
    await router.push(`/dataset-versions/${response.data.data.draft.id}/edit`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

watch(
  () => filters.evaluation_target_id,
  () => {
    if (
      filters.scenario_id &&
      !visibleScenarios.value.some((item) => item.id === filters.scenario_id)
    ) {
      filters.scenario_id = ''
    }
  },
)

watch(includeDisabledCatalog, (included) => {
  if (included) return
  if (!selectableTargets.value.some((item) => item.id === filters.evaluation_target_id)) {
    filters.evaluation_target_id = ''
  }
  if (!selectableScenarios.value.some((item) => item.id === filters.scenario_id)) {
    filters.scenario_id = ''
  }
})

onMounted(async () => {
  await loadCatalog()
  await load()
})
</script>

<template>
  <section class="dataset-page">
    <div class="dataset-layout">
      <el-card class="dataset-list-card" shadow="never">
        <div v-if="auth.isAdmin" class="content-primary-actions">
          <el-button type="primary" :icon="Plus" @click="openCreate">新建评测集</el-button>
        </div>
        <div class="dataset-filter-bar">
          <el-input
            v-model="filters.keyword"
            :prefix-icon="Search"
            placeholder="搜索评测集"
            aria-label="搜索评测集"
            clearable
            @keyup.enter="load"
          />
          <el-select
            v-model="filters.evaluation_target_id"
            clearable
            filterable
            placeholder="全部评测对象"
            aria-label="按评测对象筛选"
            @change="changeTarget"
          >
            <el-option
              v-for="target in catalogTargets"
              :key="target.id"
              :label="target.status === 'disabled' ? `${target.name}（停用）` : target.name"
              :value="target.id"
            />
          </el-select>
          <el-select
            v-model="filters.scenario_id"
            clearable
            filterable
            :disabled="!filters.evaluation_target_id"
            :placeholder="filters.evaluation_target_id ? '全部场景' : '请先选择评测对象'"
            aria-label="按场景筛选"
            @change="load"
          >
            <el-option
              v-for="scenario in visibleScenarios"
              :key="scenario.id"
              :label="scenario.status === 'disabled' ? `${scenario.name}（停用）` : scenario.name"
              :value="scenario.id"
            />
          </el-select>
          <el-select
            v-if="auth.isAdmin"
            v-model="filters.status"
            clearable
            placeholder="全部状态"
            aria-label="按评测集状态筛选"
            @change="load"
          >
            <el-option label="活跃" value="active" />
            <el-option label="已归档" value="archived" />
          </el-select>
          <div
            class="filter-actions dataset-filter-actions"
            :class="{ 'has-reset': hasFilters }"
          >
            <el-button type="primary" @click="load">查询</el-button>
            <el-button v-if="hasFilters" @click="resetFilters">重置</el-button>
          </div>
        </div>
        <div v-if="auth.isAdmin" class="dataset-filter-options">
          <el-checkbox v-model="includeDisabledCatalog">包含停用归属</el-checkbox>
          <span>场景仅在选定评测对象后可用</span>
        </div>
        <div class="dataset-toolbar">
          <div>
            <strong>{{ visibleDatasets.length }} 个评测集</strong>
            <span>草稿与已发布版本始终相互隔离</span>
          </div>
        </div>
        <el-table
          v-loading="loading"
          :data="visibleDatasets"
          :expand-row-keys="expandedDatasetIds"
          row-key="id"
          row-class-name="clickable-row"
          @expand-change="handlePreviewChange"
          @row-click="(row: Dataset) => router.push(`/datasets/${row.id}`)"
        >
          <template #empty>
            <ActionableEmptyState
              :title="hasFilters ? '没有符合条件的评测集' : auth.isAdmin ? '还没有评测集' : '暂无可开始的评测集'"
              :description="hasFilters ? '调整对象、场景或关键词后再试。' : auth.isAdmin ? '先创建评测集，再维护并发布首个用例版本。' : '管理员发布评测集版本后，你可以从这里发起人工评测。'"
              :action-label="hasFilters ? '清除筛选' : auth.isAdmin ? '创建首个评测集' : ''"
              compact
              @action="hasFilters ? resetFilters() : openCreate()"
            />
          </template>
          <el-table-column type="expand" width="64" label="预览">
            <template #default="{ row }">
              <div
                v-loading="previewLoading && previewDatasetId === row.id"
                class="dataset-case-preview"
              >
                <template v-if="previewDatasetId === row.id && previewVersion">
                  <div class="dataset-case-preview-heading">
                    <div>
                      <strong>最新已发布版本 V{{ previewVersion.version_no }}</strong>
                      <span>显示前 {{ previewCases.length }} 条，共 {{ previewTotal }} 条</span>
                    </div>
                    <el-button link type="primary" @click.stop="router.push(`/datasets/${row.id}`)">
                      查看全部用例
                    </el-button>
                  </div>
                  <ol class="dataset-case-preview-list">
                    <li v-for="(item, index) in previewCases" :key="item.id">
                      <span class="case-sequence">{{ index + 1 }}</span>
                      <button
                        class="table-primary-link"
                        @click.stop="router.push(`/version-cases/${item.id}`)"
                      >
                        {{ caseDisplayName(item.name, item.user_prompt) }}
                      </button>
                      <span class="dataset-case-preview-prompt">{{ item.user_prompt }}</span>
                    </li>
                  </ol>
                </template>
                <el-empty
                  v-else-if="previewDatasetId === row.id && !previewLoading"
                  description="最新已发布版本暂无用例"
                  :image-size="56"
                />
              </div>
            </template>
          </el-table-column>
          <el-table-column label="评测集名称" min-width="250">
            <template #default="{ row }">
              <a class="dataset-name" @click.stop="router.push(`/datasets/${row.id}`)">
                {{ row.name }}
              </a>
              <div class="table-secondary">
                {{ row.evaluation_target_name }}
                <el-tag v-if="!datasetOwnershipActive(row)" type="info" size="small">归属已停用</el-tag>
              </div>
              <div class="table-description">{{ row.description || '暂无说明' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="最新版本" width="120">
            <template #default="{ row }">
              <el-tag v-if="row.latest_version_no" type="success">
                V{{ row.latest_version_no }} 已发布
              </el-tag>
              <span v-else class="muted">尚未发布</span>
            </template>
          </el-table-column>
          <el-table-column label="草稿状态" width="120">
            <template #default="{ row }">
              <el-tag v-if="row.draft_version_id" type="warning">有草稿</el-tag>
              <span v-else class="muted">无草稿</span>
            </template>
          </el-table-column>
          <el-table-column label="草稿用例" width="110">
            <template #default="{ row }">{{ row.draft_case_count }} 条</template>
          </el-table-column>
          <el-table-column prop="updated_at" label="最近更新" min-width="180">
            <template #default="{ row }">{{ new Date(row.updated_at).toLocaleString() }}</template>
          </el-table-column>
          <el-table-column label="操作" width="210" align="right">
            <template #default="{ row }">
              <el-button
                link
                :disabled="!row.latest_version_no"
                @click.stop="togglePreview(row)"
              >
                {{ expandedDatasetIds.includes(row.id) ? '收起预览' : '预览用例' }}
              </el-button>
              <el-button link type="primary" @click.stop="router.push(`/datasets/${row.id}`)">
                查看详情
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>
  </section>

  <el-dialog v-model="createDialog" title="新建评测集" width="560">
    <el-form label-position="top">
      <el-form-item label="所属评测对象" required>
        <el-select v-model="form.evaluation_target_id" filterable aria-label="所属评测对象">
          <el-option
            v-for="target in selectableTargets"
            :key="target.id"
            :label="target.name"
            :value="target.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="评测集名称" required>
        <el-input v-model="form.name" maxlength="200" show-word-limit />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="4" maxlength="10000" />
      </el-form-item>
      <el-alert
        title="创建评测集时会同时生成唯一的首个草稿。"
        type="info"
        :closable="false"
      />
    </el-form>
    <template #footer>
      <el-button @click="createDialog = false">取消</el-button>
      <el-button
        type="primary"
        :disabled="!form.evaluation_target_id || !form.name.trim()"
        @click="createDataset"
      >
        创建并编辑草稿
      </el-button>
    </template>
  </el-dialog>
</template>
