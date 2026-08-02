<script setup lang="ts">
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Check,
  Delete,
  UploadFilled,
  Warning,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  CaseResult,
  EvaluationProgress,
  EvaluationRun,
  EvaluationWorkbench,
  ResponseEnvelope,
  ResultStatus,
  Tag,
} from '@/api/types'
import { caseDisplayName } from '@/features/datasets/case-display'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const runId = String(route.params.runId)
const loading = ref(false)
const saving = ref(false)
const uploading = ref(false)
const deletingAttachmentId = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const issueTags = ref<Tag[]>([])
const markBadcase = ref(false)
const badcaseAutoMarked = ref(false)
const badcaseForm = reactive({
  description: '',
  issue_tag_ids: [] as string[],
})
const run = ref<EvaluationRun | null>(null)
const results = ref<CaseResult[]>([])
const selectedId = ref('')
const filter = ref<'all' | ResultStatus>('all')
const scoreFilter = ref<number | null>(null)
const badcaseOnly = ref(false)
const keyword = ref('')
const dirty = ref(false)
const skipOpen = ref(false)
const settingsOpen = ref(false)
const skipPreset = ref('')
const skipOther = ref('')
const form = reactive({
  answer_text: '',
  score: null as number | null,
})
const settingsForm = reactive({
  agent_version: '',
  environment: 'staging',
  purpose_note: '',
  config_note: '',
})

const readOnly = computed(
  () => route.name === 'evaluation-result' || run.value?.status !== 'in_progress',
)
const selected = computed(() => results.value.find((item) => item.id === selectedId.value) || null)
const filteredResults = computed(() =>
  results.value.filter((item) => {
    const statusMatches = filter.value === 'all' || item.status === filter.value
    const scoreMatches = scoreFilter.value === null || item.score === scoreFilter.value
    const badcaseMatches = !badcaseOnly.value || item.has_badcase
    const keywordMatches =
      !keyword.value ||
      (item.name || '').includes(keyword.value) ||
      item.user_prompt.includes(keyword.value)
    return statusMatches && scoreMatches && badcaseMatches && keywordMatches
  }),
)
const selectedIndex = computed(() =>
  results.value.findIndex((item) => item.id === selectedId.value),
)
const scoreLabels = ['完全错误', '大部分错误', '基本可用', '基本正确', '完全符合']
const scoreShortLabels = ['错误', '较差', '可用', '较好', '优秀']
const lowScoreRequiresBadcase = computed(
  () => !selected.value?.badcase && form.score !== null && form.score <= 2,
)
const badcaseDetailsComplete = computed(() => Boolean(badcaseForm.description.trim()))
const saveDisabled = computed(
  () =>
    form.score === null ||
    (markBadcase.value && !selected.value?.badcase && !badcaseDetailsComplete.value),
)
const saveHint = computed(() => {
  if (form.score === null) return '请选择评分后保存'
  if (markBadcase.value && !selected.value?.badcase && !badcaseDetailsComplete.value) {
    return '请说明具体问题'
  }
  return '保存后自动进入下一条待评用例'
})

function applySelected(item: CaseResult) {
  selectedId.value = item.id
  Object.assign(form, {
    answer_text: item.answer_text || '',
    score: item.score,
  })
  dirty.value = false
  const autoMark = !item.badcase && item.score !== null && item.score <= 2
  markBadcase.value = Boolean(item.badcase) || autoMark
  badcaseAutoMarked.value = autoMark
  Object.assign(badcaseForm, {
    description: item.badcase?.description || '',
    issue_tag_ids: item.badcase?.issue_tags.map((tag) => tag.id) || [],
  })
}

async function load() {
  loading.value = true
  try {
    const first = await apiClient.get<ResponseEnvelope<EvaluationWorkbench>>(
      `/api/v1/pages/evaluation-runs/${runId}/workbench`,
      { params: { page: 1, page_size: 100 } },
    )
    run.value = first.data.data.run
    const all = [...first.data.data.results.items]
    const pages = Math.ceil(first.data.data.results.total / 100)
    for (let page = 2; page <= pages; page += 1) {
      const response = await apiClient.get<ResponseEnvelope<EvaluationWorkbench>>(
        `/api/v1/pages/evaluation-runs/${runId}/workbench`,
        { params: { page, page_size: 100 } },
      )
      all.push(...response.data.data.results.items)
    }
    results.value = all
    const existing = all.find((item) => item.id === selectedId.value)
    const requested = all.find((item) => item.id === String(route.query.result_id || ''))
    const initial = requested || existing || all.find((item) => item.status === 'pending') || all[0]
    if (initial) applySelected(initial)
    if (route.name === 'evaluation-workbench' && run.value.status === 'completed') {
      await router.replace(`/evaluation-runs/${runId}/result`)
    }
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

async function choose(item: CaseResult) {
  if (item.id === selectedId.value) return
  if (dirty.value) {
    try {
      await ElMessageBox.confirm('当前题有未保存修改，确定切换吗？', '放弃修改', {
        type: 'warning',
      })
    } catch {
      return
    }
  }
  applySelected(item)
}

async function saveAndNext() {
  if (!selected.value || !run.value) return
  if (form.score === null) {
    ElMessage.warning('请选择评分后再保存')
    return
  }
  if (markBadcase.value && !selected.value.badcase && !badcaseDetailsComplete.value) {
    ElMessage.warning('请说明具体问题')
    return
  }
  saving.value = true
  try {
    const request = {
      status: 'evaluated',
      answer_text: form.answer_text || null,
      score: form.score,
      comment: selected.value.badcase
        ? selected.value.comment
        : markBadcase.value
          ? badcaseForm.description.trim()
          : null,
      skip_reason: null,
      expected_lock_version: selected.value.lock_version,
    }
    const response =
      markBadcase.value && !selected.value.badcase
        ? await apiClient.post<
            ResponseEnvelope<{
              badcase: CaseResult['badcase']
              result: CaseResult
              progress: EvaluationProgress
              run_lock_version: number
            }>
          >(
            `/api/v1/case-results/${selected.value.id}/mark-badcase`,
            {
              expected_result_lock_version: selected.value.lock_version,
              result_patch: request,
              badcase: badcaseForm,
            },
            { headers: { 'Idempotency-Key': crypto.randomUUID() } },
          )
        : await apiClient.patch<
            ResponseEnvelope<{
              result: CaseResult
              progress: EvaluationProgress
              run_lock_version: number
            }>
          >(`/api/v1/case-results/${selected.value.id}`, request)
    const index = results.value.findIndex((item) => item.id === selected.value?.id)
    results.value[index] = response.data.data.result
    run.value.progress = response.data.data.progress
    run.value.lock_version = response.data.data.run_lock_version
    dirty.value = false
    ElMessage.success('当前用例已保存')
    const next =
      results.value.slice(index + 1).find((item) => item.status === 'pending') ||
      results.value.find((item) => item.status === 'pending') ||
      results.value[index + 1]
    if (next) applySelected(next)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    saving.value = false
  }
}

async function loadIssueTags() {
  try {
    const response = await apiClient.get<ResponseEnvelope<{ items: Tag[] }>>('/api/v1/issue-tags', {
      params: { page: 1, page_size: 100, status: 'active' },
    })
    issueTags.value = response.data.data.items
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

function chooseFiles() {
  fileInput.value?.click()
}

async function uploadFiles(files: File[]) {
  if (!selected.value || readOnly.value || files.length === 0) return
  const owner = selected.value
  if (uploading.value) {
    ElMessage.info('截图正在上传，请稍候')
    return
  }
  const remaining = auth.uploadPolicy.max_files_per_owner - owner.attachments.length
  if (remaining <= 0 || files.length > remaining) {
    ElMessage.warning(`当前用例最多还能上传 ${Math.max(remaining, 0)} 张截图`)
    return
  }
  const allowed = auth.uploadPolicy.allowed_media_types
  const invalid = files.find(
    (file) => !allowed.includes(file.type) || file.size > auth.uploadPolicy.max_file_size,
  )
  if (invalid) {
    const maxSizeMB = Math.round(auth.uploadPolicy.max_file_size / 1024 / 1024)
    ElMessage.warning(`“${invalid.name}”格式不支持或超过 ${maxSizeMB} MB`)
    return
  }
  uploading.value = true
  try {
    const body = new FormData()
    files.forEach((file) => body.append('files', file))
    body.append('expected_owner_lock_version', String(owner.lock_version))
    const response = await apiClient.post<
      ResponseEnvelope<{ items: CaseResult['attachments']; owner_lock_version: number }>
    >(`/api/v1/case-results/${owner.id}/attachments`, body, {
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    })
    owner.attachments.push(...response.data.data.items)
    owner.lock_version = response.data.data.owner_lock_version
    ElMessage.success(`已添加 ${response.data.data.items.length} 张截图`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    uploading.value = false
  }
}

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  void uploadFiles(files)
}

function handlePaste(event: ClipboardEvent) {
  if (readOnly.value || !selected.value) return
  const files = Array.from(event.clipboardData?.items || [])
    .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
  if (!files.length) return
  event.preventDefault()
  void uploadFiles(files)
}

function handleDrop(event: DragEvent) {
  if (readOnly.value) return
  void uploadFiles(Array.from(event.dataTransfer?.files || []))
}

function selectScore(score: number, focus = false) {
  if (readOnly.value) return
  const shouldAutoMark = score <= 2 && !selected.value?.badcase
  if (shouldAutoMark && !markBadcase.value) {
    markBadcase.value = true
    badcaseAutoMarked.value = true
  } else if (!shouldAutoMark && badcaseAutoMarked.value) {
    markBadcase.value = false
    badcaseAutoMarked.value = false
  }
  form.score = score
  dirty.value = true
  if (focus) {
    void nextTick(() => document.getElementById(`score-${score}`)?.focus())
  }
}

function changeBadcase() {
  badcaseAutoMarked.value = false
  dirty.value = true
}

function moveScore(score: number, offset: number) {
  selectScore(Math.min(5, Math.max(1, score + offset)), true)
}

async function deleteAttachment(attachmentId: string) {
  if (!selected.value) return
  try {
    await ElMessageBox.confirm('确定删除这张截图吗？', '删除截图', { type: 'warning' })
    deletingAttachmentId.value = attachmentId
    const response = await apiClient.delete<ResponseEnvelope<{ owner_lock_version: number }>>(
      `/api/v1/attachments/${attachmentId}`,
      {
        params: { expected_owner_lock_version: selected.value.lock_version },
      },
    )
    selected.value.attachments = selected.value.attachments.filter(
      (item) => item.id !== attachmentId,
    )
    selected.value.lock_version = response.data.data.owner_lock_version
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(apiErrorMessage(error))
  } finally {
    deletingAttachmentId.value = ''
  }
}

async function reorderAttachment(index: number, offset: number) {
  if (!selected.value) return
  const target = index + offset
  if (target < 0 || target >= selected.value.attachments.length) return
  const reordered = [...selected.value.attachments]
  const currentItem = reordered[index]
  const targetItem = reordered[target]
  if (!currentItem || !targetItem) return
  reordered[index] = targetItem
  reordered[target] = currentItem
  try {
    const response = await apiClient.post<ResponseEnvelope<{ owner_lock_version: number }>>(
      `/api/v1/case-results/${selected.value.id}/attachments/reorder`,
      {
        expected_owner_lock_version: selected.value.lock_version,
        items: reordered.map((item, position) => ({ id: item.id, sort_order: position })),
      },
    )
    reordered.forEach((item, position) => {
      item.sort_order = position
    })
    selected.value.attachments = reordered
    selected.value.lock_version = response.data.data.owner_lock_version
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function skipCase() {
  if (!selected.value || !run.value) return
  const reason = skipPreset.value === 'other' ? skipOther.value.trim() : skipPreset.value
  if (!reason) return
  try {
    const response = await apiClient.patch<
      ResponseEnvelope<{
        result: CaseResult
        progress: EvaluationProgress
        run_lock_version: number
      }>
    >(`/api/v1/case-results/${selected.value.id}`, {
      status: 'skipped',
      answer_text: form.answer_text || null,
      score: null,
      comment: null,
      skip_reason: reason,
      expected_lock_version: selected.value.lock_version,
    })
    const index = results.value.findIndex((item) => item.id === selected.value?.id)
    results.value[index] = response.data.data.result
    run.value.progress = response.data.data.progress
    run.value.lock_version = response.data.data.run_lock_version
    dirty.value = false
    skipOpen.value = false
    skipPreset.value = ''
    skipOther.value = ''
    const next = results.value.find((item) => item.status === 'pending')
    if (next) applySelected(next)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function openSkip() {
  if (form.answer_text.trim() || form.score !== null) {
    try {
      await ElMessageBox.confirm('已有回答会保留，评分会被清除。确定标记为跳过吗？', '确认跳过', {
        type: 'warning',
      })
    } catch {
      return
    }
  }
  skipPreset.value = ''
  skipOther.value = ''
  skipOpen.value = true
}

async function completeRun() {
  if (!run.value) return
  if (run.value.progress.pending_count > 0) {
    ElMessage.warning(`还有 ${run.value.progress.pending_count} 条待评用例`)
    const pending = results.value.find((item) => item.status === 'pending')
    if (pending) applySelected(pending)
    return
  }
  try {
    await ElMessageBox.confirm(
      `已评 ${run.value.progress.evaluated_count} 条，跳过 ${run.value.progress.skipped_count} 条。完成后结果将锁定。`,
      '完成评测',
      { type: 'success', confirmButtonText: '确认完成' },
    )
    const response = await apiClient.post<ResponseEnvelope<EvaluationRun>>(
      `/api/v1/evaluation-runs/${runId}/complete`,
      { expected_lock_version: run.value.lock_version },
    )
    run.value = response.data.data
    ElMessage.success('评测已完成')
    await router.replace(`/evaluation-runs/${runId}/result`)
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function reopenRun() {
  if (!run.value) return
  try {
    const { value } = await ElMessageBox.prompt('请填写重开原因', '重开评测', {
      inputType: 'textarea',
      inputValidator: (text) => Boolean(text.trim()) || '重开原因不能为空',
    })
    await apiClient.post(`/api/v1/evaluation-runs/${runId}/reopen`, {
      reason: value,
      expected_lock_version: run.value.lock_version,
    })
    ElMessage.success('评测已重开')
    await router.replace(`/evaluation-runs/${runId}/workbench`)
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function voidRun() {
  if (!run.value) return
  try {
    const { value } = await ElMessageBox.prompt('作废后不能恢复，请填写原因', '作废评测', {
      type: 'warning',
      inputType: 'textarea',
      inputValidator: (text) => Boolean(text.trim()) || '作废原因不能为空',
    })
    const response = await apiClient.post<ResponseEnvelope<EvaluationRun>>(
      `/api/v1/evaluation-runs/${runId}/void`,
      { reason: value, expected_lock_version: run.value.lock_version },
    )
    run.value = response.data.data
    ElMessage.success('评测已作废')
    await router.replace(`/evaluation-runs/${runId}/result`)
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

function openSettings() {
  if (!run.value) return
  Object.assign(settingsForm, {
    agent_version: run.value.agent_version,
    environment: run.value.environment,
    purpose_note: run.value.purpose_note || '',
    config_note: run.value.config_note || '',
  })
  settingsOpen.value = true
}

async function saveSettings() {
  if (!run.value) return
  try {
    const response = await apiClient.patch<ResponseEnvelope<EvaluationRun>>(
      `/api/v1/evaluation-runs/${runId}`,
      {
        agent_version: settingsForm.agent_version,
        environment: settingsForm.environment,
        purpose_note: settingsForm.purpose_note || null,
        config_note: settingsForm.config_note || null,
        expected_lock_version: run.value.lock_version,
      },
    )
    run.value = response.data.data
    settingsOpen.value = false
    ElMessage.success('评测设置已更新')
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

function move(offset: number) {
  const target = results.value[selectedIndex.value + offset]
  if (target) choose(target)
}

function locatePending() {
  const pending = results.value.find((item) => item.status === 'pending')
  if (pending) choose(pending)
}

function statusLabel(status: ResultStatus) {
  return { pending: '待评', evaluated: '已评', skipped: '跳过' }[status]
}

function statusType(status: ResultStatus) {
  return status === 'evaluated' ? 'success' : status === 'skipped' ? 'warning' : 'info'
}

onBeforeRouteLeave(() => !dirty.value || window.confirm('当前题有未保存修改，确定离开吗？'))
onMounted(() => {
  document.addEventListener('paste', handlePaste)
  void Promise.all([load(), loadIssueTags()])
})
onBeforeUnmount(() => document.removeEventListener('paste', handlePaste))
</script>

<template>
  <section v-loading="loading" class="workbench-page">
    <template v-if="run">
      <div class="workbench-heading">
        <div>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/evaluations' }">我的评测</el-breadcrumb-item>
            <el-breadcrumb-item>{{ run.dataset_name }} V{{ run.version_no }}</el-breadcrumb-item>
            <el-breadcrumb-item>{{ readOnly ? '评测结果' : '评测工作台' }}</el-breadcrumb-item>
          </el-breadcrumb>
          <div class="title-line">
            <h1>{{ run.dataset_name }} V{{ run.version_no }}</h1>
            <el-tag
              :type="
                run.status === 'completed'
                  ? 'success'
                  : run.status === 'voided'
                    ? 'danger'
                    : 'primary'
              "
            >
              {{
                run.status === 'completed'
                  ? '已完成'
                  : run.status === 'voided'
                    ? '已作废'
                    : '进行中'
              }}
            </el-tag>
          </div>
          <p>{{ run.evaluation_target_name }} · Agent {{ run.agent_version }}</p>
        </div>
        <div class="heading-actions">
          <span v-if="!dirty" class="saved-hint">✓ 所有更改已保存</span>
          <el-button v-if="run.status === 'in_progress'" @click="openSettings">评测设置</el-button>
          <el-button v-if="run.status === 'completed'" @click="reopenRun">重开评测</el-button>
          <el-dropdown v-if="run.status !== 'voided'">
            <el-button>更多</el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item class="danger-menu-item" @click="voidRun">
                  作废评测
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button
            v-if="run.status === 'in_progress'"
            type="primary"
            :disabled="run.progress.pending_count > 0"
            @click="completeRun"
          >
            完成评测
            <template v-if="run.progress.pending_count">
              （还有 {{ run.progress.pending_count }} 条待评）
            </template>
          </el-button>
          <el-button
            v-if="run.status === 'in_progress' && run.progress.pending_count > 0"
            @click="locatePending"
          >
            定位第一条待评
          </el-button>
        </div>
      </div>

      <div class="workbench-progress">
        <div class="workbench-progress-metrics">
          <span>总用例 <strong>{{ run.progress.total_count }}</strong></span>
          <span>待评 <strong>{{ run.progress.pending_count }}</strong></span>
          <span>已评 <strong>{{ run.progress.evaluated_count }}</strong></span>
          <span>跳过 <strong>{{ run.progress.skipped_count }}</strong></span>
          <span>已评分 <strong>{{ run.progress.scored_count }}</strong></span>
          <span v-if="readOnly">
            平均分 <strong>{{ run.progress.average_score?.toFixed(1) || '-' }}</strong>
          </span>
        </div>
        <div class="workbench-completion">
          <span>完成 <strong>{{ Math.round(run.progress.completion_rate * 100) }}%</strong></span>
          <el-progress
            :percentage="Math.round(run.progress.completion_rate * 100)"
            :show-text="false"
          />
        </div>
      </div>

      <div class="workbench-grid">
        <aside class="result-directory">
          <h2>评测用例</h2>
          <el-input v-model="keyword" placeholder="搜索用例" clearable />
          <el-segmented
            v-model="filter"
            :options="[
              { label: '全部', value: 'all' },
              { label: '待评', value: 'pending' },
              { label: '已评', value: 'evaluated' },
              { label: '跳过', value: 'skipped' },
            ]"
          />
          <div v-if="readOnly" class="result-readonly-filters">
            <el-select v-model="scoreFilter" clearable placeholder="全部评分">
              <el-option v-for="score in 5" :key="score" :label="`${score} 分`" :value="score" />
            </el-select>
            <el-checkbox v-model="badcaseOnly">仅 Badcase</el-checkbox>
          </div>
          <div class="result-directory-list">
            <button
              v-for="item in filteredResults"
              :key="item.id"
              :class="{ active: item.id === selectedId }"
              @click="choose(item)"
            >
              <span>{{ String(results.indexOf(item) + 1).padStart(2, '0') }}</span>
              <strong>{{ caseDisplayName(item.name, item.user_prompt, 18) }}</strong>
              <el-tag :type="statusType(item.status)" size="small">
                {{
                  statusLabel(item.status)
                }}
              </el-tag>
              <em v-if="item.score">{{ item.score }}</em>
            </button>
          </div>
          <footer>共 {{ results.length }} 条</footer>
        </aside>

        <div v-if="selected" class="workbench-main">
          <section class="case-snapshot" aria-label="当前用例内容">
            <section class="case-brief">
              <div class="case-brief-prompt">
                <span>用户输入</span>
                <div class="case-brief-prompt-content">
                  <p>{{ selected.user_prompt }}</p>
                  <div class="case-brief-tags">
                    <el-tag :type="selected.scenario_name ? 'info' : 'warning'" size="small">
                      {{ selected.scenario_name || '待归类' }}
                    </el-tag>
                    <el-tag v-for="tag in selected.tags" :key="tag.id" size="small">
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </div>
              </div>
              <dl
                v-if="selected.precondition || selected.expected_result || selected.judging_guide"
                class="case-brief-details"
              >
                <div v-if="selected.judging_guide">
                  <dt>评判要点</dt>
                  <dd>{{ selected.judging_guide }}</dd>
                </div>
                <div v-if="selected.expected_result">
                  <dt>期望结果</dt>
                  <dd>{{ selected.expected_result }}</dd>
                </div>
                <div v-if="selected.precondition">
                  <dt>前置条件</dt>
                  <dd>{{ selected.precondition }}</dd>
                </div>
              </dl>
              <footer class="case-brief-navigation">
                <el-button :icon="ArrowLeft" :disabled="selectedIndex <= 0" @click="move(-1)">
                  上一条
                </el-button>
                <span>{{ selectedIndex + 1 }} / {{ results.length }}</span>
                <el-button :disabled="selectedIndex >= results.length - 1" @click="move(1)">
                  下一条<el-icon><ArrowRight /></el-icon>
                </el-button>
              </footer>
            </section>
          </section>

          <aside class="result-editor">
            <template v-if="selected.status === 'skipped' && readOnly">
              <el-alert
                :title="`已跳过：${selected.skip_reason}`"
                type="warning"
                :closable="false"
              />
            </template>
            <el-form class="result-entry-form" label-position="top" @input="dirty = true">
              <section class="quick-score-row" aria-labelledby="quick-score-title">
                <div class="quick-score-copy">
                  <h2 id="quick-score-title">
                    快速评分 <span v-if="!readOnly" aria-hidden="true">*</span>
                  </h2>
                  <p v-if="!readOnly">选择评分后即可保存</p>
                  <p v-else>本条用例的评分结果</p>
                </div>
                <div class="score-segments" role="radiogroup" aria-label="评分，1 分最低，5 分最高">
                  <el-tooltip
                    v-for="score in 5"
                    :key="score"
                    :content="`${score} 分：${scoreLabels[score - 1]}`"
                  >
                    <button
                      :id="`score-${score}`"
                      type="button"
                      role="radio"
                      :aria-label="`${score} 分，${scoreLabels[score - 1]}`"
                      :aria-checked="form.score === score"
                      :tabindex="
                        form.score === score || (form.score === null && score === 1) ? 0 : -1
                      "
                      :class="{ selected: form.score === score }"
                      :disabled="readOnly"
                      @click="selectScore(score)"
                      @keydown.left.prevent="moveScore(score, -1)"
                      @keydown.right.prevent="moveScore(score, 1)"
                      @keydown.down.prevent="moveScore(score, 1)"
                      @keydown.up.prevent="moveScore(score, -1)"
                    >
                      <strong>{{ score }}</strong>
                      <span>{{ scoreShortLabels[score - 1] }}</span>
                    </button>
                  </el-tooltip>
                </div>
                <small v-if="lowScoreRequiresBadcase" class="score-rule-note">
                  1～2 分视为 Badcase，已自动展开问题信息。
                </small>
              </section>

              <section class="supporting-details" aria-label="补充证据（可选）">
                <el-form-item class="evidence-form-item" label="现场截图（可选）">
                  <input
                    ref="fileInput"
                    class="visually-hidden"
                    type="file"
                    tabindex="-1"
                    aria-hidden="true"
                    accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
                    multiple
                    @change="selectFiles"
                  />
                  <div v-if="selected.attachments.length" class="evidence-grid">
                    <div
                      v-for="(attachment, index) in selected.attachments"
                      :key="attachment.id"
                      class="evidence-card"
                    >
                      <el-image
                        :src="attachment.content_url"
                        :preview-src-list="selected.attachments.map((item) => item.content_url)"
                        :initial-index="index"
                        fit="cover"
                        preview-teleported
                      />
                      <span :title="attachment.original_name">{{ attachment.original_name }}</span>
                      <div v-if="!readOnly" class="evidence-actions">
                        <el-button
                          text
                          :icon="ArrowUp"
                          :disabled="index === 0"
                          aria-label="前移截图"
                          @click="reorderAttachment(index, -1)"
                        />
                        <el-button
                          text
                          :icon="ArrowDown"
                          :disabled="index === selected.attachments.length - 1"
                          aria-label="后移截图"
                          @click="reorderAttachment(index, 1)"
                        />
                        <el-button
                          text
                          type="danger"
                          :icon="Delete"
                          :loading="deletingAttachmentId === attachment.id"
                          aria-label="删除截图"
                          @click="deleteAttachment(attachment.id)"
                        />
                      </div>
                    </div>
                  </div>
                  <div
                    v-if="!readOnly"
                    class="result-upload-zone"
                    :class="{
                      uploading,
                      disabled: selected.attachments.length >= auth.uploadPolicy.max_files_per_owner,
                    }"
                    role="button"
                    :tabindex="
                      selected.attachments.length >= auth.uploadPolicy.max_files_per_owner ? -1 : 0
                    "
                    :aria-disabled="
                      selected.attachments.length >= auth.uploadPolicy.max_files_per_owner
                    "
                    @click="
                      selected.attachments.length < auth.uploadPolicy.max_files_per_owner &&
                        chooseFiles()
                    "
                    @keydown.enter="
                      selected.attachments.length < auth.uploadPolicy.max_files_per_owner &&
                        chooseFiles()
                    "
                    @keydown.space.prevent="
                      selected.attachments.length < auth.uploadPolicy.max_files_per_owner &&
                        chooseFiles()
                    "
                    @dragover.prevent
                    @drop.prevent="handleDrop"
                  >
                    <el-icon :size="20"><UploadFilled /></el-icon>
                    <div>
                      <strong>{{ uploading ? '正在上传截图…' : '粘贴、拖拽或选择截图' }}</strong>
                      <span>截图后直接粘贴 · PNG/JPG/WebP · 单张 10 MB</span>
                    </div>
                    <small>{{ selected.attachments.length }} /
                      {{ auth.uploadPolicy.max_files_per_owner }}</small>
                  </div>
                </el-form-item>

                <el-form-item class="agent-answer-field" label="Agent 回答（可选）">
                  <el-input
                    v-model="form.answer_text"
                    type="textarea"
                    :rows="2"
                    maxlength="20000"
                    show-word-limit
                    resize="none"
                    :readonly="readOnly"
                    aria-label="Agent 回答（可选）"
                    placeholder="粘贴或输入需要保留的 Agent 回答"
                  />
                </el-form-item>
              </section>

              <section
                v-if="selected.badcase || !readOnly"
                class="badcase-panel"
                :class="{ active: markBadcase || selected.badcase }"
              >
                <template v-if="selected.badcase">
                  <div class="badcase-panel-heading">
                    <strong>已标记 Badcase</strong>
                    <el-button
                      type="danger"
                      link
                      @click="router.push(`/badcases/${selected.badcase?.id}`)"
                    >
                      查看详情
                    </el-button>
                  </div>
                  <p>{{ selected.badcase.title }}</p>
                  <p v-if="selected.badcase.description" class="badcase-description">
                    {{ selected.badcase.description }}
                  </p>
                  <div>
                    <el-tag
                      v-for="tag in selected.badcase.issue_tags"
                      :key="tag.id"
                      type="danger"
                      effect="plain"
                    >
                      {{ tag.name }}
                    </el-tag>
                  </div>
                </template>
                <template v-else-if="!readOnly">
                  <div class="badcase-decision">
                    <div>
                      <strong>标记 Badcase</strong>
                      <span v-if="lowScoreRequiresBadcase">当前为 1～2 分，已自动标记为 Badcase。</span>
                      <span v-else>需要跟进时开启并补充问题信息。</span>
                    </div>
                    <el-switch
                      v-model="markBadcase"
                      :active-text="lowScoreRequiresBadcase ? '低分自动标记' : ''"
                      :disabled="lowScoreRequiresBadcase"
                      aria-label="标记 Badcase"
                      @change="changeBadcase"
                    />
                  </div>
                  <el-collapse-transition>
                    <div v-if="markBadcase" class="badcase-fields">
                      <el-form-item class="badcase-description-field" label="具体问题" required>
                        <el-input
                          v-model="badcaseForm.description"
                          type="textarea"
                          :rows="3"
                          maxlength="5000"
                          show-word-limit
                          placeholder="说明问题表现、判断依据或定位线索"
                          @input="dirty = true"
                        />
                      </el-form-item>
                      <el-form-item label="问题标签（可选）">
                        <el-select
                          v-model="badcaseForm.issue_tag_ids"
                          multiple
                          clearable
                          placeholder="用于统计，可稍后补充"
                          @change="dirty = true"
                        >
                          <el-option
                            v-for="tag in issueTags"
                            :key="tag.id"
                            :label="tag.name"
                            :value="tag.id"
                          />
                        </el-select>
                      </el-form-item>
                    </div>
                  </el-collapse-transition>
                </template>
              </section>
            </el-form>
            <div v-if="!readOnly" class="result-editor-actions">
              <el-button :icon="Warning" @click="openSkip">跳过此用例</el-button>
              <div>
                <span>{{ saveHint }}</span>
                <el-button
                  type="primary"
                  :icon="Check"
                  :loading="saving"
                  :disabled="saveDisabled"
                  @click="saveAndNext"
                >
                  保存并下一条
                </el-button>
              </div>
            </div>
            <el-descriptions v-else :column="1" border>
              <el-descriptions-item label="结果状态">
                {{
                  statusLabel(selected.status)
                }}
              </el-descriptions-item>
              <el-descriptions-item label="跳过原因">
                {{
                  selected.skip_reason || '-'
                }}
              </el-descriptions-item>
            </el-descriptions>
          </aside>
        </div>
      </div>
    </template>
  </section>

  <el-dialog v-model="skipOpen" title="跳过此用例" width="500">
    <el-form label-position="top">
      <el-form-item label="跳过原因" required>
        <el-radio-group v-model="skipPreset" class="skip-reasons">
          <el-radio value="测试环境不可用">测试环境不可用</el-radio>
          <el-radio value="当前账号无权限">当前账号无权限</el-radio>
          <el-radio value="用例不适用于当前版本">用例不适用于当前版本</el-radio>
          <el-radio value="other">其他原因</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="skipPreset === 'other'" label="其他原因">
        <el-input v-model="skipOther" type="textarea" :rows="4" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="skipOpen = false">取消</el-button>
      <el-button
        type="warning"
        :disabled="!skipPreset || (skipPreset === 'other' && !skipOther.trim())"
        @click="skipCase"
      >
        确认跳过
      </el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="settingsOpen" title="评测设置" width="520">
    <el-form label-position="top">
      <el-form-item label="Agent 版本" required>
        <el-input v-model="settingsForm.agent_version" maxlength="100" />
      </el-form-item>
      <el-form-item label="运行环境" required>
        <el-select v-model="settingsForm.environment">
          <el-option label="测试" value="test" />
          <el-option label="预发布" value="staging" />
          <el-option label="生产" value="production" />
          <el-option label="其他" value="other" />
        </el-select>
      </el-form-item>
      <el-form-item label="评测说明">
        <el-input v-model="settingsForm.purpose_note" type="textarea" :rows="3" />
      </el-form-item>
      <el-form-item label="配置备注">
        <el-input v-model="settingsForm.config_note" type="textarea" :rows="3" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="settingsOpen = false">取消</el-button>
      <el-button
        type="primary"
        :disabled="!settingsForm.agent_version.trim()"
        @click="saveSettings"
      >
        保存
      </el-button>
    </template>
  </el-dialog>
</template>
