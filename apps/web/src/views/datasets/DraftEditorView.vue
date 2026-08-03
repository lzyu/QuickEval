<script setup lang="ts">
import { Download, Minus, Plus, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type UploadFile } from 'element-plus'
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
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
import CaseInlineForm from '@/components/datasets/CaseInlineForm.vue'
import { caseDisplayName } from '@/features/datasets/case-display'

const route = useRoute()
const router = useRouter()
const versionId = String(route.params.versionId)
const loading = ref(false)
const dataset = ref<Dataset | null>(null)
const version = ref<DatasetVersion | null>(null)
const cases = ref<VersionCase[]>([])
const scenarios = ref<Scenario[]>([])
const caseTags = ref<Tag[]>([])
const createOpen = ref(false)
const editing = ref<VersionCase | null>(null)
const dirty = ref(false)
const savingCase = ref(false)
const savingMode = ref<'close' | 'continue' | null>(null)
const createTrigger = ref<HTMLButtonElement | null>(null)
const trackFormChanges = ref(false)
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
const availableTargetTags = computed(() =>
  caseTags.value.filter(
    (tag) => tag.status === 'active' && tag.scope === 'target',
  ),
)
const targetScenarios = computed(() =>
  scenarios.value.filter(
    (item) => item.status === 'active' && item.evaluation_target_id === dataset.value?.evaluation_target_id,
  ),
)
let caseTagRequestID = 0

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
    const scenarioResponse = await apiClient.get<ResponseEnvelope<PageData<Scenario>>>(
      '/api/v1/scenarios?page_size=100',
    )
    scenarios.value = scenarioResponse.data.data.items
    await loadCaseTags()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadCaseTags() {
  const requestID = ++caseTagRequestID
  if (!dataset.value) return
  const [globalResponse, targetResponse] = await Promise.all([
    apiClient.get<ResponseEnvelope<{ items: Tag[] }>>('/api/v1/case-tags?scope=global'),
    apiClient.get<ResponseEnvelope<{ items: Tag[] }>>(
      `/api/v1/case-tags?scope=target&evaluation_target_id=${dataset.value.evaluation_target_id}`,
    ),
  ])
  if (requestID !== caseTagRequestID) return
  caseTags.value = [...globalResponse.data.data.items, ...targetResponse.data.data.items]
}

async function refreshVersionSnapshot() {
  try {
    const response = await apiClient.get<ResponseEnvelope<DatasetVersion>>(
      `/api/v1/dataset-versions/${versionId}`,
    )
    version.value = response.data.data
    if (dataset.value) dataset.value.draft_case_count = response.data.data.case_count
  } catch {
    ElMessage.warning('用例已保存，但版本状态刷新失败；发布前请刷新页面')
  }
}

function resetCaseForm(options: { preserveClassification?: boolean } = {}) {
  const scenarioID = options.preserveClassification ? form.scenario_id : ''
  const tagIDs = options.preserveClassification ? [...form.tag_ids] : []
  const isEnabled = options.preserveClassification ? form.is_enabled : true
  Object.assign(form, {
    scenario_id: scenarioID,
    name: '',
    user_prompt: '',
    precondition: '',
    expected_result: '',
    judging_guide: '',
    tag_ids: tagIDs,
    is_enabled: isEnabled,
  })
}

async function confirmDiscardCreate() {
  if (!createOpen.value || !dirty.value) return true
  try {
    await ElMessageBox.confirm('当前新增用例尚未保存，确定放弃吗？', '放弃新增', {
      type: 'warning',
      confirmButtonText: '放弃新增',
    })
    return true
  } catch {
    return false
  }
}

async function confirmDiscardEdit() {
  if (!editing.value || !dirty.value) return true
  try {
    await ElMessageBox.confirm('当前用例修改尚未保存，确定放弃吗？', '放弃修改', {
      type: 'warning',
      confirmButtonText: '放弃修改',
    })
    return true
  } catch {
    return false
  }
}

function focusInlineEditor(editorID: string) {
  const editor = document.getElementById(editorID)
  editor?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  editor?.querySelector<HTMLTextAreaElement>('textarea')?.focus()
}

async function openCreateEditor() {
  if (createOpen.value) {
    await nextTick()
    focusInlineEditor('create-case-editor')
    return
  }
  if (!(await confirmDiscardEdit())) return
  editing.value = null
  trackFormChanges.value = false
  resetCaseForm()
  dirty.value = false
  createOpen.value = true
  await nextTick()
  focusInlineEditor('create-case-editor')
  trackFormChanges.value = true
}

async function closeCreateEditor() {
  if (!(await confirmDiscardCreate())) return
  trackFormChanges.value = false
  createOpen.value = false
  dirty.value = false
  resetCaseForm()
  await nextTick()
  createTrigger.value?.focus()
}

async function openEditor(item: VersionCase) {
  if (editing.value?.id === item.id) {
    await nextTick()
    focusInlineEditor(`edit-case-editor-${item.id}`)
    return
  }
  if (!(await confirmDiscardCreate())) return
  if (!(await confirmDiscardEdit())) return
  createOpen.value = false
  trackFormChanges.value = false
  editing.value = item
  Object.assign(form, {
    scenario_id: item.scenario_id || '',
    name: item.name || '',
    user_prompt: item.user_prompt,
    precondition: item.precondition || '',
    expected_result: item.expected_result || '',
    judging_guide: item.judging_guide || '',
    tag_ids: item.tags.map((tag) => tag.id),
    is_enabled: item.is_enabled,
  })
  dirty.value = false
  await nextTick()
  focusInlineEditor(`edit-case-editor-${item.id}`)
  trackFormChanges.value = true
}

async function closeEditor() {
  if (!(await confirmDiscardEdit())) return
  const editingID = editing.value?.id
  trackFormChanges.value = false
  dirty.value = false
  editing.value = null
  resetCaseForm()
  await nextTick()
  if (editingID) document.getElementById(`edit-case-trigger-${editingID}`)?.focus()
}

function downloadTemplate() {
  window.open('/api/v1/case-import-template.csv', '_blank')
}

async function saveCase(continueCreating = false) {
  if (!form.user_prompt.trim()) {
    ElMessage.warning('请输入用户问题或任务指令')
    return
  }
  const wasEditing = Boolean(editing.value)
  const editingID = editing.value?.id
  const payload = {
    scenario_id: form.scenario_id || null,
    name: form.name.trim() || null,
    user_prompt: form.user_prompt.trim(),
    precondition: form.precondition || null,
    expected_result: form.expected_result || null,
    judging_guide: form.judging_guide || null,
    tag_ids: form.tag_ids,
    is_enabled: form.is_enabled,
    expected_lock_version: editing.value?.lock_version || 0,
  }
  savingCase.value = true
  savingMode.value = continueCreating ? 'continue' : 'close'
  try {
    let savedCase: VersionCase
    if (editing.value) {
      const response = await apiClient.patch<ResponseEnvelope<VersionCase>>(
        `/api/v1/version-cases/${editing.value.id}`,
        payload,
      )
      savedCase = response.data.data
      const index = cases.value.findIndex((item) => item.id === savedCase.id)
      if (index >= 0) cases.value[index] = savedCase
    } else {
      const response = await apiClient.post<ResponseEnvelope<VersionCase>>(
        `/api/v1/dataset-versions/${versionId}/cases`,
        payload,
      )
      savedCase = response.data.data
      cases.value.push(savedCase)
      if (version.value) {
        version.value.case_count += 1
        if (savedCase.is_enabled) version.value.enabled_count += 1
      }
      if (dataset.value) dataset.value.draft_case_count += 1
    }
    ElMessage.success(wasEditing ? '用例已保存' : '用例已添加')
    await refreshVersionSnapshot()
    if (!wasEditing && continueCreating) {
      trackFormChanges.value = false
      resetCaseForm({ preserveClassification: true })
      createOpen.value = true
      dirty.value = false
      await nextTick()
      focusInlineEditor('create-case-editor')
      trackFormChanges.value = true
      return
    }
    trackFormChanges.value = false
    dirty.value = false
    createOpen.value = false
    editing.value = null
    resetCaseForm()
    await nextTick()
    if (wasEditing && editingID) {
      document.getElementById(`edit-case-trigger-${editingID}`)?.focus()
    } else createTrigger.value?.focus()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    savingCase.value = false
    savingMode.value = null
  }
}

async function deleteCase(item: VersionCase) {
  try {
    await ElMessageBox.confirm(`确定删除“${caseDisplayName(item.name, item.user_prompt, 20)}”吗？`, '删除用例', {
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
  form,
  () => {
    if (trackFormChanges.value) dirty.value = true
  },
  { deep: true },
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
        <span :class="dirty ? 'unsaved-hint' : 'saved-hint'" role="status">
          {{ dirty ? '有未保存的用例' : '✓ 所有更改已保存' }}
        </span>
        <el-button :icon="Download" @click="downloadTemplate">
          下载模板
        </el-button>
        <el-button :icon="Upload" @click="importOpen = true">导入 CSV</el-button>
        <el-button
          type="primary"
          :disabled="enabledCount === 0 || dirty"
          :title="dirty ? '请先保存或放弃正在编辑的用例' : ''"
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
      </div>
      <el-table
        :data="cases"
        row-key="id"
        :expand-row-keys="editing ? [editing.id] : []"
        empty-text="暂无用例，可在下方添加第一条或导入 CSV"
      >
        <el-table-column type="expand" width="1">
          <template #default="{ row }">
            <section
              v-if="editing?.id === row.id"
              :id="`edit-case-editor-${row.id}`"
              class="inline-case-editor inline-case-editor--edit"
              role="region"
              :aria-labelledby="`edit-case-heading-${row.id}`"
              :aria-busy="savingCase"
            >
              <div class="inline-case-edit-heading">
                <div>
                  <strong :id="`edit-case-heading-${row.id}`">编辑评测用例</strong>
                  <span>{{ caseDisplayName(row.name, row.user_prompt) }}</span>
                </div>
                <el-button link :disabled="savingCase" @click="closeEditor">收起</el-button>
              </div>
              <CaseInlineForm
                v-model:scenario-id="form.scenario_id"
                v-model:name="form.name"
                v-model:user-prompt="form.user_prompt"
                v-model:precondition="form.precondition"
                v-model:expected-result="form.expected_result"
                v-model:judging-guide="form.judging_guide"
                v-model:tag-ids="form.tag_ids"
                v-model:is-enabled="form.is_enabled"
                mode="edit"
                :target-scenarios="targetScenarios"
                :available-global-tags="availableGlobalTags"
                :available-target-tags="availableTargetTags"
                :saving-case="savingCase"
                :saving-mode="savingMode"
                @cancel="closeEditor"
                @save="saveCase()"
              />
            </section>
          </template>
        </el-table-column>
        <el-table-column label="序号" width="72" align="center">
          <template #default="{ $index }">
            <span class="case-sequence">{{ $index + 1 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="评测用例" min-width="360">
          <template #default="{ row }">
            <div class="case-content-cell">
              <strong>{{ caseDisplayName(row.name, row.user_prompt) }}</strong>
              <span v-if="row.name">{{ row.user_prompt }}</span>
              <span v-else>未设置名称，使用用户输入作为识别文本</span>
            </div>
          </template>
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
            <el-button
              :id="`edit-case-trigger-${row.id}`"
              link
              type="primary"
              :aria-expanded="editing?.id === row.id"
              :aria-controls="`edit-case-editor-${row.id}`"
              @click="openEditor(row)"
            >
              {{ editing?.id === row.id ? '正在编辑' : '编辑' }}
            </el-button>
            <el-button link type="danger" @click="deleteCase(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="case-create-area">
        <button
          id="create-case-trigger"
          ref="createTrigger"
          type="button"
          class="case-create-trigger"
          :aria-expanded="createOpen"
          aria-controls="create-case-editor"
          @click="createOpen ? closeCreateEditor() : openCreateEditor()"
        >
          <el-icon>
            <Minus v-if="createOpen" />
            <Plus v-else />
          </el-icon>
          <span>
            <strong>{{ createOpen ? '收起新增区域' : '添加评测用例' }}</strong>
            <small>只需填写用户输入即可保存，其他信息可以稍后补充</small>
          </span>
        </button>
        <el-collapse-transition>
          <section
            v-if="createOpen"
            id="create-case-editor"
            class="inline-case-editor"
            role="region"
            aria-labelledby="create-case-trigger"
            :aria-busy="savingCase"
          >
            <CaseInlineForm
              v-model:scenario-id="form.scenario_id"
              v-model:name="form.name"
              v-model:user-prompt="form.user_prompt"
              v-model:precondition="form.precondition"
              v-model:expected-result="form.expected_result"
              v-model:judging-guide="form.judging_guide"
              v-model:tag-ids="form.tag_ids"
              v-model:is-enabled="form.is_enabled"
              mode="create"
              :target-scenarios="targetScenarios"
              :available-global-tags="availableGlobalTags"
              :available-target-tags="availableTargetTags"
              :saving-case="savingCase"
              :saving-mode="savingMode"
              @cancel="closeCreateEditor"
              @save="saveCase()"
              @save-continue="saveCase(true)"
            />
          </section>
        </el-collapse-transition>
      </div>
      <div class="danger-zone">
        <el-button link type="danger" @click="deleteDraft">删除当前草稿</el-button>
      </div>
    </el-card>
  </section>

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
