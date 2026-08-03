import { createRouter, createWebHistory } from 'vue-router'

import AppShell from '@/components/app/AppShell.vue'
import { useAuthStore } from '@/stores/auth'
import AuditLogsView from '@/views/admin/AuditLogsView.vue'
import CatalogView from '@/views/admin/CatalogView.vue'
import IssueTagsView from '@/views/admin/IssueTagsView.vue'
import UsersView from '@/views/admin/UsersView.vue'
import ForbiddenView from '@/views/ForbiddenView.vue'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/LoginView.vue'
import NotFoundView from '@/views/NotFoundView.vue'
import PasswordChangeView from '@/views/PasswordChangeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { public: true },
    },
    {
      path: '/forbidden',
      name: 'forbidden',
      component: ForbiddenView,
    },
    {
      path: '/change-password',
      name: 'password-change',
      component: PasswordChangeView,
    },
    {
      path: '/',
      component: AppShell,
      children: [
        {
          path: '',
          name: 'home',
          component: HomeView,
          meta: { title: '首页', section: 'home' },
        },
        {
          path: 'datasets',
          name: 'datasets',
          component: () => import('@/views/datasets/DatasetListView.vue'),
          meta: { title: '评测集', section: 'datasets' },
        },
        {
          path: 'datasets/:datasetId',
          name: 'dataset-detail',
          component: () => import('@/views/datasets/DatasetDetailView.vue'),
          meta: { title: '评测集详情', section: 'datasets' },
        },
        {
          path: 'version-cases/:caseId',
          name: 'case-detail',
          component: () => import('@/views/datasets/CaseDetailView.vue'),
          meta: { title: '用例详情', section: 'datasets' },
        },
        {
          path: 'dataset-versions/:versionId/edit',
          name: 'draft-editor',
          component: () => import('@/views/datasets/DraftEditorView.vue'),
          meta: { operationsAdmin: true, title: '编辑评测集草稿', section: 'datasets' },
        },
        {
          path: 'evaluations',
          name: 'my-evaluations',
          component: () => import('@/views/evaluations/MyEvaluationsView.vue'),
          meta: { title: '我的评测', section: 'evaluations' },
        },
        {
          path: 'badcases',
          name: 'badcases',
          component: () => import('@/views/badcases/BadcaseListView.vue'),
          meta: { title: 'Badcase 中心', section: 'badcases' },
        },
        {
          path: 'badcases/register',
          name: 'badcase-register',
          component: () => import('@/views/badcases/BadcaseRegisterView.vue'),
          meta: { title: '主动登记 Badcase', section: 'badcases' },
        },
        {
          path: 'badcases/:badcaseId',
          name: 'badcase-detail',
          component: () => import('@/views/badcases/BadcaseDetailView.vue'),
          meta: { title: 'Badcase 详情', section: 'badcases' },
        },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
          meta: { title: '数据看板', section: 'dashboard' },
        },
        {
          path: 'evaluation-results',
          name: 'evaluation-results',
          component: () => import('@/views/evaluations/EvaluationResultsView.vue'),
          meta: { title: '评测结果', section: 'dashboard' },
        },
        {
          path: 'evaluation-runs/:runId/workbench',
          name: 'evaluation-workbench',
          component: () => import('@/views/evaluations/EvaluationWorkbenchView.vue'),
          meta: { title: '评测工作台', section: 'evaluations' },
        },
        {
          path: 'evaluation-runs/:runId/result',
          name: 'evaluation-result',
          component: () => import('@/views/evaluations/EvaluationWorkbenchView.vue'),
          meta: { title: '评测结果', section: 'evaluations' },
        },
        {
          path: 'admin/catalog',
          name: 'admin-catalog',
          component: CatalogView,
          meta: { operationsAdmin: true, title: '评测配置', section: 'admin' },
        },
        {
          path: 'admin/users',
          name: 'admin-users',
          component: UsersView,
          meta: { superAdmin: true, title: '用户管理', section: 'admin' },
        },
        {
          path: 'admin/issue-tags',
          name: 'admin-issue-tags',
          component: IssueTagsView,
          meta: { operationsAdmin: true, title: '问题标签', section: 'admin' },
        },
        {
          path: 'admin/audit-logs',
          name: 'admin-audit',
          component: AuditLogsView,
          meta: { operationsAdmin: true, title: '审计日志', section: 'admin' },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: NotFoundView,
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.restore()
  if (to.meta.public) {
    if (!auth.isAuthenticated || to.name !== 'login') return true
    return auth.passwordChangeRequired ? { name: 'password-change' } : { name: 'home' }
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (auth.passwordChangeRequired && to.name !== 'password-change') {
    return { name: 'password-change' }
  }
  if (!auth.passwordChangeRequired && to.name === 'password-change') {
    return { name: 'home' }
  }
  if (to.meta.superAdmin && !auth.isSuperAdmin) {
    return { name: 'forbidden' }
  }
  if (to.meta.operationsAdmin && !auth.isOperationsAdmin) {
    return { name: 'forbidden' }
  }
  return true
})

export default router
