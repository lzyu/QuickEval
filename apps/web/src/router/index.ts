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
