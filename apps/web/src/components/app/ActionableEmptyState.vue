<script setup lang="ts">
import { Compass } from '@element-plus/icons-vue'

withDefaults(
  defineProps<{
    title: string
    description: string
    actionLabel?: string
    secondaryLabel?: string
    compact?: boolean
  }>(),
  {
    actionLabel: '',
    secondaryLabel: '',
    compact: false,
  },
)

defineEmits<{
  action: []
  secondary: []
}>()
</script>

<template>
  <div class="actionable-empty-state" :class="{ compact }" role="status">
    <span class="actionable-empty-icon" aria-hidden="true">
      <el-icon><Compass /></el-icon>
    </span>
    <strong>{{ title }}</strong>
    <p>{{ description }}</p>
    <div v-if="actionLabel || secondaryLabel" class="actionable-empty-actions">
      <el-button v-if="actionLabel" type="primary" @click="$emit('action')">
        {{ actionLabel }}
      </el-button>
      <el-button v-if="secondaryLabel" @click="$emit('secondary')">
        {{ secondaryLabel }}
      </el-button>
    </div>
  </div>
</template>
