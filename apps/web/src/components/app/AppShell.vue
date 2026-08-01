<script setup lang="ts">
import {
  ArrowDown,
  Collection,
  DataAnalysis,
  HomeFilled,
  List,
  Operation,
  Setting,
  Search as SearchIcon,
  Tickets,
  User,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { apiClient, apiErrorMessage } from '@/api/client'
import type { ResponseEnvelope, SearchItem } from '@/api/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const passwordDialog = ref(false)
const searchKeyword = ref('')
const searchLoading = ref(false)
const searchOpen = ref(false)
const searchItems = ref<SearchItem[]>([])
let searchTimer: ReturnType<typeof setTimeout> | null = null
const passwordForm = reactive({ current_password: '', new_password: '' })
const navigation = [
  { section: 'home', label: '首页', icon: HomeFilled, to: '/' },
  { section: 'datasets', label: '评测集', icon: Collection, to: '/datasets' },
  { section: 'evaluations', label: '我的评测', icon: List, to: '/evaluations' },
  { section: 'badcases', label: 'Badcase 中心', icon: Tickets, to: '/badcases' },
  { section: 'dashboard', label: '数据看板', icon: DataAnalysis, to: '/dashboard' },
]
const adminNavigation = [
  { label: '基础目录', icon: Operation, to: '/admin/catalog' },
  { label: '问题标签', icon: Tickets, to: '/admin/issue-tags' },
  { label: '用户管理', icon: User, to: '/admin/users' },
  { label: '审计日志', icon: Setting, to: '/admin/audit-logs' },
]
const adminNavigationOpen = ref(false)
const activeSection = computed(() => String(route.meta.section || ''))
const adminStorageKey = computed(
  () => `quickeval:admin-navigation-expanded:${auth.user?.id || 'anonymous'}`,
)

function restoreAdminNavigation() {
  adminNavigationOpen.value =
    activeSection.value === 'admin' || localStorage.getItem(adminStorageKey.value) === '1'
}

function toggleAdminNavigation() {
  adminNavigationOpen.value = !adminNavigationOpen.value
  localStorage.setItem(adminStorageKey.value, adminNavigationOpen.value ? '1' : '0')
}

async function logout() {
  await auth.logout()
  await router.replace('/login')
}

async function changePassword() {
  try {
    await apiClient.post('/api/v1/auth/change-password', passwordForm)
    passwordDialog.value = false
    auth.clear()
    ElMessage.success('密码已修改，请重新登录')
    await router.replace('/login')
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  }
}

async function search() {
  const keyword = searchKeyword.value.trim()
  if (!keyword) {
    searchItems.value = []
    searchOpen.value = false
    return
  }
  searchLoading.value = true
  try {
    const response = await apiClient.get<
      ResponseEnvelope<{ items: SearchItem[]; total: number }>
    >('/api/v1/search', {
      params: { q: keyword, page: 1, page_size: 12 },
    })
    searchItems.value = response.data.data.items
    searchOpen.value = true
  } catch (error) {
    ElMessage.error(apiErrorMessage(error))
  } finally {
    searchLoading.value = false
  }
}

function openSearchItem(item: SearchItem) {
  searchOpen.value = false
  searchKeyword.value = ''
  router.push(item.url)
}

function searchTypeLabel(type: SearchItem['type']) {
  return {
    target: '评测对象',
    scenario: '场景',
    dataset: '评测集',
    case: '用例',
    evaluation_result: '评测回答',
    badcase: 'Badcase',
  }[type]
}

watch(searchKeyword, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(search, 350)
})
watch(adminStorageKey, restoreAdminNavigation, { immediate: true })
watch(activeSection, (section) => {
  if (section === 'admin') adminNavigationOpen.value = true
})
onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
})
</script>

<template>
  <div class="app-shell">
    <aside class="app-sidebar" aria-label="主导航">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">Q</span>
        <span>QuickEval</span>
      </div>
      <nav class="navigation">
        <button
          v-for="item in navigation"
          :key="item.label"
          class="nav-item"
          :class="{ active: item.section === activeSection }"
          type="button"
          @click="router.push(item.to)"
        >
          <el-icon :size="18"><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </button>
        <div v-if="auth.isAdmin" class="nav-group">
          <button
            class="nav-item nav-group-toggle"
            :class="{ active: activeSection === 'admin' }"
            type="button"
            :aria-expanded="adminNavigationOpen"
            aria-controls="admin-navigation"
            @click="toggleAdminNavigation"
          >
            <el-icon :size="18"><Setting /></el-icon>
            <span>系统管理</span>
            <el-icon class="nav-group-chevron" :class="{ open: adminNavigationOpen }">
              <ArrowDown />
            </el-icon>
          </button>
          <div v-show="adminNavigationOpen" id="admin-navigation" class="nav-subitems">
            <button
              v-for="item in adminNavigation"
              :key="item.to"
              class="nav-item nav-subitem"
              :class="{ active: item.to === route.path }"
              type="button"
              @click="router.push(item.to)"
            >
              <el-icon :size="16"><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </button>
          </div>
        </div>
      </nav>
    </aside>

    <div class="app-main">
      <header class="app-header">
        <span class="header-title">{{ String(route.meta.title || 'QuickEval') }}</span>
        <el-popover
          v-model:visible="searchOpen"
          placement="bottom-start"
          :width="520"
          trigger="click"
          popper-class="global-search-popover"
        >
          <template #reference>
            <el-input
              v-model="searchKeyword"
              class="global-search-input"
              :prefix-icon="SearchIcon"
              placeholder="搜索对象、场景、用例、回答或 Badcase"
              clearable
              @keyup.enter="search"
            />
          </template>
          <div v-loading="searchLoading" class="global-search-results">
            <el-empty
              v-if="!searchLoading && searchItems.length === 0"
              :description="searchKeyword.trim() ? '没有匹配结果' : '输入关键词快速定位'"
              :image-size="60"
            />
            <button
              v-for="item in searchItems"
              :key="`${item.type}-${item.id}`"
              class="search-result-row"
              type="button"
              @click="openSearchItem(item)"
            >
              <el-tag size="small" effect="plain">{{ searchTypeLabel(item.type) }}</el-tag>
              <div>
                <strong>{{ item.title }}</strong>
                <span>{{ item.subtitle }}</span>
                <p v-if="item.snippet">{{ item.snippet }}</p>
              </div>
            </button>
          </div>
        </el-popover>
        <el-dropdown>
          <button class="user-menu" type="button">
            <span class="avatar">{{ auth.user?.display_name.slice(0, 1) }}</span>
            <span>{{ auth.user?.display_name }}</span>
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="passwordDialog = true">修改密码</el-dropdown-item>
              <el-dropdown-item divided @click="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </header>
      <main class="app-content"><RouterView /></main>
    </div>
  </div>

  <el-dialog v-model="passwordDialog" title="修改密码" width="440">
    <el-form label-position="top">
      <el-form-item label="当前密码">
        <el-input v-model="passwordForm.current_password" type="password" show-password />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="passwordForm.new_password" type="password" show-password />
        <small class="form-help">至少 10 位字符</small>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="passwordDialog = false">取消</el-button>
      <el-button type="primary" @click="changePassword">确认修改</el-button>
    </template>
  </el-dialog>
</template>
