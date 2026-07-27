<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  CatalogItem,
  PageData,
  ResponseEnvelope,
  Scenario,
  Tag,
} from '@/api/types'

type Kind = 'target' | 'scenario' | 'case-tag'

const activeTab = ref<Kind>('target')
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const caseTags = ref<Tag[]>([])
const selectedScenario = ref('')
const dialog = ref(false)
const editing = ref<CatalogItem | Scenario | Tag | null>(null)
const form = reactive({ name: '', description: '', evaluation_target_id: '' })

const tableData = computed(() => {
  if (activeTab.value === 'target') return targets.value
  if (activeTab.value === 'scenario') return scenarios.value
  return caseTags.value
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
  const firstScenario = scenarios.value[0]
  if (!selectedScenario.value && firstScenario) {
    selectedScenario.value = firstScenario.id
  }
  await loadCaseTags()
}

async function loadCaseTags() {
  if (!selectedScenario.value) {
    caseTags.value = []
    return
  }
  const response = await apiClient.get<ResponseEnvelope<{ items: Tag[] }>>(
    `/api/v1/scenarios/${selectedScenario.value}/case-tags`,
  )
  caseTags.value = response.data.data.items
}

function openCreate() {
  editing.value = null
  Object.assign(form, {
    name: '',
    description: '',
    evaluation_target_id: targets.value[0]?.id || '',
  })
  dialog.value = true
}

function openEdit(item: CatalogItem | Scenario | Tag) {
  editing.value = item
  Object.assign(form, {
    name: item.name,
    description: item.description || '',
    evaluation_target_id:
      'evaluation_target_id' in item ? item.evaluation_target_id : targets.value[0]?.id || '',
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
      const path = editing.value
        ? `/api/v1/case-tags/${editing.value.id}`
        : `/api/v1/scenarios/${selectedScenario.value}/case-tags`
      await apiClient.request({ method: editing.value ? 'put' : 'post', url: path, data: payload })
    }
    dialog.value = false
    ElMessage.success('目录项已保存')
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
</script>

<template>
  <section class="management-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">CATALOG</p>
        <h1>基础目录</h1>
        <p>维护评测对象、场景及场景内用例标签。</p>
      </div>
      <el-button type="primary" @click="openCreate">新建目录项</el-button>
    </div>
    <el-card shadow="never">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="评测对象" name="target" />
        <el-tab-pane label="评测场景" name="scenario" />
        <el-tab-pane label="用例标签" name="case-tag" />
      </el-tabs>
      <el-select
        v-if="activeTab === 'case-tag'"
        v-model="selectedScenario"
        class="catalog-filter"
        placeholder="选择场景"
        @change="loadCaseTags"
      >
        <el-option v-for="item in scenarios" :key="item.id" :label="item.name" :value="item.id" />
      </el-select>
      <el-table :data="tableData" empty-text="暂无目录数据">
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column v-if="activeTab === 'scenario'" prop="evaluation_target_name" label="评测对象" />
        <el-table-column prop="description" label="说明" min-width="260" show-overflow-tooltip />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link @click="toggle(row)">
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>

  <el-dialog v-model="dialog" :title="editing ? '编辑目录项' : '新建目录项'" width="520">
    <el-form label-position="top">
      <el-form-item v-if="activeTab === 'scenario'" label="评测对象">
        <el-select v-model="form.evaluation_target_id">
          <el-option v-for="item in targets" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>
