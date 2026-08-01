<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'

import { apiClient } from '@/api/client'
import type { AuditLog, PageData, ResponseEnvelope } from '@/api/types'

const items = ref<AuditLog[]>([])
const filters = reactive({ action: '', entity_type: '' })

async function load() {
  const response = await apiClient.get<ResponseEnvelope<PageData<AuditLog>>>('/api/v1/audit-logs', {
    params: { page_size: 100, ...filters },
  })
  items.value = response.data.data.items
}

onMounted(load)
</script>

<template>
  <section class="management-page">
    <el-card shadow="never">
      <el-form inline class="filter-row" @submit.prevent="load">
        <el-form-item label="动作"><el-input v-model="filters.action" clearable /></el-form-item>
        <el-form-item label="资源类型">
          <el-input v-model="filters.entity_type" clearable />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit">查询</el-button>
        </el-form-item>
      </el-form>
      <el-table :data="items" empty-text="暂无审计记录">
        <el-table-column prop="created_at" label="时间" width="190" />
        <el-table-column prop="action" label="动作" min-width="170" />
        <el-table-column prop="entity_type" label="资源类型" width="130" />
        <el-table-column prop="entity_id" label="资源 ID" min-width="250" />
        <el-table-column prop="request_id" label="请求 ID" min-width="250" />
        <el-table-column label="变更" width="90">
          <template #default="{ row }">
            <el-popover placement="left" :width="520" trigger="click">
              <pre class="audit-json">{{ JSON.stringify({ before: row.before_data, after: row.after_data }, null, 2) }}</pre>
              <template #reference><el-button link type="primary">查看</el-button></template>
            </el-popover>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>
</template>
