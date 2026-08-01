<script setup lang="ts">
import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp, Check, Delete, Plus, Warning } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
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
const badcaseForm = reactive({
  title: '',
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
  comment: '',
})
const settingsForm = reactive({
  agent_version: '',
  environment: 'staging',
  purpose_note: '',
  config_note: '',
})

const readOnly = computed(() => route.name === 'evaluation-result' || run.value?.status !== 'in_progress')
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
const selectedIndex = computed(() => results.value.findIndex((item) => item.id === selectedId.value))
const scoreLabels = ['完全错误', '大部分错误', '基本可用', '基本正确', '完全符合']

function applySelected(item: CaseResult) {
  selectedId.value = item.id
  Object.assign(form, {
    answer_text: item.answer_text || '',
    score: item.score,
    comment: item.comment || '',
  })
  dirty.value = false
  markBadcase.value = Boolean(item.badcase)
  Object.assign(badcaseForm, {
    title: item.badcase?.title || '',
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
  saving.value = true
  try {
    const request = {
      status: 'evaluated',
      answer_text: form.answer_text || null,
      score: form.score,
      comment: form.comment || null,
      skip_reason: null,
      expected_lock_version: selected.value.lock_version,
    }
    const response = markBadcase.value && !selected.value.badcase
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

async function uploadFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!selected.value || files.length === 0) return
  const remaining = 10 - selected.value.attachments.length
  if (files.length > remaining) {
    ElMessage.warning(`当前用例最多还能上传 ${remaining} 张截图`)
    return
  }
  const allowed = auth.uploadPolicy.allowed_media_types
  const invalid = files.find(
    (file) => !allowed.includes(file.type) || file.size > auth.uploadPolicy.max_file_size,
  )
  if (invalid) {
    ElMessage.warning(`文件 ${invalid.name} 的格式不支持或超过 10 MB`)
    return
  }
  uploading.value = true
  try {
    const body = new FormData()
    files.forEach((file) => body.append('files', file))
    body.append('expected_owner_lock_version', String(selected.value.lock_version))
    const response = await apiClient.post<
      ResponseEnvelope<{ items: CaseResult['attachments']; owner_lock_version: number }>
    >(`/api/v1/case-results/${selected.value.id}/attachments`, body, {
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    })
    selected.value.attachments.push(...response.data.data.items)
    selected.value.lock_version = response.data.data.owner_lock_version
    ElMessage.success(`已上传 ${response.data.data.items.length} 张截图`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    uploading.value = false
  }
}

async function deleteAttachment(attachmentId: string) {
  if (!selected.value) return
  try {
    await ElMessageBox.confirm('确定删除这张截图吗？', '删除截图', { type: 'warning' })
    deletingAttachmentId.value = attachmentId
    const response = await apiClient.delete<
      ResponseEnvelope<{ owner_lock_version: number }>
    >(`/api/v1/attachments/${attachmentId}`, {
      params: { expected_owner_lock_version: selected.value.lock_version },
    })
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
    const response = await apiClient.post<
      ResponseEnvelope<{ owner_lock_version: number }>
    >(`/api/v1/case-results/${selected.value.id}/attachments/reorder`, {
      expected_owner_lock_version: selected.value.lock_version,
      items: reordered.map((item, position) => ({ id: item.id, sort_order: position })),
    })
    reordered.forEach((item, position) => { item.sort_order = position })
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
      comment: form.comment || null,
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
  if (form.answer_text.trim() || form.comment.trim() || form.score !== null) {
    try {
      await ElMessageBox.confirm(
        '已有回答和评语会保留，评分会被清除。确定标记为跳过吗？',
        '确认跳过',
        { type: 'warning' },
      )
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
onMounted(() => Promise.all([load(), loadIssueTags()]))
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
            <el-tag :type="run.status === 'completed' ? 'success' : run.status === 'voided' ? 'danger' : 'primary'">
              {{ run.status === 'completed' ? '已完成' : run.status === 'voided' ? '已作废' : '进行中' }}
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
            <template v-if="run.progress.pending_count">（还有 {{ run.progress.pending_count }} 条待评）</template>
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
        <div><span>总用例</span><strong>{{ run.progress.total_count }}</strong></div>
        <div><span>待评</span><strong>{{ run.progress.pending_count }}</strong></div>
        <div><span>已评</span><strong>{{ run.progress.evaluated_count }}</strong></div>
        <div><span>跳过</span><strong>{{ run.progress.skipped_count }}</strong></div>
        <div><span>已评分</span><strong>{{ run.progress.scored_count }}</strong></div>
        <div v-if="readOnly"><span>平均分</span><strong>{{ run.progress.average_score?.toFixed(1) || '-' }}</strong></div>
        <el-progress :percentage="Math.round(run.progress.completion_rate * 100)" />
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
              <el-tag :type="statusType(item.status)" size="small">{{ statusLabel(item.status) }}</el-tag>
              <em v-if="item.score">{{ item.score }}</em>
            </button>
          </div>
          <footer>共 {{ results.length }} 条</footer>
        </aside>

        <main v-if="selected" class="case-snapshot">
          <div class="case-snapshot-heading">
            <h2>{{ caseDisplayName(selected.name, selected.user_prompt) }}</h2>
            <div>
              <el-tag :type="selected.scenario_name ? 'info' : 'warning'">
                {{ selected.scenario_name || '待归类' }}
              </el-tag>
              <el-tag v-for="tag in selected.tags" :key="tag.id">{{ tag.name }}</el-tag>
            </div>
          </div>
          <article>
            <h3>用户问题或任务指令</h3>
            <p>{{ selected.user_prompt }}</p>
          </article>
          <article v-if="selected.precondition">
            <h3>前置条件</h3>
            <p>{{ selected.precondition }}</p>
          </article>
          <article v-if="selected.expected_result">
            <h3>期望结果</h3>
            <p>{{ selected.expected_result }}</p>
          </article>
          <article v-if="selected.judging_guide">
            <h3>评判要点</h3>
            <p>{{ selected.judging_guide }}</p>
          </article>
          <footer>
            <el-button :icon="ArrowLeft" :disabled="selectedIndex <= 0" @click="move(-1)">上一条</el-button>
            <span>{{ selectedIndex + 1 }} / {{ results.length }}</span>
            <el-button :disabled="selectedIndex >= results.length - 1" @click="move(1)">
              下一条<el-icon><ArrowRight /></el-icon>
            </el-button>
          </footer>
        </main>

        <aside v-if="selected" class="result-editor">
          <h2>评测结果</h2>
          <template v-if="selected.status === 'skipped' && readOnly">
            <el-alert
              :title="`已跳过：${selected.skip_reason}`"
              type="warning"
              :closable="false"
            />
          </template>
          <el-form label-position="top" @input="dirty = true">
            <el-form-item label="Agent 回答">
              <el-input
                v-model="form.answer_text"
                type="textarea"
                :rows="9"
                maxlength="20000"
                show-word-limit
                :readonly="readOnly"
                placeholder="粘贴或输入 Agent 的完整回答"
              />
              <small class="form-help">回答文本与截图至少提供一种，便于后续分析定位。</small>
            </el-form-item>
            <el-form-item label="交互截图">
              <input
                ref="fileInput"
                class="visually-hidden"
                type="file"
                accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
                multiple
                @change="uploadFiles"
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
              <el-button
                v-if="!readOnly"
                :icon="Plus"
                :loading="uploading"
                :disabled="selected.attachments.length >= auth.uploadPolicy.max_files_per_owner"
                @click="chooseFiles"
              >
                上传截图（{{ selected.attachments.length }}/{{ auth.uploadPolicy.max_files_per_owner }}）
              </el-button>
              <small class="form-help">支持 PNG、JPG、WebP，单张不超过 10 MB。</small>
            </el-form-item>
            <el-form-item label="评分（可选）">
              <div class="score-picker">
                <el-tooltip
                  v-for="score in 5"
                  :key="score"
                  :content="`${score} 分：${scoreLabels[score - 1]}`"
                >
                  <button
                    type="button"
                    :class="{ selected: form.score === score }"
                    :disabled="readOnly"
                    @click="form.score = form.score === score ? null : score; dirty = true"
                  >
                    <strong>{{ score }}</strong>
                    <span>{{ scoreLabels[score - 1] }}</span>
                  </button>
                </el-tooltip>
              </div>
            </el-form-item>
            <el-form-item label="评语">
              <el-input
                v-model="form.comment"
                type="textarea"
                :rows="5"
                maxlength="5000"
                show-word-limit
                :readonly="readOnly"
                placeholder="记录判断依据或需要定位的问题"
              />
            </el-form-item>
            <section class="badcase-panel">
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
                <el-checkbox v-model="markBadcase" @change="dirty = true">
                  标记为 Badcase
                </el-checkbox>
                <div v-if="markBadcase" class="badcase-fields">
                  <el-input
                    v-model="badcaseForm.title"
                    maxlength="200"
                    show-word-limit
                    placeholder="问题标题"
                    @input="dirty = true"
                  />
                  <el-input
                    v-model="badcaseForm.description"
                    type="textarea"
                    :rows="3"
                    maxlength="5000"
                    show-word-limit
                    placeholder="描述具体问题与复现表现"
                    @input="dirty = true"
                  />
                  <el-select
                    v-model="badcaseForm.issue_tag_ids"
                    multiple
                    placeholder="至少选择一个问题标签"
                    @change="dirty = true"
                  >
                    <el-option
                      v-for="tag in issueTags"
                      :key="tag.id"
                      :label="tag.name"
                      :value="tag.id"
                    />
                  </el-select>
                  <small class="form-help">标记 Badcase 时评语必填。</small>
                </div>
              </template>
            </section>
          </el-form>
          <div v-if="!readOnly" class="result-editor-actions">
            <el-button :icon="Warning" @click="openSkip">跳过此用例</el-button>
            <el-button
              type="primary"
              :icon="Check"
              :loading="saving"
              :disabled="
                (!form.answer_text.trim() && selected.attachments.length === 0) ||
                  (markBadcase &&
                    !selected.badcase &&
                    (!form.comment.trim() ||
                      !badcaseForm.title.trim() ||
                      !badcaseForm.description.trim() ||
                      badcaseForm.issue_tag_ids.length === 0))
              "
              @click="saveAndNext"
            >
              保存并下一条
            </el-button>
          </div>
          <el-descriptions v-else :column="1" border>
            <el-descriptions-item label="结果状态">{{ statusLabel(selected.status) }}</el-descriptions-item>
            <el-descriptions-item label="跳过原因">{{ selected.skip_reason || '-' }}</el-descriptions-item>
          </el-descriptions>
        </aside>
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
