<script setup lang="ts">
import { ElMessage, ElMessageBox } from 'element-plus'
import { onMounted, reactive, ref } from 'vue'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { PageData, ResponseEnvelope, User } from '@/api/types'

const users = ref<User[]>([])
const loading = ref(false)
const dialog = ref(false)
const editing = ref<User | null>(null)
const form = reactive({
  username: '',
  display_name: '',
  email: '',
  role: 'member' as 'admin' | 'member',
})

async function load() {
  loading.value = true
  try {
    const response =
      await apiClient.get<ResponseEnvelope<PageData<User>>>('/api/v1/users?page_size=100')
    users.value = response.data.data.items
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  Object.assign(form, { username: '', display_name: '', email: '', role: 'member' })
  dialog.value = true
}

function openEdit(user: User) {
  editing.value = user
  Object.assign(form, {
    username: user.username,
    display_name: user.display_name,
    email: user.email || '',
    role: user.role,
  })
  dialog.value = true
}

async function save() {
  try {
    if (editing.value) {
      await apiClient.put(`/api/v1/users/${editing.value.id}`, {
        display_name: form.display_name,
        email: form.email || null,
        role: form.role,
        expected_lock_version: editing.value.lock_version,
      })
    } else {
      await apiClient.post('/api/v1/users', {
        username: form.username,
        display_name: form.display_name,
        role: form.role,
        email: form.email || null,
      })
    }
    dialog.value = false
    ElMessage.success(editing.value ? '用户已保存' : '用户已创建，初始密码为 123456')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function toggle(user: User) {
  try {
    const action = user.status === 'active' ? 'disable' : 'enable'
    await apiClient.post(`/api/v1/users/${user.id}/${action}`, {
      expected_lock_version: user.lock_version,
    })
    ElMessage.success(action === 'disable' ? '用户已停用' : '用户已启用')
    await load()
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function resetPassword(user: User) {
  try {
    await ElMessageBox.confirm(`将 ${user.display_name} 的密码重置为 123456。该用户下次登录时必须设置新密码。`, '重置为初始密码', {
      type: 'warning',
      confirmButtonText: '确认重置',
      cancelButtonText: '取消',
    })
    await apiClient.post(`/api/v1/users/${user.id}/reset-password`)
    ElMessage.success('密码已重置为 123456，原会话已失效')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(apiErrorMessage(error))
    }
  }
}

onMounted(load)
</script>

<template>
  <section class="management-page">
    <el-card shadow="never">
      <div class="content-primary-actions">
        <el-button type="primary" @click="openCreate">新建用户</el-button>
      </div>
      <el-table v-loading="loading" :data="users" empty-text="暂无用户">
        <el-table-column prop="display_name" label="用户" min-width="150">
          <template #default="{ row }">
            <strong>{{ row.display_name }}</strong>
            <div class="table-secondary">{{ row.username }} · {{ row.email || '未填写邮箱' }}</div>
            <el-tag v-if="row.password_change_required" size="small" type="warning">待设置密码</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="role" label="角色" width="110">
          <template #default="{ row }">{{ row.role === 'admin' ? '管理员' : '成员' }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'">
              {{ row.status === 'active' ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="270" align="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link @click="resetPassword(row)">重置为初始密码</el-button>
            <el-button link :type="row.status === 'active' ? 'danger' : 'success'" @click="toggle(row)">
              {{ row.status === 'active' ? '停用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </section>

  <el-dialog v-model="dialog" :title="editing ? '编辑用户' : '新建用户'" width="520">
    <el-form label-position="top">
      <el-form-item label="用户名">
        <el-input v-model="form.username" :disabled="Boolean(editing)" />
      </el-form-item>
      <el-form-item label="显示名称"><el-input v-model="form.display_name" /></el-form-item>
      <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
      <el-form-item label="角色">
        <el-radio-group v-model="form.role">
          <el-radio-button value="member">成员</el-radio-button>
          <el-radio-button value="admin">管理员</el-radio-button>
        </el-radio-group>
      </el-form-item>
      <el-alert
        v-if="!editing"
        title="初始密码为 123456"
        description="用户首次登录后必须设置新密码，才能继续使用系统。"
        type="info"
        :closable="false"
        show-icon
      />
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>
