<script setup lang="ts">
import { Box, Search } from '@element-plus/icons-vue'
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
const targets = ref<CatalogItem[]>([])
const scenarios = ref<Scenario[]>([])
const choices = computed(() => targetChoices(targets.value, scenarios.value))
const filteredChoices = computed(() => {
  const query = keyword.value.trim().toLocaleLowerCase()
  if (!query) return choices.value
  return choices.value.filter((target) => target.name.toLocaleLowerCase().includes(query))
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
    width="720px"
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

    <el-input
      v-model="keyword"
      class="target-picker-search"
      clearable
      :prefix-icon="Search"
      placeholder="搜索评测对象"
    />

    <div v-loading="loading" class="target-choice-grid">
      <el-empty
        v-if="!loading && filteredChoices.length === 0"
        description="没有匹配的评测对象"
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
        <span class="target-choice-icon"><el-icon :size="25"><Box /></el-icon></span>
        <span class="target-choice-copy">
          <strong>{{ target.name }}</strong>
          <span>{{ target.description || '暂无对象描述' }}</span>
          <small v-if="target.status !== 'active'">对象已停用</small>
          <small v-else-if="target.availableScenarioCount === 0">暂无场景，仍可先登记为待归类</small>
          <small v-else>{{ target.availableScenarioCount }} 个可用场景，可稍后归类</small>
        </span>
        <span v-if="target.status === 'active'" class="target-choice-action">
          进入登记 <span aria-hidden="true">→</span>
        </span>
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
  box-shadow: 0 24px 64px rgb(20 36 64 / 24%);
}

:global(.target-picker-dialog .el-dialog__header) {
  padding: 24px 28px 8px;
}

:global(.target-picker-dialog .el-dialog__body) {
  padding: 8px 28px 20px;
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

.target-picker-search {
  width: 305px;
  margin: 12px 0 18px;
}

.target-choice-grid {
  display: grid;
  min-height: 250px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  align-content: start;
}

.target-choice-grid :deep(.el-empty) {
  grid-column: 1 / -1;
}

.target-choice {
  position: relative;
  display: grid;
  min-height: 168px;
  grid-template-columns: 54px minmax(0, 1fr);
  gap: 14px;
  padding: 22px;
  border: 1px solid #cfd9e8;
  border-radius: 11px;
  color: #344054;
  background: #fff;
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}

.target-choice:hover,
.target-choice:focus-visible,
.target-choice.selected {
  border-color: #2878e8;
  outline: 0;
  box-shadow: 0 8px 22px rgb(23 105 224 / 12%);
  transform: translateY(-1px);
}

.target-choice.disabled {
  border-style: dashed;
  color: #7b8492;
  background: #fafbfc;
  box-shadow: none;
  cursor: not-allowed;
  transform: none;
}

.target-choice-icon {
  display: grid;
  width: 54px;
  height: 54px;
  place-items: center;
  border-radius: 12px;
  color: #fff;
  background: #2375e7;
  box-shadow: 0 8px 18px rgb(35 117 231 / 22%);
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
  margin: 2px 0 7px;
  color: #1d2939;
  font-size: 16px;
  font-weight: 600;
}

.target-choice-copy span {
  display: -webkit-box;
  overflow: hidden;
  min-height: 42px;
  color: #667085;
  font-size: 13px;
  line-height: 1.6;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.target-choice-copy small {
  margin-top: 8px;
  color: #475467;
  font-size: 13px;
}

.target-choice-action {
  position: absolute;
  right: 22px;
  bottom: 18px;
  color: #1769e0;
  font-size: 14px;
  font-weight: 600;
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

  .target-picker-search {
    width: 100%;
  }

  .target-choice-grid {
    grid-template-columns: 1fr;
  }

  .target-choice {
    min-height: 140px;
    padding: 18px;
  }
}
</style>
