<script setup lang="ts">
import { computed, ref } from 'vue'

import type { Scenario, Tag } from '@/api/types'

const props = defineProps<{
  mode: 'create' | 'edit'
  targetScenarios: Scenario[]
  availableGlobalTags: Tag[]
  availableScenarioTags: Tag[]
  savingCase: boolean
  savingMode: 'close' | 'continue' | null
}>()

const emit = defineEmits<{
  cancel: []
  save: []
  saveContinue: []
}>()

const scenarioID = defineModel<string>('scenarioId', { required: true })
const caseName = defineModel<string>('name', { required: true })
const userPrompt = defineModel<string>('userPrompt', { required: true })
const precondition = defineModel<string>('precondition', { required: true })
const expectedResult = defineModel<string>('expectedResult', { required: true })
const judgingGuide = defineModel<string>('judgingGuide', { required: true })
const tagIDs = defineModel<string[]>('tagIds', { required: true })
const isEnabled = defineModel<boolean>('isEnabled', { required: true })
const extrasOpen = ref<string[]>([])
const selectedScenarioName = computed(
  () => props.targetScenarios.find((item) => item.id === scenarioID.value)?.name || '',
)
</script>

<template>
  <el-form label-position="top" @submit.prevent>
    <div class="inline-case-main-grid">
      <el-form-item class="inline-case-prompt" label="用户输入" required>
        <el-input
          v-model="userPrompt"
          type="textarea"
          :rows="6"
          resize="vertical"
          placeholder="输入要发送给 Agent 的问题或任务指令"
        />
        <span class="inline-form-help">
          {{ mode === 'create' ? '这是新增评测用例唯一必填的信息。' : '用户输入是评测用例唯一必填的信息。' }}
        </span>
      </el-form-item>
      <div class="inline-case-options">
        <el-form-item label="用例名称（可选）">
          <el-input
            v-model="caseName"
            maxlength="200"
            placeholder="留空时使用用户输入摘要"
          />
        </el-form-item>
        <el-form-item label="场景归类（可选）">
          <el-select v-model="scenarioID" clearable filterable placeholder="暂不归类">
            <el-option
              v-for="scenario in targetScenarios"
              :key="scenario.id"
              :label="scenario.name"
              :value="scenario.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="用例标签（可选）">
          <el-select v-model="tagIDs" multiple clearable placeholder="暂不添加标签">
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
              :label="`场景标签 · ${selectedScenarioName}`"
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
      </div>
    </div>
    <el-collapse v-model="extrasOpen" class="inline-case-extras">
      <el-collapse-item title="补充评测信息（可选）" name="extra">
        <div class="inline-case-extra-grid">
          <el-form-item label="前置条件">
            <el-input v-model="precondition" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item label="期望结果">
            <el-input v-model="expectedResult" type="textarea" :rows="3" />
          </el-form-item>
          <el-form-item label="评判要点">
            <el-input v-model="judgingGuide" type="textarea" :rows="3" />
          </el-form-item>
        </div>
      </el-collapse-item>
    </el-collapse>
    <div class="inline-case-actions">
      <el-switch
        v-model="isEnabled"
        :active-text="mode === 'create' ? '新增后启用' : '启用此用例'"
      />
      <div>
        <span v-if="mode === 'create'">“添加并继续”会保留场景和标签</span>
        <el-button :disabled="savingCase" @click="emit('cancel')">取消</el-button>
        <el-button
          v-if="mode === 'create'"
          :loading="savingMode === 'continue'"
          :disabled="!userPrompt.trim() || savingCase"
          @click="emit('saveContinue')"
        >
          添加并继续
        </el-button>
        <el-button
          type="primary"
          :loading="savingMode === 'close'"
          :disabled="!userPrompt.trim() || savingCase"
          @click="emit('save')"
        >
          {{ mode === 'create' ? '添加用例' : '保存修改' }}
        </el-button>
      </div>
    </div>
  </el-form>
</template>
