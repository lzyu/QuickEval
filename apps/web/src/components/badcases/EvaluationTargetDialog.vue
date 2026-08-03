<script setup lang="ts">
import { ArrowRight, Box, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, ref, watch } from 'vue'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { CatalogItem, PageData, ResponseEnvelope, Scenario } from '@/api/types'
import { targetChoices } from '@/features/badcases/registration'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    required?: boolean
    currentTargetId?: string
  }>(),
  { required: false, currentTargetId: '' },
)
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  select: [targetId: string]
  closeRequired: []
}>()

const loading = ref(false)
const loaded = ref(false)
const selectionMade = ref(false)
const keyword = ref('')
const showDisabled = ref(false)
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const choices = computed(() => targetChoices(targets.value, scenarios.value))
const activeChoiceCount = computed(
  () => choices.value.filter((target) => target.status === 'active').length,
)
const disabledChoiceCount = computed(() => choices.value.length - activeChoiceCount.value)
const filteredChoices = computed(() => {
  const query = keyword.value.trim().toLocaleLowerCase()
  return choices.value.filter((target) => {
    if (!showDisabled.value && target.status !== 'active') return false
    if (!query) return true
    return [target.name, target.description || ''].some((value) =>
      value.toLocaleLowerCase().includes(query),
    )
  })
})

async function load() {
  if (loaded.value) return
  loading.value = true
  try {
    const [targetResponse, scenarioResponse] = await Promise.all([
      apiClient.get<ResponseEnvelope<PageData<CatalogItem>>>('/api/v1/evaluation-targets', {
        params: { page: 1, page_size: 100 },
      }),
      apiClient.get<ResponseEnvelope<PageData<Scenario>>>('/api/v1/scenarios', {
        params: { page: 1, page_size: 100 },
      }),
    ])
    targets.value = targetResponse.data.data.items
    scenarios.value = scenarioResponse.data.data.items
    loaded.value = true
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    loading.value = false
  }
}

function close() {
  emit('update:modelValue', false)
  if (props.required && !selectionMade.value) emit('closeRequired')
}

function selectTarget(target: (typeof choices.value)[number]) {
  if (target.status !== 'active') return
  selectionMade.value = true
  emit('select', target.id)
  emit('update:modelValue', false)
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      selectionMade.value = false
      keyword.value = ''
      showDisabled.value = false
      void load()
    }
  },
  { immediate: true },
)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    class="target-picker-dialog"
    width="800px"
    :close-on-click-modal="!required"
    :close-on-press-escape="!required"
    :show-close="!required"
    @close="close"
  >
    <template #header>
      <div class="target-picker-heading">
        <h2>选择评测对象</h2>
        <p>选择本次主动登记 Badcase 所属的 Agent，进入后可连续登记多条问题。</p>
      </div>
    </template>

    <div class="target-picker-controls">
      <el-input
        v-model="keyword"
        class="target-picker-search"
        clearable
        :prefix-icon="Search"
        placeholder="搜索评测对象"
        aria-label="搜索评测对象"
      />
      <el-checkbox
        v-if="disabledChoiceCount"
        v-model="showDisabled"
        class="target-picker-disabled-toggle"
      >
        显示已停用（{{ disabledChoiceCount }}）
      </el-checkbox>
    </div>

    <div class="target-picker-summary" aria-live="polite">
      <strong>可登记 {{ activeChoiceCount }} 个对象</strong>
      <span v-if="keyword.trim()">匹配 {{ filteredChoices.length }} 个</span>
      <span v-else>选择对象后即可开始连续登记</span>
    </div>

    <div v-loading="loading" class="target-choice-list" aria-label="评测对象列表">
      <el-empty
        v-if="!loading && filteredChoices.length === 0"
        :description="showDisabled ? '没有匹配的评测对象' : '没有匹配的可登记对象'"
        :image-size="72"
      />
      <button
        v-for="target in filteredChoices"
        :key="target.id"
        class="target-choice"
        :class="{
          selected: target.id === currentTargetId,
          disabled: target.status !== 'active',
        }"
        type="button"
        :disabled="target.status !== 'active'"
        @click="selectTarget(target)"
      >
        <span class="target-choice-icon"><el-icon :size="21"><Box /></el-icon></span>
        <span class="target-choice-copy">
          <strong>{{ target.name }}</strong>
          <span>{{ target.description || '暂无对象描述' }}</span>
        </span>
        <span class="target-choice-meta">
          <span v-if="target.status !== 'active'" class="target-choice-status disabled">已停用</span>
          <span v-else-if="target.availableScenarioCount === 0" class="target-choice-status pending">
            暂无场景 · 待归类
          </span>
          <span v-else class="target-choice-status">
            {{ target.availableScenarioCount }} 个可用场景
          </span>
          <small v-if="target.id === currentTargetId">当前对象</small>
        </span>
        <el-icon v-if="target.status === 'active'" class="target-choice-action"><ArrowRight /></el-icon>
      </button>
    </div>

    <template #footer>
      <el-button @click="close">取消</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
:global(.target-picker-dialog) {
  border-radius: 14px;
  box-shadow: 0 20px 52px rgb(20 36 64 / 20%);
}

:global(.target-picker-dialog .el-dialog__header) {
  padding: 24px 28px 10px;
}

:global(.target-picker-dialog .el-dialog__body) {
  padding: 8px 28px 16px;
}

:global(.target-picker-dialog .el-dialog__footer) {
  padding: 0 28px 20px;
}

.target-picker-heading h2 {
  margin: 0 0 8px;
  color: #1d2939;
  font-size: 20px;
  font-weight: 600;
}

.target-picker-heading p {
  margin: 0;
  color: #667085;
  font-size: 14px;
}

.target-picker-controls {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 14px 0 10px;
}

.target-picker-search {
  flex: 1;
}

.target-picker-disabled-toggle {
  flex: 0 0 auto;
  color: #667085;
  font-size: 13px;
}

.target-picker-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 30px;
  color: #667085;
  font-size: 13px;
}

.target-picker-summary strong {
  color: #344054;
  font-size: 14px;
}

.target-picker-summary span::before {
  margin-right: 10px;
  color: #c0c7d1;
  content: '·';
}

.target-choice-list {
  display: grid;
  max-height: min(420px, calc(100vh - 300px));
  min-height: 248px;
  gap: 2px;
  overflow-y: auto;
  padding: 2px;
  border-top: 1px solid #e4e9f2;
  border-bottom: 1px solid #e4e9f2;
}

.target-choice {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr) max-content 20px;
  align-items: center;
  min-height: 76px;
  gap: 14px;
  padding: 14px 16px;
  border: 1px solid transparent;
  border-radius: 8px;
  color: #344054;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background-color 160ms ease, border-color 160ms ease;
}

.target-choice:hover,
.target-choice:focus-visible,
.target-choice.selected {
  border-color: #2878e8;
  outline: 0;
  background: #f4f8ff;
}

.target-choice.disabled {
  color: #7b8492;
  background: #fafbfc;
  cursor: not-allowed;
}

.target-choice-icon {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border-radius: 10px;
  color: #fff;
  background: #2375e7;
}

.target-choice.disabled .target-choice-icon {
  background: #98a2b3;
  box-shadow: none;
}

.target-choice-copy strong,
.target-choice-copy span,
.target-choice-copy small {
  display: block;
}

.target-choice-copy strong {
  margin-bottom: 4px;
  color: #1d2939;
  font-size: 15px;
  font-weight: 600;
}

.target-choice-copy span {
  overflow: hidden;
  color: #667085;
  font-size: 13px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-choice-meta {
  display: grid;
  justify-items: end;
  gap: 5px;
  color: #667085;
  font-size: 12px;
}

.target-choice-meta small {
  color: #1769e0;
  font-size: 12px;
  font-weight: 600;
}

.target-choice-status {
  min-height: 24px;
  padding: 3px 8px;
  border-radius: 999px;
  color: #1f6a43;
  background: #eef8f0;
  line-height: 18px;
  white-space: nowrap;
}

.target-choice-status.pending {
  color: #8b5d10;
  background: #fff7e8;
}

.target-choice-status.disabled {
  color: #667085;
  background: #eef1f5;
}

.target-choice-action {
  color: #1769e0;
  font-size: 18px;
}

@media (max-width: 600px) {
  :global(.target-picker-dialog) {
    width: calc(100% - 24px) !important;
    margin-top: 72px;
  }

  :global(.target-picker-dialog .el-dialog__header),
  :global(.target-picker-dialog .el-dialog__body),
  :global(.target-picker-dialog .el-dialog__footer) {
    padding-right: 18px;
    padding-left: 18px;
  }

  .target-picker-controls {
    align-items: stretch;
    flex-direction: column;
    gap: 10px;
  }

  .target-choice-list {
    max-height: min(440px, calc(100vh - 290px));
  }

  .target-choice {
    grid-template-columns: 40px minmax(0, 1fr) 18px;
    min-height: 70px;
    gap: 10px;
    padding: 12px;
  }

  .target-choice-icon {
    width: 40px;
    height: 40px;
  }

  .target-choice-meta {
    display: none;
  }
}
</style>
