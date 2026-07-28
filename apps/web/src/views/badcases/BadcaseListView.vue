<script setup lang="ts">
import { Plus, Search, UploadFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { Badcase, PageData, ResponseEnvelope, Scenario, Tag } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const loading = ref(false)
const creating = ref(false)
const drawerOpen = ref(false)
const items = ref<Badcase[]>([])
const total = ref(0)
const scenarios = ref<Scenario[]>([])
const issueTags = ref<Tag[]>([])
const assignees = ref<Array<{ id: string; display_name: string }>>([])
const pendingFiles = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const query = reactive({
  page: 1,
  page_size: 20,
  keyword: '',
  source_type: '',
  status: '',
  validity: '',
  scenario_id: '',
  assignee_id: '',
  environment: '',
})
const createForm = reactive({
  scenario_id: '',
  title: '',
  description: '',
  agent_response_text: '',
  agent_version: '',
  environment: 'production',
  occurred_at: new Date().toISOString().slice(0, 16),
  business_reference: '',
  session_id: '',
  issue_tag_ids: [] as string[],
})
const createValid = computed(
  () =>
    createForm.scenario_id &&
    createForm.title.trim() &&
    createForm.description.trim() &&
    createForm.occurred_at &&
    createForm.issue_tag_ids.length > 0,
)

async function loadOptions() {
  try {
    const [scenarioResponse, optionResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<
        ResponseEnvelope<{
          assignees: Array<{ id: string; display_name: string }>
          issue_tags: Tag[]
        }>
      >('/api/v1/badcase-options'),
    ])
    scenarios.value = scenarioResponse.data.data.items
    assignees.value = optionResponse.data.data.assignees
    issueTags.value = optionResponse.data.data.issue_tags
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<PageData<Badcase>>>(
      '/api/v1/badcases',
      { params: query },
    )
    items.value = response.data.data.items
    total.value = response.data.data.total
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function search() {
  query.page = 1
  load()
}

function chooseFiles() {
  fileInput.value?.click()
}

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const selected = Array.from(input.files || [])
  input.value = ''
  const combined = [...pendingFiles.value, ...selected]
  if (combined.length > auth.uploadPolicy.max_files_per_owner) {
    ElMessage.warning(`最多选择 ${auth.uploadPolicy.max_files_per_owner} 张截图`)
    return
  }
  const invalid = selected.find(
    (file) =>
      !auth.uploadPolicy.allowed_media_types.includes(file.type) ||
      file.size > auth.uploadPolicy.max_file_size,
  )
  if (invalid) {
    ElMessage.warning(`文件 ${invalid.name} 的格式不支持或超过 10 MB`)
    return
  }
  pendingFiles.value = combined
}

async function createBusinessBadcase() {
  if (!createValid.value) return
  creating.value = true
  try {
    const response = await apiClient.post<ResponseEnvelope<Badcase>>(
      '/api/v1/badcases',
      {
        scenario_id: createForm.scenario_id,
        title: createForm.title,
        description: createForm.description,
        agent_response_text: createForm.agent_response_text || null,
        agent_version: createForm.agent_version || null,
        environment: createForm.environment,
        occurred_at: new Date(createForm.occurred_at).toISOString(),
        business_reference: createForm.business_reference || null,
        session_id: createForm.session_id || null,
        issue_tag_ids: createForm.issue_tag_ids,
      },
      { headers: { 'Idempotency-Key': crypto.randomUUID() } },
    )
    const item = response.data.data
    if (pendingFiles.value.length > 0) {
      const body = new FormData()
      pendingFiles.value.forEach((file) => body.append('files', file))
      body.append('expected_owner_lock_version', String(item.lock_version))
      await apiClient.post(`/api/v1/badcases/${item.id}/attachments`, body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      })
    }
    ElMessage.success('业务 Badcase 已登记')
    drawerOpen.value = false
    await router.push(`/badcases/${item.id}`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    creating.value = false
  }
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

function resetCreateForm() {
  Object.assign(createForm, {
    scenario_id: '',
    title: '',
    description: '',
    agent_response_text: '',
    agent_version: '',
    environment: 'production',
    occurred_at: new Date().toISOString().slice(0, 16),
    business_reference: '',
    session_id: '',
    issue_tag_ids: [],
  })
  pendingFiles.value = []
}

onMounted(() => Promise.all([loadOptions(), load()]))
</script>

<template>
  <section class="badcase-list-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">问题沉淀</p>
        <h1>Badcase 中心</h1>
        <p>统一记录评测与真实业务问题，保留现场证据并跟踪处理闭环。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="resetCreateForm(); drawerOpen = true">
        登记业务 Badcase
      </el-button>
    </div>

    <el-card shadow="never">
      <div class="badcase-filter-bar">
        <el-input
          v-model="query.keyword"
          clearable
          placeholder="搜索标题、描述或 Agent 回答"
          :prefix-icon="Search"
          @keyup.enter="search"
        />
        <el-select v-model="query.source_type" clearable placeholder="全部来源" @change="search">
          <el-option label="评测发现" value="evaluation" />
          <el-option label="业务登记" value="business" />
        </el-select>
        <el-select v-model="query.scenario_id" clearable placeholder="全部场景" @change="search">
          <el-option
            v-for="scenario in scenarios"
            :key="scenario.id"
            :label="scenario.name"
            :value="scenario.id"
          />
        </el-select>
        <el-select v-model="query.assignee_id" clearable placeholder="全部负责人" @change="search">
          <el-option
            v-for="user in assignees"
            :key="user.id"
            :label="user.display_name"
            :value="user.id"
          />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="全部状态" @change="search">
          <el-option label="待处理" value="pending" />
          <el-option label="处理中" value="processing" />
          <el-option label="已解决" value="resolved" />
          <el-option label="暂不处理" value="deferred" />
        </el-select>
        <el-select v-model="query.validity" clearable placeholder="有效记录" @change="search">
          <el-option label="有效记录" value="" />
          <el-option label="无效记录" value="invalid" />
          <el-option label="全部记录" value="all" />
        </el-select>
        <el-button type="primary" @click="search">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="items" row-key="id">
        <el-table-column label="Badcase" min-width="300">
          <template #default="{ row }">
            <button class="table-primary-link" @click="router.push(`/badcases/${row.id}`)">
              {{ row.title }}
            </button>
            <small>{{ row.description || row.evaluation?.user_prompt || '-' }}</small>
          </template>
        </el-table-column>
        <el-table-column label="来源" width="100">
          <template #default="{ row }">
            <el-tag :type="row.source_type === 'business' ? 'warning' : 'primary'" effect="plain">
              {{ row.source_type === 'business' ? '业务登记' : '评测发现' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="scenario_name" label="场景" min-width="130" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.invalidated_at ? 'info' : row.status === 'resolved' ? 'success' : ''">
              {{ row.invalidated_at ? '无效' : statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="问题标签" min-width="180">
          <template #default="{ row }">
            <el-tag
              v-for="tag in row.issue_tags"
              :key="tag.id"
              type="danger"
              effect="plain"
              size="small"
            >
              {{ tag.name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="负责人" width="120">
          <template #default="{ row }">{{ row.assignee_name || '未分配' }}</template>
        </el-table-column>
        <el-table-column label="发现时间" width="180">
          <template #default="{ row }">{{ formatTime(row.occurred_at) }}</template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.page_size"
        class="table-pagination"
        layout="total, prev, pager, next"
        :total="total"
        @current-change="load"
      />
    </el-card>
  </section>

  <el-drawer v-model="drawerOpen" title="登记业务 Badcase" size="620px" destroy-on-close>
    <el-form label-position="top">
      <el-form-item label="所属场景" required>
        <el-select v-model="createForm.scenario_id" filterable placeholder="选择启用场景">
          <el-option
            v-for="scenario in scenarios"
            :key="scenario.id"
            :label="`${scenario.evaluation_target_name || ''} / ${scenario.name}`"
            :value="scenario.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="问题标题" required>
        <el-input v-model="createForm.title" maxlength="200" show-word-limit />
      </el-form-item>
      <el-form-item label="问题描述" required>
        <el-input v-model="createForm.description" type="textarea" :rows="4" maxlength="5000" show-word-limit />
      </el-form-item>
      <el-form-item label="Agent 回答现场">
        <el-input v-model="createForm.agent_response_text" type="textarea" :rows="5" maxlength="20000" show-word-limit />
      </el-form-item>
      <div class="drawer-form-grid">
        <el-form-item label="Agent 版本">
          <el-input v-model="createForm.agent_version" maxlength="100" />
        </el-form-item>
        <el-form-item label="运行环境" required>
          <el-select v-model="createForm.environment">
            <el-option label="测试" value="test" />
            <el-option label="预发布" value="staging" />
            <el-option label="生产" value="production" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="发生时间" required>
          <el-input v-model="createForm.occurred_at" type="datetime-local" />
        </el-form-item>
        <el-form-item label="问题标签" required>
          <el-select v-model="createForm.issue_tag_ids" multiple placeholder="至少选择一个">
            <el-option v-for="tag in issueTags" :key="tag.id" :label="tag.name" :value="tag.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="业务单号">
          <el-input v-model="createForm.business_reference" maxlength="200" />
        </el-form-item>
        <el-form-item label="会话 ID">
          <el-input v-model="createForm.session_id" maxlength="200" />
        </el-form-item>
      </div>
      <el-form-item label="现场截图">
        <input
          ref="fileInput"
          class="visually-hidden"
          type="file"
          accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
          multiple
          @change="selectFiles"
        />
        <div class="business-upload-box" @click="chooseFiles">
          <el-icon :size="28"><UploadFilled /></el-icon>
          <span>选择截图（{{ pendingFiles.length }}/{{ auth.uploadPolicy.max_files_per_owner }}）</span>
          <small>PNG、JPG、WebP，单张不超过 10 MB</small>
        </div>
        <div class="pending-file-list">
          <el-tag
            v-for="(file, index) in pendingFiles"
            :key="`${file.name}-${index}`"
            closable
            @close="pendingFiles.splice(index, 1)"
          >
            {{ file.name }}
          </el-tag>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="drawerOpen = false">取消</el-button>
      <el-button
        type="primary"
        :loading="creating"
        :disabled="!createValid"
        @click="createBusinessBadcase"
      >
        登记 Badcase
      </el-button>
    </template>
  </el-drawer>
</template>
