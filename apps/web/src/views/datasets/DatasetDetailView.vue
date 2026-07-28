<script setup lang="ts">
import { Document, EditPen, Files, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type {
  Dataset,
  DatasetDetail,
  DatasetVersion,
  ResponseEnvelope,
} from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const dataset = ref<Dataset | null>(null)
const versions = ref<DatasetVersion[]>([])
const editOpen = ref(false)
const editSaving = ref(false)
const editForm = reactive({ name: '', description: '' })
const startOpen = ref(false)
const startSaving = ref(false)
const startVersion = ref<DatasetVersion | null>(null)
const startForm = reactive({
  agent_version: '',
  environment: 'staging',
  purpose_note: '',
  config_note: '',
})

const draft = computed(() => versions.value.find((item) => item.status === 'draft'))
const releasedVersions = computed(() =>
  versions.value.filter((item) => item.status !== 'draft'),
)

async function load() {
  loading.value = true
  try {
    const response = await apiClient.get<ResponseEnvelope<DatasetDetail>>(
      `/api/v1/datasets/${route.params.datasetId}`,
    )
    dataset.value = response.data.data.dataset
    versions.value = response.data.data.versions
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

async function createDraft(base?: DatasetVersion) {
  if (!dataset.value) return
  if (draft.value) {
    await router.push(`/dataset-versions/${draft.value.id}/edit`)
    return
  }
  try {
    const response = await apiClient.post<ResponseEnvelope<DatasetVersion>>(
      `/api/v1/datasets/${dataset.value.id}/drafts`,
      {
        base_version_id: base?.id || null,
        expected_dataset_lock_version: dataset.value.lock_version,
      },
    )
    ElMessage.success(base ? `已从 V${base.version_no} 复制草稿` : '空白草稿已创建')
    await router.push(`/dataset-versions/${response.data.data.id}/edit`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function archiveVersion(version: DatasetVersion) {
  try {
    await ElMessageBox.confirm(
      `归档 V${version.version_no} 后，该版本不能用于开始新的评测。`,
      '归档版本',
      { type: 'warning' },
    )
    await apiClient.post(`/api/v1/dataset-versions/${version.id}/archive`, {
      expected_lock_version: version.lock_version,
    })
    ElMessage.success(`V${version.version_no} 已归档`)
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(apiErrorMessage(error))
  }
}

async function toggleDataset() {
  if (!dataset.value) return
  const action = dataset.value.status === 'active' ? 'archive' : 'restore'
  try {
    await apiClient.post(`/api/v1/datasets/${dataset.value.id}/${action}`, {
      expected_lock_version: dataset.value.lock_version,
    })
    ElMessage.success(action === 'archive' ? '评测集已归档' : '评测集已恢复')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

function exportVersion(version: DatasetVersion) {
  window.open(`/api/v1/dataset-versions/${version.id}/cases.csv`, '_blank')
}

function openStart(version?: DatasetVersion) {
  const selected =
    version ||
    releasedVersions.value.find((item) => item.status === 'published') ||
    null
  if (!selected) {
    ElMessage.warning('暂无可开始评测的已发布版本')
    return
  }
  startVersion.value = selected
  Object.assign(startForm, {
    agent_version: '',
    environment: 'staging',
    purpose_note: '',
    config_note: '',
  })
  startOpen.value = true
}

async function startEvaluation() {
  if (!startVersion.value) return
  startSaving.value = true
  try {
    const response = await apiClient.post<ResponseEnvelope<{ id: string }>>(
      '/api/v1/evaluation-runs',
      {
        dataset_version_id: startVersion.value.id,
        agent_version: startForm.agent_version,
        environment: startForm.environment,
        purpose_note: startForm.purpose_note || null,
        config_note: startForm.config_note || null,
      },
      { headers: { 'Idempotency-Key': crypto.randomUUID() } },
    )
    startOpen.value = false
    ElMessage.success('独立评测已创建')
    await router.push(`/evaluation-runs/${response.data.data.id}/workbench`)
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    startSaving.value = false
  }
}

function openEdit() {
  if (!dataset.value) return
  editForm.name = dataset.value.name
  editForm.description = dataset.value.description || ''
  editOpen.value = true
}

async function saveDataset() {
  if (!dataset.value) return
  editSaving.value = true
  try {
    await apiClient.patch(`/api/v1/datasets/${dataset.value.id}`, {
      scenario_id: dataset.value.scenario_id,
      name: editForm.name,
      description: editForm.description || null,
      expected_lock_version: dataset.value.lock_version,
    })
    editOpen.value = false
    ElMessage.success('评测集基本信息已更新')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    editSaving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="dataset-detail-page">
    <template v-if="dataset">
      <el-breadcrumb separator="/">
        <el-breadcrumb-item :to="{ path: '/datasets' }">评测集</el-breadcrumb-item>
        <el-breadcrumb-item>{{ dataset.evaluation_target_name }}</el-breadcrumb-item>
        <el-breadcrumb-item>{{ dataset.scenario_name }}</el-breadcrumb-item>
        <el-breadcrumb-item>{{ dataset.name }}</el-breadcrumb-item>
      </el-breadcrumb>

      <el-card class="dataset-hero-card" shadow="never">
        <div class="dataset-detail-heading">
          <div>
            <div class="title-line">
              <h1>{{ dataset.name }}</h1>
              <el-tag :type="dataset.status === 'active' ? 'success' : 'info'">
                {{ dataset.status === 'active' ? '活跃' : '已归档' }}
              </el-tag>
            </div>
            <p>{{ dataset.description || '暂无说明' }}</p>
          </div>
          <div class="heading-actions">
            <el-button
              v-if="auth.isAdmin && draft"
              type="primary"
              :icon="EditPen"
              @click="router.push(`/dataset-versions/${draft.id}/edit`)"
            >
              编辑草稿
            </el-button>
            <el-button
              v-else-if="auth.isAdmin && dataset.status === 'active'"
              type="primary"
              :icon="Plus"
              @click="createDraft(releasedVersions[0])"
            >
              创建草稿
            </el-button>
            <el-button
              :disabled="!releasedVersions.some((item) => item.status === 'published')"
              @click="openStart()"
            >
              开始评测
            </el-button>
            <el-dropdown v-if="auth.isAdmin">
              <el-button>更多</el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openEdit">编辑基本信息</el-dropdown-item>
                  <el-dropdown-item @click="toggleDataset">
                    {{ dataset.status === 'active' ? '归档评测集' : '恢复评测集' }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>

        <div class="dataset-metrics">
          <div>
            <el-icon><Document /></el-icon>
            <span>已发布版本</span>
            <strong>{{ dataset.published_version_count }} 个</strong>
          </div>
          <div>
            <el-icon><EditPen /></el-icon>
            <span>当前草稿</span>
            <strong>{{ draft ? `${draft.case_count} 条用例` : '无草稿' }}</strong>
          </div>
          <div>
            <el-icon><Files /></el-icon>
            <span>最新版本</span>
            <strong>{{ dataset.latest_version_no ? `V${dataset.latest_version_no}` : '尚未发布' }}</strong>
          </div>
        </div>
      </el-card>

      <el-card class="version-card" shadow="never">
        <div class="section-title">
          <div>
            <p class="eyebrow">VERSION HISTORY</p>
            <h2>版本记录</h2>
          </div>
          <el-button
            v-if="auth.isAdmin && !draft && dataset.status === 'active'"
            :icon="Plus"
            @click="createDraft(releasedVersions[0])"
          >
            从最新版本创建草稿
          </el-button>
        </div>

        <div class="version-timeline">
          <article v-if="draft" class="version-item draft-version">
            <span class="timeline-dot warning"></span>
            <div>
              <div class="version-title">
                <h3>当前草稿</h3>
                <el-tag type="warning">未发布</el-tag>
              </div>
              <p>
                {{ draft.base_version_id ? '基于已发布版本复制' : '空白草稿' }}
                · {{ draft.case_count }} 条用例 · 更新于
                {{ new Date(draft.updated_at).toLocaleString() }}
              </p>
            </div>
            <el-button type="warning" plain @click="router.push(`/dataset-versions/${draft.id}/edit`)">
              编辑草稿
            </el-button>
          </article>

          <article
            v-for="version in releasedVersions"
            :key="version.id"
            class="version-item"
          >
            <span class="timeline-dot" :class="{ archived: version.status === 'archived' }"></span>
            <div>
              <div class="version-title">
                <h3>V{{ version.version_no }}</h3>
                <el-tag :type="version.status === 'published' ? 'success' : 'info'">
                  {{ version.status === 'published' ? '已发布' : '已归档' }}
                </el-tag>
              </div>
              <p>
                发布时间 {{ version.published_at ? new Date(version.published_at).toLocaleString() : '-' }}
                · {{ version.enabled_count }} 条启用用例
              </p>
              <p class="release-note">发布说明：{{ version.release_note || '无' }}</p>
            </div>
            <div class="version-actions">
              <el-button @click="exportVersion(version)">导出 CSV</el-button>
              <el-button
                v-if="version.status === 'published'"
                type="primary"
                plain
                @click="openStart(version)"
              >
                开始评测
              </el-button>
              <el-button
                v-if="auth.isAdmin && !draft && dataset.status === 'active'"
                @click="createDraft(version)"
              >
                复制草稿
              </el-button>
              <el-button
                v-if="auth.isAdmin && version.status === 'published'"
                link
                @click="archiveVersion(version)"
              >
                归档
              </el-button>
            </div>
          </article>

          <el-empty v-if="!versions.length" description="尚无版本记录" />
        </div>
      </el-card>
    </template>
  </section>

  <el-dialog v-model="editOpen" title="编辑评测集基本信息" width="560">
    <el-form label-position="top">
      <el-form-item label="所属场景">
        <el-input :model-value="`${dataset?.evaluation_target_name} / ${dataset?.scenario_name}`" disabled />
        <div class="muted">发布版本后不允许更换场景。</div>
      </el-form-item>
      <el-form-item label="评测集名称" required>
        <el-input v-model="editForm.name" maxlength="200" show-word-limit />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="editForm.description" type="textarea" :rows="4" maxlength="10000" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button
        type="primary"
        :loading="editSaving"
        :disabled="!editForm.name.trim()"
        @click="saveDataset"
      >
        保存
      </el-button>
    </template>
  </el-dialog>

  <el-drawer v-model="startOpen" title="开始评测" size="500">
    <el-descriptions v-if="dataset && startVersion" :column="1" border>
      <el-descriptions-item label="评测集">{{ dataset.name }}</el-descriptions-item>
      <el-descriptions-item label="版本">V{{ startVersion.version_no }}</el-descriptions-item>
      <el-descriptions-item label="场景">
        {{ dataset.evaluation_target_name }} / {{ dataset.scenario_name }}
      </el-descriptions-item>
      <el-descriptions-item label="启用用例">{{ startVersion.enabled_count }} 条</el-descriptions-item>
    </el-descriptions>
    <el-form class="start-run-form" label-position="top">
      <el-form-item label="Agent 版本" required>
        <el-input
          v-model="startForm.agent_version"
          maxlength="100"
          placeholder="例如：2026.07.3"
        />
      </el-form-item>
      <el-form-item label="运行环境" required>
        <el-select v-model="startForm.environment">
          <el-option label="测试" value="test" />
          <el-option label="预发布" value="staging" />
          <el-option label="生产" value="production" />
          <el-option label="其他" value="other" />
        </el-select>
      </el-form-item>
      <el-form-item label="评测说明">
        <el-input
          v-model="startForm.purpose_note"
          type="textarea"
          :rows="4"
          placeholder="例如：提示词优化后复测"
        />
      </el-form-item>
      <el-form-item label="配置备注">
        <el-input
          v-model="startForm.config_note"
          type="textarea"
          :rows="4"
          placeholder="例如：知识库版本、提示词版本"
        />
      </el-form-item>
      <el-alert
        title="开始后会为全部启用用例创建独立结果，可随时保存并继续。"
        type="info"
        :closable="false"
      />
    </el-form>
    <template #footer>
      <el-button @click="startOpen = false">取消</el-button>
      <el-button
        type="primary"
        :loading="startSaving"
        :disabled="!startForm.agent_version.trim()"
        @click="startEvaluation"
      >
        开始评测
      </el-button>
    </template>
  </el-drawer>
</template>
