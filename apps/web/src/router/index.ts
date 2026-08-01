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
      path: '/',
      component: AppShell,
      children: [
        {
          path: '',
          name: 'home',
          component: HomeView,
        },
        {
          path: 'datasets',
          name: 'datasets',
          component: () => import('@/views/datasets/DatasetListView.vue'),
        },
        {
          path: 'datasets/:datasetId',
          name: 'dataset-detail',
          component: () => import('@/views/datasets/DatasetDetailView.vue'),
        },
        {
          path: 'version-cases/:caseId',
          name: 'case-detail',
          component: () => import('@/views/datasets/CaseDetailView.vue'),
        },
        {
          path: 'dataset-versions/:versionId/edit',
          name: 'draft-editor',
          component: () => import('@/views/datasets/DraftEditorView.vue'),
          meta: { admin: true },
        },
        {
          path: 'evaluations',
          name: 'my-evaluations',
          component: () => import('@/views/evaluations/MyEvaluationsView.vue'),
        },
        {
          path: 'badcases',
          name: 'badcases',
          component: () => import('@/views/badcases/BadcaseListView.vue'),
        },
        {
          path: 'badcases/register',
          name: 'badcase-register',
          component: () => import('@/views/badcases/BadcaseRegisterView.vue'),
          meta: { title: 'Badcase 中心 / 主动登记' },
        },
        {
          path: 'badcases/:badcaseId',
          name: 'badcase-detail',
          component: () => import('@/views/badcases/BadcaseDetailView.vue'),
        },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
        },
        {
          path: 'evaluation-results',
          name: 'evaluation-results',
          component: () => import('@/views/evaluations/EvaluationResultsView.vue'),
        },
        {
          path: 'evaluation-runs/:runId/workbench',
          name: 'evaluation-workbench',
          component: () => import('@/views/evaluations/EvaluationWorkbenchView.vue'),
        },
        {
          path: 'evaluation-runs/:runId/result',
          name: 'evaluation-result',
          component: () => import('@/views/evaluations/EvaluationWorkbenchView.vue'),
        },
        {
          path: 'admin/catalog',
          name: 'admin-catalog',
          component: CatalogView,
          meta: { admin: true },
        },
        {
          path: 'admin/users',
          name: 'admin-users',
          component: UsersView,
          meta: { admin: true },
        },
        {
          path: 'admin/issue-tags',
          name: 'admin-issue-tags',
          component: IssueTagsView,
          meta: { admin: true },
        },
        {
          path: 'admin/audit-logs',
          name: 'admin-audit',
          component: AuditLogsView,
          meta: { admin: true },
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
    return auth.isAuthenticated && to.name === 'login' ? { name: 'home' } : true
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'forbidden' }
  }
  return true
})

export default router
