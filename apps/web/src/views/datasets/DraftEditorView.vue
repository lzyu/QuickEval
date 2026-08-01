<script setup lang="ts">
import { Download, Plus, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type UploadFile } from 'element-plus'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  Dataset,
  DatasetDetail,
  DatasetVersion,
  ImportPreview,
  PageData,
  ResponseEnvelope,
  Scenario,
  Tag,
  VersionCase,
} from '@/api/types'

const route = useRoute()
const router = useRouter()
const versionId = String(route.params.versionId)
const loading = ref(false)
const dataset = ref<Dataset | null>(null)
const version = ref<DatasetVersion | null>(null)
const cases = ref<VersionCase[]>([])
const scenarios = ref<Scenario[]>([])
const caseTags = ref<Tag[]>([])
const editorOpen = ref(false)
const editing = ref<VersionCase | null>(null)
const dirty = ref(false)
const importOpen = ref(false)
const importFile = ref<File | null>(null)
const importPreview = ref<ImportPreview | null>(null)
const importLoading = ref(false)
const publishOpen = ref(false)
const releaseNote = ref('')
const form = reactive({
  scenario_id: '',
  name: '',
  user_prompt: '',
  precondition: '',
  expected_result: '',
  judging_guide: '',
  tag_ids: [] as string[],
  is_enabled: true,
})

const enabledCount = computed(() => cases.value.filter((item) => item.is_enabled).length)
const disabledCount = computed(() => cases.value.length - enabledCount.value)
const nextVersionNo = computed(() => (dataset.value?.latest_version_no || 0) + 1)
const availableGlobalTags = computed(() =>
  caseTags.value.filter((tag) => tag.status === 'active' && tag.scope === 'global'),
)
const availableScenarioTags = computed(() =>
  caseTags.value.filter(
    (tag) => tag.status === 'active' && tag.scope === 'scenario' && tag.scenario_id === form.scenario_id,
  ),
)
const targetScenarios = computed(() =>
  scenarios.value.filter(
    (item) => item.status === 'active' && item.evaluation_target_id === dataset.value?.evaluation_target_id,
  ),
)

async function load() {
  loading.value = true
  try {
    const versionResponse = await apiClient.get<ResponseEnvelope<DatasetVersion>>(
      `/api/v1/dataset-versions/${versionId}`,
    )
    version.value = versionResponse.data.data
    if (version.value.status !== 'draft') {
      ElMessage.warning('该版本已经不是草稿')
      await router.replace(`/datasets/${version.value.dataset_id}`)
      return
    }
    const [datasetResponse, caseResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<DatasetDetail>>(
        `/api/v1/datasets/${version.value.dataset_id}`,
      ),
      apiClient.get<ResponseEnvelope<PageData<VersionCase>>>(
        `/api/v1/dataset-versions/${versionId}/cases?page_size=100`,
      ),
    ])
    dataset.value = datasetResponse.data.data.dataset
    cases.value = caseResponse.data.data.items
    const [scenarioResponse, tagResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios?page_size=100'),
      apiClient.get<ResponseEnvelope<{ items: Tag[] }>>('/api/v1/case-tags?scope=global'),
    ])
    scenarios.value = scenarioResponse.data.data.items
    caseTags.value = tagResponse.data.data.items
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadCaseTags(scenarioID: string) {
  if (!scenarioID) {
    const response = await apiClient.get<ResponseEnvelope<{ items: Tag[] }>>(
      '/api/v1/case-tags?scope=global',
    )
    caseTags.value = response.data.data.items
    return
  }
  const response = await apiClient.get<ResponseEnvelope<{ global: Tag[]; scenario: Tag[] }>>(
    `/api/v1/scenarios/${scenarioID}/available-case-tags`,
  )
  caseTags.value = [...response.data.data.global, ...response.data.data.scenario]
}

async function openEditor(item?: VersionCase) {
  editing.value = item || null
  try {
    await loadCaseTags(item?.scenario_id || '')
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
    return
  }
  Object.assign(form, {
    scenario_id: item?.scenario_id || '',
    name: item?.name || '',
    user_prompt: item?.user_prompt || '',
    precondition: item?.precondition || '',
    expected_result: item?.expected_result || '',
    judging_guide: item?.judging_guide || '',
    tag_ids: item?.tags.map((tag) => tag.id) || [],
    is_enabled: item?.is_enabled ?? true,
  })
  dirty.value = false
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
}

function confirmEditorClose(done: () => void) {
  if (!dirty.value) {
    done()
    return
  }
  void ElMessageBox.confirm('当前表单尚未保存，确定关闭吗？', '放弃修改', {
    type: 'warning',
  }).then(() => {
    dirty.value = false
    done()
  })
}

function downloadTemplate() {
  window.open('/api/v1/case-import-template.csv', '_blank')
}

async function saveCase() {
  const payload = {
    scenario_id: form.scenario_id || null,
    name: form.name || null,
    user_prompt: form.user_prompt,
    precondition: form.precondition || null,
    expected_result: form.expected_result || null,
    judging_guide: form.judging_guide || null,
    tag_ids: form.tag_ids,
    is_enabled: form.is_enabled,
    expected_lock_version: editing.value?.lock_version || 0,
  }
  try {
    if (editing.value) {
      await apiClient.patch(`/api/v1/version-cases/${editing.value.id}`, payload)
    } else {
      await apiClient.post(`/api/v1/dataset-versions/${versionId}/cases`, payload)
    }
    dirty.value = false
    editorOpen.value = false
    ElMessage.success('用例已保存')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function deleteCase(item: VersionCase) {
  try {
    await ElMessageBox.confirm(`确定删除“${item.name || item.user_prompt.slice(0, 20)}”吗？`, '删除用例', {
      type: 'warning',
    })
    await apiClient.delete(`/api/v1/version-cases/${item.id}`, {
      params: { expected_lock_version: item.lock_version },
    })
    ElMessage.success('用例已删除')
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function moveCase(index: number, offset: number) {
  const target = index + offset
  const current = cases.value[index]
  const neighbor = cases.value[target]
  if (!current || !neighbor) return
  const reordered = [...cases.value]
  reordered[index] = neighbor
  reordered[target] = current
  try {
    await apiClient.post(`/api/v1/dataset-versions/${versionId}/cases/reorder`, {
      items: reordered.map((item, order) => ({
        id: item.id,
        sort_order: (order + 1) * 10,
        expected_lock_version: item.lock_version,
      })),
    })
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

function chooseCSV(uploadFile: UploadFile) {
  importFile.value = uploadFile.raw || null
  importPreview.value = null
}

async function previewCSV() {
  if (!importFile.value) return
  importLoading.value = true
  try {
    const data = new FormData()
    data.append('file', importFile.value)
    const response = await apiClient.post<ResponseEnvelope<ImportPreview>>(
      `/api/v1/dataset-versions/${versionId}/case-imports/preview`,
      data,
    )
    importPreview.value = response.data.data
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    importLoading.value = false
  }
}

async function commitCSV() {
  if (!importPreview.value) return
  try {
    await apiClient.post(`/api/v1/dataset-versions/${versionId}/case-imports/commit`, {
      import_token: importPreview.value.import_token,
    })
    ElMessage.success(`已追加 ${importPreview.value.valid_row_count} 条用例`)
    importOpen.value = false
    importFile.value = null
    importPreview.value = null
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function publish() {
  if (!version.value) return
  try {
    await apiClient.post(`/api/v1/dataset-versions/${versionId}/publish`, {
      release_note: releaseNote.value || null,
      expected_lock_version: version.value.lock_version,
    })
    ElMessage.success(`V${nextVersionNo.value} 已发布，内容现已锁定`)
    publishOpen.value = false
    await router.replace(`/datasets/${version.value.dataset_id}`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function deleteDraft() {
  if (!version.value) return
  try {
    await ElMessageBox.confirm('删除草稿会同时删除草稿中的全部用例，且无法恢复。', '删除草稿', {
      type: 'warning',
      confirmButtonText: '确认删除',
    })
    await apiClient.delete(`/api/v1/dataset-versions/${versionId}`, {
      params: { expected_lock_version: version.value.lock_version },
    })
    ElMessage.success('草稿已删除')
    await router.replace(`/datasets/${version.value.dataset_id}`)
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('当前用例存在未保存修改，确定离开吗？')
})

watch(
  () => form.scenario_id,
  async (scenarioID) => {
    try {
      await loadCaseTags(scenarioID)
    } catch (error) {
      ElMessage.error(apiErrorMessage(error))
      return
    }
    const allowed = new Set([
      ...availableGlobalTags.value.map((tag) => tag.id),
      ...availableScenarioTags.value.map((tag) => tag.id),
    ])
    form.tag_ids = form.tag_ids.filter((tagID) => allowed.has(tagID))
  },
)

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="draft-page">
    <el-breadcrumb separator="/">
      <el-breadcrumb-item :to="{ path: '/datasets' }">评测集</el-breadcrumb-item>
      <el-breadcrumb-item :to="{ path: `/datasets/${dataset?.id}` }">
        {{ dataset?.name || '加载中' }}
      </el-breadcrumb-item>
      <el-breadcrumb-item>草稿</el-breadcrumb-item>
    </el-breadcrumb>

    <div class="draft-heading">
      <div>
        <div class="title-line">
          <h1>编辑草稿</h1>
          <el-tag type="warning">未发布</el-tag>
        </div>
        <p>{{ dataset?.evaluation_target_name }} / {{ dataset?.name }}</p>
      </div>
      <div class="heading-actions">
        <span class="saved-hint">✓ 所有更改已保存</span>
        <el-button :icon="Download" @click="downloadTemplate">
          下载模板
        </el-button>
        <el-button :icon="Upload" @click="importOpen = true">导入 CSV</el-button>
        <el-button
          type="primary"
          :disabled="enabledCount === 0"
          @click="publishOpen = true"
        >
          发布版本
        </el-button>
      </div>
    </div>

    <el-card class="case-editor-card" shadow="never">
      <div class="case-toolbar">
        <div>
          <strong>用例列表</strong>
          <span>共 {{ cases.length }} 条，{{ enabledCount }} 条启用</span>
        </div>
        <el-button type="primary" :icon="Plus" @click="openEditor()">新建用例</el-button>
      </div>
      <el-table :data="cases" empty-text="暂无用例，请新建或导入 CSV">
        <el-table-column label="排序" width="105">
          <template #default="{ $index }">
            <el-button link :disabled="$index === 0" @click="moveCase($index, -1)">↑</el-button>
            <el-button link :disabled="$index === cases.length - 1" @click="moveCase($index, 1)">↓</el-button>
          </template>
        </el-table-column>
        <el-table-column label="用例名称" min-width="180">
          <template #default="{ row }"><strong>{{ row.name || '未命名用例' }}</strong></template>
        </el-table-column>
        <el-table-column label="用户问题摘要" min-width="280" show-overflow-tooltip>
          <template #default="{ row }">{{ row.user_prompt }}</template>
        </el-table-column>
        <el-table-column label="场景归类" min-width="150">
          <template #default="{ row }">
            <el-tag v-if="row.scenario_name" type="info">{{ row.scenario_name }}</el-tag>
            <span v-else class="classification-pending">待归类</span>
          </template>
        </el-table-column>
        <el-table-column label="用例标签" min-width="170">
          <template #default="{ row }">
            <el-tag v-for="tag in row.tags" :key="tag.id" class="case-tag">{{ tag.name }}</el-tag>
            <span v-if="!row.tags.length" class="muted">无标签</span>
          </template>
        </el-table-column>
        <el-table-column label="启用状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.is_enabled ? 'success' : 'info'">
              {{ row.is_enabled ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" align="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEditor(row)">编辑</el-button>
            <el-button link type="danger" @click="deleteCase(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="danger-zone">
        <el-button link type="danger" @click="deleteDraft">删除当前草稿</el-button>
      </div>
    </el-card>
  </section>

  <el-drawer
    v-model="editorOpen"
    :before-close="confirmEditorClose"
    :title="editing ? '编辑评测用例' : '新建评测用例'"
    size="520"
  >
    <el-form label-position="top" @input="dirty = true">
      <el-form-item label="用例名称">
        <el-input v-model="form.name" maxlength="200" show-word-limit />
      </el-form-item>
      <el-form-item label="用户问题或任务指令" required>
        <el-input v-model="form.user_prompt" type="textarea" :rows="5" />
      </el-form-item>
      <el-form-item label="场景归类（可选）">
        <el-select v-model="form.scenario_id" clearable filterable placeholder="暂不归类，稍后补充">
          <el-option
            v-for="scenario in targetScenarios"
            :key="scenario.id"
            :label="scenario.name"
            :value="scenario.id"
          />
        </el-select>
        <div class="muted">场景不完整时可以留空，不影响保存、导入或发布。</div>
      </el-form-item>
      <el-collapse>
        <el-collapse-item title="补充评测信息" name="extra">
          <el-form-item label="前置条件">
            <el-input v-model="form.precondition" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item label="期望结果（可选）">
            <el-input v-model="form.expected_result" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item label="评判要点">
            <el-input v-model="form.judging_guide" type="textarea" :rows="4" />
          </el-form-item>
        </el-collapse-item>
      </el-collapse>
      <el-form-item label="用例标签">
        <el-select v-model="form.tag_ids" multiple clearable>
          <el-option-group v-if="availableGlobalTags.length" label="通用能力 · 全部场景">
            <el-option
              v-for="tag in availableGlobalTags"
              :key="tag.id"
              :label="tag.name"
              :value="tag.id"
            />
          </el-option-group>
          <el-option-group
            v-if="availableScenarioTags.length"
            :label="`场景标签 · ${targetScenarios.find((item) => item.id === form.scenario_id)?.name || ''}`"
          >
            <el-option
              v-for="tag in availableScenarioTags"
              :key="tag.id"
              :label="tag.name"
              :value="tag.id"
            />
          </el-option-group>
        </el-select>
      </el-form-item>
      <el-form-item label="启用状态">
        <el-switch v-model="form.is_enabled" active-text="启用" inactive-text="停用" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="closeEditor">取消</el-button>
      <el-button type="primary" :disabled="!form.user_prompt.trim()" @click="saveCase">保存</el-button>
    </template>
  </el-drawer>

  <el-dialog v-model="importOpen" title="批量导入评测用例" width="860">
    <el-steps :active="importPreview ? 1 : 0" align-center>
      <el-step title="上传 CSV" />
      <el-step title="校验预览" />
      <el-step title="确认追加" />
    </el-steps>
    <el-upload
      class="csv-upload"
      drag
      accept=".csv"
      :auto-upload="false"
      :limit="1"
      :on-change="chooseCSV"
    >
      <el-icon class="el-icon--upload"><Upload /></el-icon>
      <div>将 CSV 文件拖到此处，或点击选择</div>
      <template #tip>仅接受 UTF-8 CSV；导入只会追加到当前草稿。</template>
    </el-upload>
    <el-alert
      title="导入的用例将先进入“待归类”，后续可逐条补充场景。"
      type="info"
      :closable="false"
      show-icon
    />
    <div v-if="importPreview" class="import-summary">
      <el-statistic title="总行数" :value="importPreview.rows.length" />
      <el-statistic title="有效数据" :value="importPreview.valid_row_count" />
      <el-statistic title="错误数据" :value="importPreview.error_row_count" />
    </div>
    <el-table v-if="importPreview" :data="importPreview.rows" max-height="360">
      <el-table-column prop="row_number" label="CSV 行号" width="90" />
      <el-table-column prop="name" label="用例名称" width="150" />
      <el-table-column prop="user_prompt" label="用户问题" min-width="240" show-overflow-tooltip />
      <el-table-column label="校验结果" min-width="220">
        <template #default="{ row }">
          <span v-if="!row.errors.length" class="validation-success">✓ 校验通过</span>
          <span v-else class="validation-error">
            {{ row.errors.map((item: { message: string }) => item.message).join('；') }}
          </span>
        </template>
      </el-table-column>
    </el-table>
    <template #footer>
      <el-button @click="importOpen = false">取消</el-button>
      <el-button
        v-if="!importPreview"
        type="primary"
        :icon="Upload"
        :loading="importLoading"
        :disabled="!importFile"
        @click="previewCSV"
      >
        上传并校验
      </el-button>
      <template v-else>
        <el-button @click="importPreview = null">重新上传</el-button>
        <el-button
          type="primary"
          :disabled="importPreview.has_errors"
          @click="commitCSV"
        >
          确认追加 {{ importPreview.valid_row_count }} 条
        </el-button>
      </template>
    </template>
  </el-dialog>

  <el-drawer v-model="publishOpen" title="发布评测集版本" size="500">
    <div class="publish-version-flow">
      <div><span>即将发布</span><strong>V{{ nextVersionNo }}</strong></div>
      <span>→</span>
      <div><span>来源版本</span><strong>{{ version?.base_version_id ? '已发布版本' : '空白草稿' }}</strong></div>
    </div>
    <el-alert
      title="发布后用例内容将被锁定，不能直接修改"
      type="warning"
      :closable="false"
    />
    <div class="publish-stats">
      <el-statistic title="启用用例" :value="enabledCount" />
      <el-statistic title="停用用例" :value="disabledCount" />
      <el-statistic title="全部用例" :value="cases.length" />
    </div>
    <el-form label-position="top">
      <el-form-item label="发布说明">
        <el-input
          v-model="releaseNote"
          type="textarea"
          :rows="6"
          maxlength="10000"
          placeholder="描述本次版本的主要变化"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="publishOpen = false">取消</el-button>
      <el-button type="primary" :disabled="enabledCount === 0" @click="publish">
        确认发布 V{{ nextVersionNo }}
      </el-button>
    </template>
  </el-drawer>
</template>
