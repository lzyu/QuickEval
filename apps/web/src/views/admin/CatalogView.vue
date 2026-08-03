<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref, watch } from 'vue'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  CatalogItem,
  PageData,
  ResponseEnvelope,
  Scenario,
  Tag,
} from '@/api/types'

type Kind = 'target' | 'scenario' | 'case-tag'
type CaseTagScope = 'global' | 'target'

const activeTab = ref<Kind>('target')
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const caseTags = ref<Tag[]>([])
const selectedTarget = ref('')
const tagScope = ref<CaseTagScope>('global')
const dialog = ref(false)
const editing = ref<CatalogItem | Scenario | Tag | null>(null)
const form = reactive({
  name: '',
  description: '',
  evaluation_target_id: '',
  scope: 'global' as CaseTagScope,
})

const tableData = computed(() => {
  if (activeTab.value === 'target') return targets.value
  if (activeTab.value === 'scenario') {
    return selectedTarget.value
      ? scenarios.value.filter((item) => item.evaluation_target_id === selectedTarget.value)
      : scenarios.value
  }
  return caseTags.value
})
const primaryActionLabel = computed(() => {
  if (activeTab.value === 'target') return '新建评测对象'
  if (activeTab.value === 'scenario') return '新建评测场景'
  return '新建用例标签'
})
const dialogTitle = computed(() => {
  const entity = activeTab.value === 'target' ? '评测对象' : activeTab.value === 'scenario' ? '评测场景' : '用例标签'
  return `${editing.value ? '编辑' : '新建'}${entity}`
})
const nameLabel = computed(() => {
  if (activeTab.value === 'target') return '评测对象名称'
  if (activeTab.value === 'scenario') return '评测场景名称'
  return '用例标签名称'
})

async function load() {
  const [targetResponse, scenarioResponse] = await Promise.all([
    apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>(
      '/api/v1/evaluation-targets?page_size=100',
    ),
    apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios?page_size=100'),
  ])
  targets.value = targetResponse.data.data.items
  scenarios.value = scenarioResponse.data.data.items
  await loadCaseTags()
}

async function loadCaseTags() {
  if (tagScope.value === 'target' && !selectedTarget.value) {
    caseTags.value = []
    return
  }
  const params = new URLSearchParams({ scope: tagScope.value })
  if (tagScope.value === 'target') {
    params.set('evaluation_target_id', selectedTarget.value)
  }
  const response = await apiClient.get<ResponseEnvelope<{ items: Tag[] }>>(
    `/api/v1/case-tags?${params.toString()}`,
  )
  caseTags.value = response.data.data.items
}

function openCreate() {
  editing.value = null
  const targetID = selectedTarget.value || targets.value[0]?.id || ''
  if (activeTab.value === 'case-tag' && tagScope.value === 'target' && !selectedTarget.value) {
    selectedTarget.value = targetID
  }
  Object.assign(form, {
    name: '',
    description: '',
    evaluation_target_id: targetID,
    scope: tagScope.value,
  })
  dialog.value = true
}

function openScenarioCreate(targetID: string) {
  selectedTarget.value = targetID
  activeTab.value = 'scenario'
  openCreate()
}

function openTagCreate(targetID: string) {
  selectedTarget.value = targetID
  tagScope.value = 'target'
  activeTab.value = 'case-tag'
  openCreate()
}

function openEdit(item: CatalogItem | Scenario | Tag) {
  editing.value = item
  Object.assign(form, {
    name: item.name,
    description: item.description || '',
    evaluation_target_id:
      'evaluation_target_id' in item ? item.evaluation_target_id : targets.value[0]?.id || '',
    scope: 'scope' in item && item.scope ? item.scope : tagScope.value,
  })
  dialog.value = true
}

async function save() {
  const payload = {
    name: form.name,
    description: form.description || null,
    expected_lock_version: editing.value?.lock_version || 0,
  }
  try {
    if (activeTab.value === 'target') {
      const path = editing.value
        ? `/api/v1/evaluation-targets/${editing.value.id}`
        : '/api/v1/evaluation-targets'
      await apiClient.request({ method: editing.value ? 'put' : 'post', url: path, data: payload })
    } else if (activeTab.value === 'scenario') {
      const path = editing.value ? `/api/v1/scenarios/${editing.value.id}` : '/api/v1/scenarios'
      await apiClient.request({
        method: editing.value ? 'put' : 'post',
        url: path,
        data: { ...payload, evaluation_target_id: form.evaluation_target_id },
      })
    } else {
      const path = editing.value ? `/api/v1/case-tags/${editing.value.id}` : '/api/v1/case-tags'
      const data = editing.value
        ? payload
        : {
            ...payload,
            scope: form.scope,
            evaluation_target_id: form.scope === 'target' ? form.evaluation_target_id : null,
          }
      await apiClient.request({ method: editing.value ? 'put' : 'post', url: path, data })
    }
    dialog.value = false
    ElMessage.success(`${activeTab.value === 'target' ? '评测对象' : activeTab.value === 'scenario' ? '评测场景' : '用例标签'}已保存`)
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function toggle(item: CatalogItem | Scenario | Tag) {
  try {
    const action = item.status === 'active' ? 'disable' : 'enable'
    const resource =
      activeTab.value === 'target'
        ? 'evaluation-targets'
        : activeTab.value === 'scenario'
          ? 'scenarios'
          : 'case-tags'
    await apiClient.post(`/api/v1/${resource}/${item.id}/${action}`, {
      expected_lock_version: item.lock_version,
    })
    ElMessage.success(action === 'disable' ? '目录项已停用' : '目录项已启用')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

onMounted(load)

watch([activeTab, tagScope, selectedTarget], ([tab]) => {
  if (tab === 'case-tag') {
    void loadCaseTags()
  }
})
</script>

<template>
  <section class="management-page">
    <el-card shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="评测对象" name="target" />
        <el-tab-pane label="评测场景" name="scenario" />
        <el-tab-pane label="用例标签" name="case-tag" />
      </el-tabs>
      <div class="catalog-toolbar">
        <div v-if="activeTab === 'scenario'" class="catalog-filter-row">
          <el-select v-model="selectedTarget" clearable filterable placeholder="全部评测对象" aria-label="筛选评测对象">
            <el-option v-for="item in targets" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </div>
        <div v-else-if="activeTab === 'case-tag'" class="catalog-filter-row">
          <el-radio-group v-model="tagScope" aria-label="用例标签作用域">
            <el-radio-button value="global">全局标签</el-radio-button>
            <el-radio-button value="target">对象专属标签</el-radio-button>
          </el-radio-group>
          <el-select
            v-if="tagScope === 'target'"
            v-model="selectedTarget"
            class="catalog-filter"
            placeholder="选择评测对象"
            aria-label="筛选评测对象"
          >
            <el-option v-for="item in targets" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </div>
        <div v-else />
        <el-button type="primary" @click="openCreate">{{ primaryActionLabel }}</el-button>
      </div>
      <el-table :data="tableData" empty-text="暂无目录数据">
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column v-if="activeTab === 'scenario'" prop="evaluation_target_name" label="评测对象" />
        <el-table-column v-if="activeTab === 'case-tag'" label="作用域" width="120">
          <template #default="{ row }">
            <el-tag :type="row.scope === 'global' ? 'primary' : 'warning'">
              {{ row.scope === 'global' ? '全局复用' : '对象专属' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          v-if="activeTab === 'case-tag' && tagScope === 'target'"
          prop="evaluation_target_name"
          label="所属评测对象"
          min-width="160"
        />
        <el-table-column prop="description" label="说明" min-width="260" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="220" align="right">
          <template #default="{ row }">
            <el-button v-if="activeTab === 'target'" link type="primary" @click="openScenarioCreate(row.id)">
              新增场景
            </el-button>
            <el-button v-if="activeTab === 'target'" link type="primary" @click="openTagCreate(row.id)">
              新增标签
            </el-button>
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link @click="toggle(row)">
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>

  <el-dialog v-model="dialog" :title="dialogTitle" width="520">
    <el-form label-position="top">
      <el-form-item v-if="activeTab === 'scenario'" label="所属评测对象" required>
        <el-select v-model="form.evaluation_target_id" filterable>
          <el-option v-for="item in targets" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </el-form-item>
      <template v-if="activeTab === 'case-tag'">
        <el-form-item label="标签作用域">
          <el-radio-group v-model="form.scope" :disabled="Boolean(editing)">
            <el-radio value="global">全局复用</el-radio>
            <el-radio value="target">指定评测对象</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.scope === 'target'" label="所属评测对象">
          <el-select v-model="form.evaluation_target_id" :disabled="Boolean(editing)">
            <el-option v-for="item in targets" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-alert
          v-else
          title="全局标签创建后将在所有评测对象中可用"
          type="info"
          show-icon
        />
      </template>
      <el-form-item :label="nameLabel" required><el-input v-model="form.name" maxlength="100" /></el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button
        type="primary"
        :disabled="!form.name.trim() || (activeTab === 'case-tag' && form.scope === 'target' && !form.evaluation_target_id)"
        @click="save"
      >
        保存
      </el-button>
    </template>
  </el-dialog>
</template>
