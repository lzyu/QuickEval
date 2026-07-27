<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { ResponseEnvelope, Tag } from '@/api/types'

const items = ref<Tag[]>([])
const dialog = ref(false)
const editing = ref<Tag | null>(null)
const form = reactive({ name: '', description: '' })

async function load() {
  const response =
    await apiClient.get<ResponseEnvelope<{ items: Tag[] }>>('/api/v1/issue-tags')
  items.value = response.data.data.items
}

function open(item?: Tag) {
  editing.value = item || null
  form.name = item?.name || ''
  form.description = item?.description || ''
  dialog.value = true
}

async function save() {
  const path = editing.value ? `/api/v1/issue-tags/${editing.value.id}` : '/api/v1/issue-tags'
  try {
    await apiClient.request({
      method: editing.value ? 'put' : 'post',
      url: path,
      data: {
        name: form.name,
        description: form.description || null,
        expected_lock_version: editing.value?.lock_version || 0,
      },
    })
    dialog.value = false
    ElMessage.success('问题标签已保存')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function toggle(item: Tag) {
  try {
    await apiClient.post(
      `/api/v1/issue-tags/${item.id}/${item.status === 'active' ? 'disable' : 'enable'}`,
      { expected_lock_version: item.lock_version },
    )
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function move(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= items.value.length) return
  const reordered = [...items.value]
  const current = reordered[index]
  const neighbor = reordered[target]
  if (!current || !neighbor) return
  reordered[index] = neighbor
  reordered[target] = current
  await apiClient.put('/api/v1/issue-tags/reorder', {
    items: reordered.map((item, order) => ({
      id: item.id,
      sort_order: order,
      expected_lock_version: item.lock_version,
    })),
  })
  await load()
}

onMounted(load)
</script>

<template>
  <section class="management-page">
    <div class="page-heading">
      <div>
        <p class="eyebrow">BADCASE TAXONOMY</p>
        <h1>问题标签</h1>
        <p>预设 Badcase 问题分类，用于后续分布统计与定位。</p>
      </div>
      <el-button type="primary" @click="open()">新建标签</el-button>
    </div>
    <el-card shadow="never">
      <el-table :data="items" empty-text="暂无问题标签">
        <el-table-column type="index" label="排序" width="80" />
        <el-table-column prop="name" label="标签名称" min-width="180" />
        <el-table-column prop="description" label="说明" min-width="260" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" align="right">
          <template #default="{ row, $index }">
            <el-button link :disabled="$index === 0" @click="move($index, -1)">上移</el-button>
            <el-button link :disabled="$index === items.length - 1" @click="move($index, 1)">
              下移
            </el-button>
            <el-button link type="primary" @click="open(row)">编辑</el-button>
            <el-button link @click="toggle(row)">
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>

  <el-dialog v-model="dialog" :title="editing ? '编辑问题标签' : '新建问题标签'" width="500">
    <el-form label-position="top">
      <el-form-item label="名称"><el-input v-model="form.name" /></el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="3" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>
