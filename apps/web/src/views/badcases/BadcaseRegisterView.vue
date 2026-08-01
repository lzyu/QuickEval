<script setup lang="ts">
import {
  ArrowDown,
  ArrowUp,
  Delete,
  Rank,
  RefreshRight,
  Switch,
  UploadFilled,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  Badcase,
  CatalogItem,
  PageData,
  ResponseEnvelope,
  Scenario,
  Tag,
} from '@/api/types'
import EvaluationTargetDialog from '@/components/badcases/EvaluationTargetDialog.vue'
import {
  availableTags,
  draftKey,
  emptyRegistrationForm,
  isRegistrationValid,
  parseDraft,
  resetAfterRegistration,
  validateRegistration,
  type RegistrationDraft,
} from '@/features/badcases/registration'
import { useAuthStore } from '@/stores/auth'

interface PendingScreenshot {
  id: string
  file: File
  previewUrl: string
}

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const loading = ref(true)
const submitting = ref(false)
const targetPickerOpen = ref(false)
const pickerRequired = ref(false)
const moreLocationOpen = ref(true)
const fileInput = ref<HTMLInputElement | null>(null)
const pendingScreenshots = ref<PendingScreenshot[]>([])
const recentItems = ref<Badcase[]>([])
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const issueTags = ref<Tag[]>([])
const currentTargetId = ref('')
const currentTarget = computed(() => targets.value.find((item) => item.id === currentTargetId.value))
const targetScenarios = computed(() =>
  scenarios.value.filter(
    (item) => item.status === 'active' && item.evaluation_target_id === currentTargetId.value,
  ),
)
const filteredIssueTags = computed(() => availableTags(issueTags.value, form.scenario_id))
const form = reactive(emptyRegistrationForm())
const errors = reactive({
  title: '',
  scenario_id: '',
  issue_tag_ids: '',
  environment: '',
  occurred_at: '',
})
const createdItem = ref<Badcase | null>(null)
const uploadFailed = ref(false)
const createIdempotencyKey = ref(crypto.randomUUID())
const uploadIdempotencyKey = ref(crypto.randomUUID())
const draftReady = ref(false)
const draftStatus = ref<'idle' | 'saved'>('idle')
let draftTimer: ReturnType<typeof setTimeout> | null = null
let draggedIndex = -1

const recordLocked = computed(() => Boolean(createdItem.value))
const hasProblemContent = computed(
  () =>
    Boolean(
      form.title.trim() ||
        form.description.trim() ||
        form.agent_response_text.trim() ||
        form.business_reference.trim() ||
        form.session_id.trim() ||
        form.issue_tag_ids.length ||
        pendingScreenshots.value.length,
    ),
)

function currentDraftKey() {
  return auth.user?.id && currentTargetId.value
    ? draftKey(auth.user.id, currentTargetId.value)
    : ''
}

function saveDraft() {
  const key = currentDraftKey()
  if (!key || !draftReady.value || recordLocked.value) return
  if (!hasProblemContent.value) {
    localStorage.removeItem(key)
    draftStatus.value = 'idle'
    return
  }
  const draft: RegistrationDraft = {
    version: 1,
    form: { ...form, issue_tag_ids: [...form.issue_tag_ids] },
    had_screenshots: pendingScreenshots.value.length > 0,
    saved_at: new Date().toISOString(),
  }
  localStorage.setItem(key, JSON.stringify(draft))
  draftStatus.value = 'saved'
}

function scheduleDraftSave() {
  if (draftTimer) clearTimeout(draftTimer)
  draftStatus.value = 'idle'
  draftTimer = setTimeout(saveDraft, 450)
}

function restoreDraft() {
  draftReady.value = false
  Object.assign(form, emptyRegistrationForm())
  const key = currentDraftKey()
  const draft = key ? parseDraft(localStorage.getItem(key)) : null
  if (draft) {
    Object.assign(form, draft.form)
    if (!targetScenarios.value.some((item) => item.id === form.scenario_id)) {
      form.scenario_id = targetScenarios.value[0]?.id || ''
      form.issue_tag_ids = []
    }
    if (draft.had_screenshots) {
      ElMessage.info('已恢复文字草稿；为保护本地文件隐私，截图需要重新选择。')
    } else {
      ElMessage.success('已恢复该评测对象的登记草稿')
    }
    draftStatus.value = 'saved'
  } else {
    form.scenario_id = targetScenarios.value[0]?.id || ''
    draftStatus.value = 'idle'
  }
  nextTick(() => {
    draftReady.value = true
  })
}

async function loadPage(targetId: string) {
  loading.value = true
  try {
    const [targetResponse, scenarioResponse, optionResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>('/api/v1/evaluation-targets', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<ResponseEnvelope<{ issue_tags: Tag[] }>>('/api/v1/badcase-options'),
    ])
    targets.value = targetResponse.data.data.items
    scenarios.value = scenarioResponse.data.data.items
    issueTags.value = optionResponse.data.data.issue_tags
    const target = targets.value.find((item) => item.id === targetId && item.status === 'active')
    const usable = target && scenarios.value.some(
      (item) => item.evaluation_target_id === target.id && item.status === 'active',
    )
    if (!usable) {
      currentTargetId.value = ''
      pickerRequired.value = true
      targetPickerOpen.value = true
      return
    }
    currentTargetId.value = target.id
    pickerRequired.value = false
    restoreDraft()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

async function confirmDiscardContext() {
  if (recordLocked.value && uploadFailed.value) {
    try {
      await ElMessageBox.confirm(
        'Badcase 已创建，但截图仍未上传。离开后将无法在此处继续重试，仍要离开吗？',
        '截图上传尚未完成',
        { confirmButtonText: '仍要离开', cancelButtonText: '继续处理', type: 'warning' },
      )
      return true
    } catch {
      return false
    }
  }
  if (!hasProblemContent.value) return true
  try {
    await ElMessageBox.confirm(
      '当前登记内容尚未提交。文字草稿会保留在当前对象下，截图不会保存，仍要离开吗？',
      '确认离开当前登记',
      { confirmButtonText: '仍要离开', cancelButtonText: '继续填写', type: 'warning' },
    )
    saveDraft()
    return true
  } catch {
    return false
  }
}

async function selectTarget(targetId: string) {
  if (targetId === currentTargetId.value) return
  await router.replace({ name: 'badcase-register', query: { evaluation_target_id: targetId } })
}

function closeRequiredPicker() {
  router.replace({ name: 'badcases' })
}

function validateFiles(files: File[]) {
  const maxCount = auth.uploadPolicy.max_files_per_owner
  if (pendingScreenshots.value.length + files.length > maxCount) {
    ElMessage.warning(`最多上传 ${maxCount} 张截图`)
    return false
  }
  const invalid = files.find(
    (file) =>
      !auth.uploadPolicy.allowed_media_types.includes(file.type) ||
      file.size > auth.uploadPolicy.max_file_size,
  )
  if (invalid) {
    ElMessage.warning(`“${invalid.name}”格式不支持或超过 10 MB，请选择 PNG、JPG 或 WebP 图片`)
    return false
  }
  return true
}

function addFiles(files: File[]) {
  if (recordLocked.value) return
  const images = files.filter((file) => file.type.startsWith('image/'))
  if (!images.length || !validateFiles(images)) return
  pendingScreenshots.value.push(
    ...images.map((file) => ({ id: crypto.randomUUID(), file, previewUrl: URL.createObjectURL(file) })),
  )
  scheduleDraftSave()
}

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  addFiles(Array.from(input.files || []))
  input.value = ''
}

function handlePaste(event: ClipboardEvent) {
  if (recordLocked.value) return
  const files = Array.from(event.clipboardData?.items || [])
    .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
  if (files.length) {
    event.preventDefault()
    addFiles(files)
    ElMessage.success(`已粘贴 ${files.length} 张截图`)
  }
}

function handleDrop(event: DragEvent) {
  if (!recordLocked.value) addFiles(Array.from(event.dataTransfer?.files || []))
}

function removeScreenshot(index: number) {
  if (recordLocked.value) return
  const screenshot = pendingScreenshots.value[index]
  if (!screenshot) return
  URL.revokeObjectURL(screenshot.previewUrl)
  pendingScreenshots.value.splice(index, 1)
  scheduleDraftSave()
}

function moveScreenshot(index: number, offset: number) {
  if (recordLocked.value) return
  const target = index + offset
  if (target < 0 || target >= pendingScreenshots.value.length) return
  const reordered = [...pendingScreenshots.value]
  const [moved] = reordered.splice(index, 1)
  if (!moved) return
  reordered.splice(target, 0, moved)
  pendingScreenshots.value = reordered
  scheduleDraftSave()
}

function dropScreenshot(targetIndex: number) {
  if (recordLocked.value) return
  if (draggedIndex < 0 || draggedIndex === targetIndex) return
  const reordered = [...pendingScreenshots.value]
  const [moved] = reordered.splice(draggedIndex, 1)
  if (!moved) return
  reordered.splice(targetIndex, 0, moved)
  pendingScreenshots.value = reordered
  draggedIndex = -1
  scheduleDraftSave()
}

function revokePreviews() {
  pendingScreenshots.value.forEach((item) => URL.revokeObjectURL(item.previewUrl))
  pendingScreenshots.value = []
}

function applyValidation() {
  Object.assign(errors, validateRegistration(form))
  return isRegistrationValid(form)
}

async function uploadScreenshots() {
  if (!createdItem.value || pendingScreenshots.value.length === 0) return true
  const body = new FormData()
  pendingScreenshots.value.forEach((item) => body.append('files', item.file))
  body.append('expected_owner_lock_version', String(createdItem.value.lock_version))
  try {
    await apiClient.post(`/api/v1/badcases/${createdItem.value.id}/attachments`, body, {
      headers: { 'Idempotency-Key': uploadIdempotencyKey.value },
    })
    uploadFailed.value = false
    return true
  } catch (error) {
    uploadFailed.value = true
    ElMessage.error(`Badcase 已创建，但截图上传失败：${apiErrorMessage(error)}`)
    return false
  }
}

function finishRegistration() {
  if (!createdItem.value) return
  recentItems.value = [createdItem.value, ...recentItems.value].slice(0, 5)
  const key = currentDraftKey()
  if (key) localStorage.removeItem(key)
  const nextForm = resetAfterRegistration(form)
  Object.assign(form, nextForm)
  revokePreviews()
  createdItem.value = null
  uploadFailed.value = false
  createIdempotencyKey.value = crypto.randomUUID()
  uploadIdempotencyKey.value = crypto.randomUUID()
  draftStatus.value = 'idle'
  ElMessage.success('Badcase 已登记，可继续录入下一条')
}

async function submit() {
  if (createdItem.value) {
    submitting.value = true
    const uploaded = await uploadScreenshots()
    submitting.value = false
    if (uploaded) finishRegistration()
    return
  }
  if (!applyValidation()) {
    ElMessage.warning('请先补全所有必填项')
    return
  }
  if (!form.agent_response_text.trim() && pendingScreenshots.value.length === 0) {
    try {
      await ElMessageBox.confirm(
        '当前未填写 Agent 回答，也未添加现场截图。缺少现场证据可能影响后续定位，仍要登记吗？',
        '现场证据为空',
        { confirmButtonText: '仍要登记', cancelButtonText: '返回补充', type: 'warning' },
      )
    } catch {
      return
    }
  }
  submitting.value = true
  try {
    const response = await apiClient.post<ResponseEnvelope<Badcase>>(
      '/api/v1/badcases',
      {
        scenario_id: form.scenario_id,
        title: form.title.trim(),
        description: form.description.trim() || null,
        agent_response_text: form.agent_response_text.trim() || null,
        agent_version: form.agent_version.trim() || null,
        environment: form.environment,
        occurred_at: new Date(form.occurred_at).toISOString(),
        business_reference: form.business_reference.trim() || null,
        session_id: form.session_id.trim() || null,
        issue_tag_ids: form.issue_tag_ids,
      },
      { headers: { 'Idempotency-Key': createIdempotencyKey.value } },
    )
    createdItem.value = response.data.data
    if (await uploadScreenshots()) finishRegistration()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    submitting.value = false
  }
}

function discardFailedScreenshots() {
  uploadFailed.value = false
  finishRegistration()
}

function clearCurrent() {
  Object.assign(form, {
    ...emptyRegistrationForm(),
    scenario_id: targetScenarios.value[0]?.id || '',
  })
  revokePreviews()
  const key = currentDraftKey()
  if (key) localStorage.removeItem(key)
  draftStatus.value = 'idle'
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function beforeUnload(event: BeforeUnloadEvent) {
  if ((!hasProblemContent.value && !uploadFailed.value) || (recordLocked.value && !uploadFailed.value)) return
  if (!recordLocked.value) saveDraft()
  event.preventDefault()
}

watch(
  form,
  () => {
    if (draftReady.value && !recordLocked.value) {
      Object.assign(errors, {
        title: '',
        scenario_id: '',
        issue_tag_ids: '',
        environment: '',
        occurred_at: '',
      })
      createIdempotencyKey.value = crypto.randomUUID()
      scheduleDraftSave()
    }
  },
  { deep: true },
)

watch(
  () => form.scenario_id,
  () => {
    const allowed = new Set(filteredIssueTags.value.map((tag) => tag.id))
    form.issue_tag_ids = form.issue_tag_ids.filter((id) => allowed.has(id))
  },
)

watch(
  () => route.query.evaluation_target_id,
  (value) => void loadPage(String(value || '')),
  { immediate: true },
)

onBeforeRouteLeave(async () => confirmDiscardContext())
onBeforeRouteUpdate(async (to, from) => {
  if (to.query.evaluation_target_id === from.query.evaluation_target_id) return true
  const confirmed = await confirmDiscardContext()
  if (confirmed) revokePreviews()
  return confirmed
})
onMounted(() => {
  document.addEventListener('paste', handlePaste)
  window.addEventListener('beforeunload', beforeUnload)
})
onBeforeUnmount(() => {
  if (draftTimer) clearTimeout(draftTimer)
  saveDraft()
  revokePreviews()
  document.removeEventListener('paste', handlePaste)
  window.removeEventListener('beforeunload', beforeUnload)
})
</script>

<template>
  <section v-loading="loading" class="badcase-register-page">
    <div class="register-heading">
      <div>
        <h1>主动登记 Badcase</h1>
        <p>记录真实业务中发现的问题，保存后可继续登记下一条。</p>
      </div>
      <span class="draft-indicator" :class="{ saved: draftStatus === 'saved' }">
        <span class="draft-dot" />{{ draftStatus === 'saved' ? '草稿已自动保存' : '输入后自动保存草稿' }}
      </span>
    </div>

    <div v-if="currentTarget" class="target-context-bar">
      <span class="target-context-icon">Q</span>
      <div>
        <strong>{{ currentTarget.name }}</strong>
        <span>{{ currentTarget.description || '当前登记的评测对象' }}</span>
      </div>
      <el-button :icon="Switch" :disabled="recordLocked" @click="targetPickerOpen = true">
        切换对象
      </el-button>
    </div>

    <div v-if="currentTarget" class="register-workspace" :class="{ locked: recordLocked }">
      <section class="register-panel evidence-panel">
        <h2>问题与现场证据</h2>
        <el-form label-position="top" :disabled="recordLocked">
          <el-form-item label="Badcase 标题" required :error="errors.title">
            <el-input v-model="form.title" maxlength="200" placeholder="请输入 Badcase 标题" />
          </el-form-item>
          <el-form-item label="问题描述（可选）">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="3"
              maxlength="5000"
              placeholder="请简要描述问题现象、影响范围、期望结果等"
            />
          </el-form-item>
          <el-form-item label="Agent 回答文本">
            <el-input
              v-model="form.agent_response_text"
              type="textarea"
              :rows="6"
              maxlength="20000"
              show-word-limit
              placeholder="请粘贴 Agent 的完整回答文本，便于复现与分析"
            />
          </el-form-item>
          <el-form-item label="现场截图">
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
            <div
              class="register-upload-zone"
              :class="{ disabled: recordLocked }"
              role="button"
              :tabindex="recordLocked ? -1 : 0"
              @click="!recordLocked && fileInput?.click()"
              @keydown.enter="!recordLocked && fileInput?.click()"
              @keydown.space.prevent="!recordLocked && fileInput?.click()"
              @dragover.prevent
              @drop.prevent="handleDrop"
            >
              <el-icon :size="30"><UploadFilled /></el-icon>
              <strong>粘贴、拖拽截图到这里，或 <span>选择文件</span></strong>
              <small>
                支持 PNG、JPG、WebP，最多 {{ auth.uploadPolicy.max_files_per_owner }} 张，单张不超过 10 MB
              </small>
            </div>
            <div v-if="pendingScreenshots.length" class="screenshot-grid">
              <article
                v-for="(item, index) in pendingScreenshots"
                :key="item.id"
                class="screenshot-card"
                :draggable="!recordLocked"
                @dragstart="draggedIndex = index"
                @dragover.prevent
                @drop.prevent="dropScreenshot(index)"
              >
                <el-image
                  :src="item.previewUrl"
                  fit="cover"
                  :preview-src-list="pendingScreenshots.map((shot) => shot.previewUrl)"
                  :initial-index="index"
                  preview-teleported
                />
                <div class="screenshot-actions">
                  <el-button text :icon="Rank" :disabled="recordLocked" aria-label="拖动排序" />
                  <span>{{ index + 1 }}</span>
                  <el-button
                    text
                    :icon="ArrowUp"
                    :disabled="recordLocked || index === 0"
                    aria-label="向前移动"
                    @click="moveScreenshot(index, -1)"
                  />
                  <el-button
                    text
                    :icon="ArrowDown"
                    :disabled="recordLocked || index === pendingScreenshots.length - 1"
                    aria-label="向后移动"
                    @click="moveScreenshot(index, 1)"
                  />
                  <el-button
                    text
                    :icon="Delete"
                    :disabled="recordLocked"
                    aria-label="删除截图"
                    @click="removeScreenshot(index)"
                  />
                </div>
              </article>
            </div>
          </el-form-item>
        </el-form>
      </section>

      <section class="register-panel context-panel">
        <h2>归属与定位</h2>
        <el-form label-position="top" :disabled="recordLocked">
          <el-form-item label="所属场景" required :error="errors.scenario_id">
            <el-select v-model="form.scenario_id" placeholder="请选择启用场景">
              <el-option
                v-for="scenario in targetScenarios"
                :key="scenario.id"
                :label="scenario.name"
                :value="scenario.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="问题标签" required :error="errors.issue_tag_ids">
            <el-select v-model="form.issue_tag_ids" multiple filterable placeholder="至少选择一个问题标签">
              <el-option v-for="tag in filteredIssueTags" :key="tag.id" :label="tag.name" :value="tag.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="运行环境" required :error="errors.environment">
            <el-select v-model="form.environment">
              <el-option label="测试" value="test" />
              <el-option label="预发布" value="staging" />
              <el-option label="生产" value="production" />
              <el-option label="其他" value="other" />
            </el-select>
          </el-form-item>
          <el-form-item label="Agent 版本（可选）">
            <el-input v-model="form.agent_version" maxlength="100" placeholder="例如 2026.07.30" />
          </el-form-item>
          <el-form-item label="发生时间" required :error="errors.occurred_at">
            <el-date-picker
              v-model="form.occurred_at"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm"
              format="YYYY-MM-DD HH:mm"
              placeholder="选择发生时间"
            />
          </el-form-item>
          <div class="location-more">
            <button type="button" @click="moreLocationOpen = !moreLocationOpen">
              <span>更多定位信息</span>
              <el-icon :class="{ expanded: moreLocationOpen }"><ArrowDown /></el-icon>
            </button>
            <div v-show="moreLocationOpen" class="location-fields">
              <el-form-item label="业务单号（可选）">
                <el-input v-model="form.business_reference" maxlength="200" placeholder="请输入业务单号" />
              </el-form-item>
              <el-form-item label="会话 ID（可选）">
                <el-input v-model="form.session_id" maxlength="200" placeholder="请输入会话 ID" />
              </el-form-item>
            </div>
          </div>
        </el-form>

        <div v-if="uploadFailed && createdItem" class="upload-recovery" role="alert">
          <strong>Badcase 已创建，截图尚未上传</strong>
          <p>记录已锁定，不会重复创建。你可以用原幂等请求重试上传，或放弃本次截图。</p>
          <div>
            <el-button :icon="Delete" @click="discardFailedScreenshots">放弃截图</el-button>
            <el-button type="primary" :icon="RefreshRight" :loading="submitting" @click="submit">
              重试上传
            </el-button>
          </div>
        </div>
      </section>
    </div>

    <section v-if="currentTarget" class="recent-registration">
      <h2>本次登记 {{ recentItems.length }} 条</h2>
      <el-empty v-if="recentItems.length === 0" description="本次会话还没有成功登记的 Badcase" :image-size="52" />
      <div v-else class="recent-table">
        <div class="recent-table-head"><span>标题</span><span>场景</span><span>发生时间</span><span>操作</span></div>
        <div v-for="item in recentItems" :key="item.id" class="recent-table-row">
          <strong>{{ item.title }}</strong>
          <span>{{ item.scenario_name }}</span>
          <span>{{ formatTime(item.occurred_at) }}</span>
          <el-button link type="primary" @click="router.push(`/badcases/${item.id}`)">查看详情</el-button>
        </div>
      </div>
    </section>

    <footer v-if="currentTarget" class="register-footer">
      <el-button :disabled="recordLocked" @click="clearCurrent">清空当前内容</el-button>
      <div>
        <span v-if="!recordLocked">保存后将保留场景、环境和 Agent 版本</span>
        <el-button
          v-if="!uploadFailed"
          type="primary"
          :loading="submitting"
          :disabled="recordLocked && !uploadFailed"
          @click="submit"
        >
          登记并继续
        </el-button>
      </div>
    </footer>
  </section>

  <EvaluationTargetDialog
    v-model="targetPickerOpen"
    :required="pickerRequired"
    :current-target-id="currentTargetId"
    @select="selectTarget"
    @close-required="closeRequiredPicker"
  />
</template>

<style scoped>
.badcase-register-page {
  min-height: calc(100vh - 120px);
  padding-bottom: 84px;
}

.register-heading,
.target-context-bar,
.register-footer,
.register-footer > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.register-heading {
  margin-bottom: 16px;
}

.register-heading h1 {
  margin: 0 0 5px;
  font-size: 24px;
  font-weight: 700;
  line-height: 1.35;
}

.register-heading p {
  margin: 0;
  color: #667085;
}

.draft-indicator {
  color: #667085;
  font-size: 13px;
}

.draft-indicator.saved {
  color: #16845b;
}

.draft-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 7px;
  border-radius: 50%;
  background: currentColor;
}

.target-context-bar,
.register-panel,
.recent-registration {
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 5px 20px rgb(39 62 102 / 7%);
}

.target-context-bar {
  gap: 14px;
  margin-bottom: 14px;
  padding: 16px 20px;
}

.target-context-icon {
  display: grid;
  width: 46px;
  height: 46px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 11px;
  color: #fff;
  background: #1769e0;
  font-size: 20px;
  font-weight: 700;
  box-shadow: 0 7px 16px rgb(23 105 224 / 22%);
}

.target-context-bar > div {
  min-width: 0;
  flex: 1;
}

.target-context-bar strong,
.target-context-bar span {
  display: block;
}

.target-context-bar strong {
  margin-bottom: 5px;
  font-size: 16px;
  font-weight: 600;
}

.target-context-bar div span {
  overflow: hidden;
  color: #667085;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.register-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(340px, 0.95fr);
  gap: 14px;
}

.register-panel {
  padding: 20px;
}

.register-panel h2,
.recent-registration h2 {
  margin: 0 0 17px;
  color: #1d2939;
  font-size: 16px;
}

.context-panel .el-select,
.context-panel .el-date-editor {
  width: 100%;
}

.register-upload-zone {
  display: flex;
  width: 100%;
  min-height: 102px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  border: 1px dashed #9bb7dc;
  border-radius: 9px;
  color: #53657c;
  background: #fbfdff;
  cursor: pointer;
  transition: border-color 160ms ease, background 160ms ease;
}

.register-upload-zone:hover,
.register-upload-zone:focus-visible {
  border-color: #1769e0;
  outline: 0;
  background: #f5f9ff;
}

.register-upload-zone.disabled {
  opacity: 0.62;
  cursor: not-allowed;
}

.register-upload-zone strong {
  margin: 7px 0 4px;
  font-size: 14px;
  font-weight: 500;
}

.register-upload-zone strong span {
  color: #1769e0;
}

.register-upload-zone small {
  color: #667085;
}

.screenshot-grid {
  display: grid;
  width: 100%;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.screenshot-card {
  overflow: hidden;
  border: 1px solid #dbe3ef;
  border-radius: 10px;
  background: #fff;
}

.screenshot-card .el-image {
  display: block;
  width: 100%;
  height: 108px;
}

.screenshot-actions {
  display: flex;
  height: 34px;
  align-items: center;
  justify-content: space-between;
  padding: 0 4px;
  color: #667085;
}

.screenshot-actions .el-button {
  margin: 0;
}

.location-more {
  overflow: hidden;
  border: 1px solid #e4e9f2;
  border-radius: 9px;
}

.location-more > button {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  padding: 11px 12px;
  border: 0;
  color: #1769e0;
  background: #f8fbff;
  cursor: pointer;
}

.location-more .el-icon {
  transition: transform 180ms ease;
}

.location-more .el-icon.expanded {
  transform: rotate(180deg);
}

.location-fields {
  padding: 12px 12px 0;
}

.upload-recovery {
  margin-top: 18px;
  padding: 14px;
  border-radius: 10px;
  color: #7a2e0e;
  background: #fff4ed;
}

.upload-recovery p {
  margin: 6px 0 12px;
  color: #934b2d;
  font-size: 13px;
  line-height: 1.55;
}

.upload-recovery > div {
  display: flex;
  justify-content: flex-end;
}

.recent-registration {
  margin-top: 14px;
  padding: 16px 20px;
}

.recent-registration h2 {
  margin-bottom: 8px;
}

.recent-registration :deep(.el-empty) {
  padding: 8px 0;
}

.recent-table-head,
.recent-table-row {
  display: grid;
  grid-template-columns: minmax(280px, 2fr) minmax(140px, 1fr) 190px 90px;
  gap: 18px;
  align-items: center;
  min-height: 38px;
  border-bottom: 1px solid #edf0f5;
  font-size: 13px;
}

.recent-table-head {
  min-height: 32px;
  color: #667085;
}

.recent-table-row:last-child {
  border-bottom: 0;
}

.recent-table-row strong {
  overflow: hidden;
  color: #344054;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.register-footer {
  position: fixed;
  z-index: 8;
  right: 0;
  bottom: 0;
  left: 216px;
  min-height: 66px;
  padding: 12px 28px;
  border-top: 1px solid #e4e9f2;
  background: rgb(255 255 255 / 97%);
  box-shadow: 0 -8px 24px rgb(39 62 102 / 6%);
}

.register-footer > div {
  gap: 18px;
}

.register-footer span {
  color: #667085;
  font-size: 13px;
}

@media (max-width: 1280px) {
  .register-workspace {
    grid-template-columns: minmax(570px, 1.55fr) minmax(330px, 0.9fr);
  }
}

@media (max-width: 860px) {
  .badcase-register-page {
    padding-bottom: 112px;
  }

  .register-heading,
  .target-context-bar,
  .register-footer > div {
    align-items: stretch;
    flex-direction: column;
  }

  .register-workspace {
    grid-template-columns: 1fr;
  }

  .register-panel,
  .recent-registration {
    padding: 16px;
  }

  .target-context-bar {
    align-items: flex-start;
  }

  .target-context-bar div span {
    white-space: normal;
  }

  .screenshot-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .recent-table-head {
    display: none;
  }

  .recent-table-row {
    grid-template-columns: 1fr auto;
    gap: 6px 12px;
    padding: 10px 0;
  }

  .register-footer {
    left: 0;
    padding: 10px 14px;
  }

  .register-footer > div {
    gap: 8px;
  }

  .register-footer .el-button {
    width: 100%;
    margin: 0;
  }
}
</style>
