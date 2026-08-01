<script setup lang="ts">
import { Collection, Plus, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  CatalogItem,
  Dataset,
  PageData,
  ResponseEnvelope,
  Scenario,
} from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import ActionableEmptyState from '@/components/app/ActionableEmptyState.vue'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const datasets = ref<Dataset[]>([])
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const createDialog = ref(false)
const filters = reactive({
  evaluation_target_id: '',
  scenario_id: String(route.query.scenario_id || ''),
  status: '',
  keyword: '',
})
const form = reactive({ scenario_id: '', name: '', description: '' })

const visibleScenarios = computed(() =>
  filters.evaluation_target_id
    ? scenarios.value.filter(
        (item) => item.evaluation_target_id === filters.evaluation_target_id,
      )
    : scenarios.value,
)
const hasFilters = computed(() => Boolean(filters.evaluation_target_id || filters.scenario_id || filters.status || filters.keyword))

async function loadCatalog() {
  const [targetResponse, scenarioResponse] = await Promise.all([
    apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>(
      '/api/v1/evaluation-targets?page_size=100',
    ),
    apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios?page_size=100'),
  ])
  targets.value = targetResponse.data.data.items
  scenarios.value = scenarioResponse.data.data.items
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
    scenario_id: filters.scenario_id || visibleScenarios.value[0]?.id || '',
    name: '',
    description: '',
  })
  createDialog.value = true
}

function resetFilters() {
  Object.assign(filters, { evaluation_target_id: '', scenario_id: '', status: '', keyword: '' })
  load()
}

async function createDataset() {
  try {
    const response = await apiClient.post<
      ResponseEnvelope<{ dataset: Dataset; draft: { id: string } }>
    >('/api/v1/datasets', {
      scenario_id: form.scenario_id,
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

onMounted(async () => {
  await loadCatalog()
  await load()
})
</script>

<template>
  <section class="dataset-page">
    <div class="page-heading">
      <div>
        <p>按评测对象和场景管理稳定、可追溯的用例版本。</p>
      </div>
      <el-button v-if="auth.isAdmin" type="primary" :icon="Plus" @click="openCreate">
        新建评测集
      </el-button>
    </div>

    <div class="dataset-layout">
      <el-card class="dataset-filter-panel" shadow="never">
        <el-input
          v-model="filters.keyword"
          :prefix-icon="Search"
          placeholder="搜索评测集"
          aria-label="搜索评测集"
          clearable
          @keyup.enter="load"
        />
        <div class="filter-block">
          <span class="filter-label">评测对象</span>
          <button
            class="filter-choice"
            :class="{ selected: !filters.evaluation_target_id }"
            @click="filters.evaluation_target_id = ''; load()"
          >
            全部对象
          </button>
          <button
            v-for="target in targets"
            :key="target.id"
            class="filter-choice"
            :class="{ selected: filters.evaluation_target_id === target.id }"
            @click="filters.evaluation_target_id = target.id; load()"
          >
            <el-icon><Collection /></el-icon>{{ target.name }}
          </button>
        </div>
        <div class="filter-block">
          <span class="filter-label">评测场景</span>
          <button
            class="filter-choice"
            :class="{ selected: !filters.scenario_id }"
            @click="filters.scenario_id = ''; load()"
          >
            全部场景
          </button>
          <button
            v-for="scenario in visibleScenarios"
            :key="scenario.id"
            class="filter-choice child"
            :class="{ selected: filters.scenario_id === scenario.id }"
            @click="filters.scenario_id = scenario.id; load()"
          >
            {{ scenario.name }}
          </button>
        </div>
      </el-card>

      <el-card class="dataset-list-card" shadow="never">
        <div class="dataset-toolbar">
          <div>
            <strong>{{ datasets.length }} 个评测集</strong>
            <span>草稿与已发布版本始终相互隔离</span>
          </div>
          <el-select v-if="auth.isAdmin" v-model="filters.status" clearable placeholder="全部状态" aria-label="按评测集状态筛选" @change="load">
            <el-option label="活跃" value="active" />
            <el-option label="已归档" value="archived" />
          </el-select>
        </div>
        <el-table
          v-loading="loading"
          :data="datasets"
          row-class-name="clickable-row"
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
          <el-table-column label="评测集名称" min-width="250">
            <template #default="{ row }">
              <a class="dataset-name" @click.stop="router.push(`/datasets/${row.id}`)">
                {{ row.name }}
              </a>
              <div class="table-secondary">
                {{ row.evaluation_target_name }} / {{ row.scenario_name }}
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
          <el-table-column label="操作" width="130" align="right">
            <template #default="{ row }">
              <el-button
                v-if="auth.isAdmin && row.draft_version_id"
                link
                type="primary"
                @click.stop="router.push(`/dataset-versions/${row.draft_version_id}/edit`)"
              >
                编辑草稿
              </el-button>
              <el-button v-else link type="primary" @click.stop="router.push(`/datasets/${row.id}`)">
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
      <el-form-item label="所属场景" required>
        <el-select v-model="form.scenario_id" filterable aria-label="所属场景">
          <el-option
            v-for="scenario in scenarios"
            :key="scenario.id"
            :label="`${scenario.evaluation_target_name} / ${scenario.name}`"
            :value="scenario.id"
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
        :disabled="!form.scenario_id || !form.name.trim()"
        @click="createDataset"
      >
        创建并编辑草稿
      </el-button>
    </template>
  </el-dialog>
</template>
