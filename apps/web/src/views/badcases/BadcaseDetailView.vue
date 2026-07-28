<script setup lang="ts">
import { ArrowDown, ArrowLeft, ArrowUp, Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { Attachment, Badcase, BadcasePage, ResponseEnvelope } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const commanding = ref(false)
const uploading = ref(false)
const editOpen = ref(false)
const item = ref<BadcasePage | null>(null)
const assigneeID = ref('')
const selectedTagIDs = ref<string[]>([])
const note = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const editForm = reactive({
  title: '',
  description: '',
  agent_response_text: '',
  agent_version: '',
  environment: 'production',
  occurred_at: '',
  business_reference: '',
  session_id: '',
})
const evidence = computed(() => [
  ...(item.value?.original_attachments || []),
  ...(item.value?.attachments || []),
])

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<BadcasePage>>(
      `/api/v1/pages/badcases/${String(route.params.badcaseId)}`,
    )
    item.value = response.data.data
    assigneeID.value = item.value.assignee_id || ''
    selectedTagIDs.value = item.value.issue_tags.map((tag) => tag.id)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function can(action: string) {
  return item.value?.allowed_actions.includes(action) || false
}

async function workflow(
  action: string,
  payload: Record<string, unknown>,
  success: string,
  idempotent = false,
) {
  if (!item.value) return
  commanding.value = true
  try {
    await apiClient.post(`/api/v1/badcases/${item.value.id}/${action}`, {
      expected_lock_version: item.value.lock_version,
      ...payload,
    }, idempotent ? { headers: { 'Idempotency-Key': crypto.randomUUID() } } : undefined)
    ElMessage.success(success)
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    commanding.value = false
  }
}

async function assign() {
  if (!item.value) return
  if (assigneeID.value) {
    await workflow('assign', { assignee_id: assigneeID.value }, '负责人已更新')
  } else if (item.value.assignee_id) {
    await workflow('unassign', {}, '已取消负责人')
  }
}

async function transition(action: string, label: string) {
  if (!item.value) return
  try {
    const { value } = await ElMessageBox.prompt(`请填写${label}说明`, label, {
      inputType: 'textarea',
      inputValidator: (text) => Boolean(text.trim()) || '说明不能为空',
    })
    await workflow(action, { reason: value }, `${label}成功`)
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(apiErrorMessage(error))
  }
}

async function addNote() {
  if (!note.value.trim()) return
  const value = note.value
  await workflow('add-note', { note: value }, '处理备注已添加', true)
  note.value = ''
}

async function updateTags() {
  if (!item.value || selectedTagIDs.value.length === 0) return
  commanding.value = true
  try {
    await apiClient.put(`/api/v1/badcases/${item.value.id}/issue-tags`, {
      issue_tag_ids: selectedTagIDs.value,
      expected_lock_version: item.value.lock_version,
    })
    ElMessage.success('问题标签已更新')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    commanding.value = false
  }
}

function openEdit() {
  if (!item.value) return
  Object.assign(editForm, {
    title: item.value.title,
    description: item.value.description || '',
    agent_response_text: item.value.agent_response_text || '',
    agent_version: item.value.agent_version || '',
    environment: item.value.environment,
    occurred_at: new Date(item.value.occurred_at).toISOString().slice(0, 16),
    business_reference: item.value.business_reference || '',
    session_id: item.value.session_id || '',
  })
  editOpen.value = true
}

async function saveEdit() {
  if (!item.value) return
  commanding.value = true
  try {
    await apiClient.patch(`/api/v1/badcases/${item.value.id}`, {
      title: editForm.title,
      description: editForm.description,
      agent_response_text: editForm.agent_response_text || null,
      agent_version: editForm.agent_version || null,
      environment: editForm.environment,
      occurred_at: new Date(editForm.occurred_at).toISOString(),
      business_reference: editForm.business_reference || null,
      session_id: editForm.session_id || null,
      expected_lock_version: item.value.lock_version,
    })
    editOpen.value = false
    ElMessage.success('原始问题内容已更新')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    commanding.value = false
  }
}

function chooseFiles() {
  fileInput.value?.click()
}

async function uploadFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  input.value = ''
  if (!item.value || files.length === 0) return
  if (files.length + item.value.attachments.length > auth.uploadPolicy.max_files_per_owner) {
    ElMessage.warning(`补充截图最多 ${auth.uploadPolicy.max_files_per_owner} 张`)
    return
  }
  const invalid = files.find(
    (file) =>
      !auth.uploadPolicy.allowed_media_types.includes(file.type) ||
      file.size > auth.uploadPolicy.max_file_size,
  )
  if (invalid) {
    ElMessage.warning(`文件 ${invalid.name} 的格式不支持或超过 10 MB`)
    return
  }
  uploading.value = true
  try {
    const body = new FormData()
    files.forEach((file) => body.append('files', file))
    body.append('expected_owner_lock_version', String(item.value.lock_version))
    await apiClient.post(`/api/v1/badcases/${item.value.id}/attachments`, body, {
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    })
    ElMessage.success('定位截图已上传')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    uploading.value = false
  }
}

async function deleteAttachment(attachment: Attachment) {
  if (!item.value) return
  try {
    await ElMessageBox.confirm('确定删除这张补充截图吗？', '删除截图', { type: 'warning' })
    await apiClient.delete(`/api/v1/attachments/${attachment.id}`, {
      params: { expected_owner_lock_version: item.value.lock_version },
    })
    await load()
  } catch (error) {
    if (error !== 'cancel') ElMessage.error(apiErrorMessage(error))
  }
}

async function reorderAttachment(index: number, offset: number) {
  if (!item.value) return
  const target = index + offset
  if (target < 0 || target >= item.value.attachments.length) return
  const reordered = [...item.value.attachments]
  const current = reordered[index]
  const other = reordered[target]
  if (!current || !other) return
  reordered[index] = other
  reordered[target] = current
  try {
    await apiClient.post(`/api/v1/badcases/${item.value.id}/attachments/reorder`, {
      expected_owner_lock_version: item.value.lock_version,
      items: reordered.map((entry, position) => ({ id: entry.id, sort_order: position })),
    })
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

function mayDelete(attachment: Attachment) {
  return auth.isAdmin || attachment.created_by === auth.user?.id || item.value?.created_by === auth.user?.id
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function statusLabel(status: Badcase['status']) {
  return {
    pending: '待处理',
    processing: '处理中',
    resolved: '已解决',
    deferred: '暂不处理',
  }[status]
}

function activityText(activity: Badcase['activities'][number]) {
  if (activity.activity_type === 'created') return '创建 Badcase'
  if (activity.activity_type === 'note_added') return '添加处理备注'
  if (activity.activity_type === 'invalidated') return '无效化 Badcase'
  if (activity.activity_type === 'reactivated') return '重新激活 Badcase'
  if (activity.activity_type === 'assignee_changed') {
    return `负责人：${activity.from_assignee_name || '未分配'} → ${activity.to_assignee_name || '未分配'}`
  }
  if (activity.activity_type === 'status_changed') {
    return `状态：${statusLabel(activity.from_status as Badcase['status'])} → ${statusLabel(activity.to_status as Badcase['status'])}`
  }
  return activity.activity_type
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="badcase-detail-page">
    <template v-if="item">
      <div class="detail-back">
        <el-button :icon="ArrowLeft" text @click="router.push('/badcases')">返回 Badcase 中心</el-button>
      </div>
      <el-alert
        v-if="item.invalidated_at"
        type="warning"
        :closable="false"
        :title="`该 Badcase 已无效：${item.invalid_reason}`"
        :description="`${item.invalidator_name || ''} · ${formatTime(item.invalidated_at)}`"
      />
      <div class="badcase-detail-heading">
        <div>
          <p class="eyebrow">{{ item.source_type === 'business' ? '业务 Badcase' : '评测 Badcase' }}</p>
          <h1>{{ item.title }}</h1>
          <p>{{ item.evaluation_target_name }} · {{ item.scenario_name }} · {{ formatTime(item.occurred_at) }}</p>
        </div>
        <div class="heading-actions">
          <el-tag :type="item.invalidated_at ? 'info' : item.status === 'resolved' ? 'success' : 'danger'">
            {{ item.invalidated_at ? '无效' : statusLabel(item.status) }}
          </el-tag>
          <el-button v-if="can('edit')" :icon="Edit" @click="openEdit">编辑原始内容</el-button>
          <el-button
            v-if="can('invalidate')"
            type="danger"
            plain
            @click="transition('invalidate', '无效化')"
          >
            无效化
          </el-button>
          <el-button
            v-if="can('reactivate')"
            type="primary"
            @click="transition('reactivate', '重新激活')"
          >
            重新激活
          </el-button>
        </div>
      </div>

      <div class="badcase-detail-grid">
        <main>
          <el-card shadow="never">
            <template #header><strong>问题现场</strong></template>
            <h3>问题描述</h3>
            <p class="prewrap">{{ item.description || '-' }}</p>
            <h3>Agent 回答</h3>
            <p class="prewrap">{{ item.agent_response_text || '未记录文本，请查看截图证据。' }}</p>
            <el-descriptions v-if="item.source_type === 'business'" :column="2" border>
              <el-descriptions-item label="业务单号">{{ item.business_reference || '-' }}</el-descriptions-item>
              <el-descriptions-item label="会话 ID">{{ item.session_id || '-' }}</el-descriptions-item>
            </el-descriptions>
          </el-card>

          <el-card v-if="item.evaluation" shadow="never">
            <template #header><strong>原始评测上下文</strong></template>
            <h3>用户问题或任务指令</h3>
            <p class="prewrap">{{ item.evaluation.user_prompt || '-' }}</p>
            <h3>评测意见</h3>
            <p class="prewrap">{{ item.evaluation.comment || '-' }}</p>
            <el-button
              class="trace-link"
              type="primary"
              plain
              @click="router.push(`/evaluation-runs/${item.evaluation?.evaluation_run_id}/result`)"
            >
              查看完整评测结果
            </el-button>
          </el-card>

          <el-card shadow="never">
            <template #header>
              <div class="card-header-actions">
                <strong>截图证据</strong>
                <el-button v-if="can('add_attachment')" :icon="Plus" :loading="uploading" @click="chooseFiles">
                  补充截图
                </el-button>
              </div>
            </template>
            <input
              ref="fileInput"
              class="visually-hidden"
              type="file"
              accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
              multiple
              @change="uploadFiles"
            />
            <el-empty v-if="evidence.length === 0" description="暂无截图证据" :image-size="70" />
            <div v-else class="badcase-evidence-grid">
              <div v-for="(attachment, index) in item.original_attachments" :key="attachment.id" class="detail-evidence-card">
                <el-image
                  :src="attachment.content_url"
                  :preview-src-list="evidence.map((entry) => entry.content_url)"
                  :initial-index="index"
                  fit="cover"
                  preview-teleported
                />
                <span>原始评测证据</span>
              </div>
              <div v-for="(attachment, index) in item.attachments" :key="attachment.id" class="detail-evidence-card">
                <el-image
                  :src="attachment.content_url"
                  :preview-src-list="evidence.map((entry) => entry.content_url)"
                  :initial-index="item.original_attachments.length + index"
                  fit="cover"
                  preview-teleported
                />
                <span>{{ attachment.original_name }}</span>
                <div v-if="can('add_attachment')" class="evidence-actions">
                  <el-button text :icon="ArrowUp" :disabled="index === 0" @click="reorderAttachment(index, -1)" />
                  <el-button text :icon="ArrowDown" :disabled="index === item.attachments.length - 1" @click="reorderAttachment(index, 1)" />
                  <el-button v-if="mayDelete(attachment)" text type="danger" :icon="Delete" @click="deleteAttachment(attachment)" />
                </div>
              </div>
            </div>
          </el-card>

          <el-card shadow="never">
            <template #header><strong>处理时间线</strong></template>
            <el-timeline>
              <el-timeline-item
                v-for="activity in item.activities"
                :key="activity.id"
                :timestamp="formatTime(activity.created_at)"
              >
                <strong>{{ activityText(activity) }}</strong> · {{ activity.actor_name }}
                <p v-if="activity.note">{{ activity.note }}</p>
              </el-timeline-item>
            </el-timeline>
            <div v-if="can('add_note')" class="note-composer">
              <el-input v-model="note" type="textarea" :rows="3" placeholder="补充定位结论、处理进展或后续计划" />
              <el-button type="primary" :disabled="!note.trim()" :loading="commanding" @click="addNote">
                添加备注
              </el-button>
            </div>
          </el-card>
        </main>

        <aside class="badcase-sidebar">
          <el-card shadow="never">
            <template #header><strong>处理动作</strong></template>
            <el-form label-position="top">
              <el-form-item label="负责人">
                <div class="inline-control">
                  <el-select v-model="assigneeID" clearable placeholder="未分配" :disabled="!can('assign')">
                    <el-option
                      v-for="user in item.candidate_assignees"
                      :key="user.id"
                      :label="user.display_name"
                      :value="user.id"
                    />
                  </el-select>
                  <el-button :disabled="!can('assign') || assigneeID === (item.assignee_id || '')" @click="assign">
                    保存
                  </el-button>
                </div>
              </el-form-item>
              <el-form-item label="问题标签">
                <el-select v-model="selectedTagIDs" multiple :disabled="!can('update_tags')">
                  <el-option
                    v-for="tag in item.candidate_issue_tags"
                    :key="tag.id"
                    :label="tag.name"
                    :value="tag.id"
                  />
                </el-select>
                <el-button
                  class="full-control-button"
                  :disabled="!can('update_tags') || selectedTagIDs.length === 0"
                  @click="updateTags"
                >
                  保存标签
                </el-button>
              </el-form-item>
            </el-form>
            <div class="workflow-buttons">
              <el-button v-if="can('start_processing')" type="primary" @click="transition('start-processing', '开始处理')">开始处理</el-button>
              <el-button v-if="can('resolve')" type="success" @click="transition('resolve', '标记已解决')">标记已解决</el-button>
              <el-button v-if="can('defer')" @click="transition('defer', '暂不处理')">暂不处理</el-button>
              <el-button v-if="can('reopen')" type="primary" plain @click="transition('reopen', '重新打开')">重新打开</el-button>
            </div>
          </el-card>

          <el-card shadow="never">
            <template #header><strong>基本信息</strong></template>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="来源">{{ item.source_type === 'business' ? '业务登记' : '评测发现' }}</el-descriptions-item>
              <el-descriptions-item label="创建人">{{ item.creator_name }}</el-descriptions-item>
              <el-descriptions-item label="Agent 版本">{{ item.agent_version || '-' }}</el-descriptions-item>
              <el-descriptions-item label="运行环境">{{ item.environment }}</el-descriptions-item>
              <el-descriptions-item v-if="item.evaluation" label="评测集">
                {{ item.evaluation.dataset_name }} V{{ item.evaluation.version_no }}
              </el-descriptions-item>
              <el-descriptions-item v-if="item.evaluation" label="评分">{{ item.evaluation.score || '未评分' }}</el-descriptions-item>
            </el-descriptions>
          </el-card>
        </aside>
      </div>
    </template>
  </section>

  <el-dialog v-model="editOpen" title="编辑业务 Badcase" width="640">
    <el-form label-position="top">
      <el-form-item label="问题标题" required><el-input v-model="editForm.title" maxlength="200" /></el-form-item>
      <el-form-item label="问题描述" required><el-input v-model="editForm.description" type="textarea" :rows="4" /></el-form-item>
      <el-form-item label="Agent 回答"><el-input v-model="editForm.agent_response_text" type="textarea" :rows="5" /></el-form-item>
      <div class="drawer-form-grid">
        <el-form-item label="Agent 版本"><el-input v-model="editForm.agent_version" maxlength="100" /></el-form-item>
        <el-form-item label="运行环境">
          <el-select v-model="editForm.environment">
            <el-option label="测试" value="test" /><el-option label="预发布" value="staging" />
            <el-option label="生产" value="production" /><el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="发生时间"><el-input v-model="editForm.occurred_at" type="datetime-local" /></el-form-item>
        <el-form-item label="业务单号"><el-input v-model="editForm.business_reference" /></el-form-item>
        <el-form-item label="会话 ID"><el-input v-model="editForm.session_id" /></el-form-item>
      </div>
    </el-form>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button
        type="primary"
        :loading="commanding"
        :disabled="!editForm.title.trim() || !editForm.description.trim()"
        @click="saveEdit"
      >
        保存
      </el-button>
    </template>
  </el-dialog>
</template>
