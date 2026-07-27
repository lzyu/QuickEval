import { expect, test, type Page } from '@playwright/test'

const adminSession = {
  data: {
    user: {
      id: '019fa2a2-ed09-7660-988d-38cb279d5198',
      username: 'admin',
      display_name: '系统管理员',
      email: 'admin@quickeval.local',
      role: 'admin',
      status: 'active',
      lock_version: 0,
    },
    permissions: {
      manage_users: true,
      manage_catalog: true,
      evaluate: true,
      manage_badcases: true,
      view_audit_logs: true,
    },
    features: { oa_login_enabled: false },
    upload_policy: {
      allowed_media_types: ['image/png'],
      max_file_size: 10485760,
      max_files_per_owner: 10,
    },
    csrf_token: 'csrf-token',
  },
  meta: { request_id: 'e2e-session' },
}

async function mockHealth(page: Page) {
  await page.route('**/health/ready', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { status: 'ok', dependencies: { mysql: 'ok', redis: 'ok' } },
        meta: { request_id: 'e2e-request' },
      }),
    })
  })
}

test('logs in, restores the shell, and exposes admin navigation', async ({ page }) => {
  let authenticated = false
  await mockHealth(page)
  await page.route('**/api/v1/auth/session', async (route) => {
    if (route.request().method() === 'DELETE') {
      authenticated = false
      await route.fulfill({ status: 204 })
      return
    }
    await route.fulfill({
      status: authenticated ? 200 : 401,
      contentType: 'application/json',
      body: authenticated ? JSON.stringify(adminSession) : JSON.stringify({ error: {} }),
    })
  })
  await page.route('**/api/v1/auth/login', async (route) => {
    authenticated = true
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(adminSession),
    })
  })

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '欢迎回来' })).toBeVisible()
  await page.getByLabel('用户名或邮箱').fill('admin')
  await page.getByLabel('密码').fill('mock-password')
  await page.getByRole('button', { name: '登录' }).click()

  await expect(page.getByRole('heading', { name: 'Agent 人工评测平台' })).toBeVisible()
  await expect(page.getByText('后端、MySQL 与 Redis 已就绪')).toBeVisible()
  await expect(page.getByRole('button', { name: /用户管理/ })).toBeVisible()
  await expect(page.getByRole('button', { name: /审计日志/ })).toBeVisible()
})

test('member is redirected away from admin routes', async ({ page }) => {
  const memberSession = structuredClone(adminSession)
  memberSession.data.user.role = 'member'
  memberSession.data.permissions.manage_users = false
  memberSession.data.permissions.manage_catalog = false
  memberSession.data.permissions.view_audit_logs = false
  await page.route('**/api/v1/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(memberSession),
    })
  })

  await page.goto('/admin/users')
  await expect(page.getByRole('heading', { name: '没有访问权限' })).toBeVisible()
  await expect(page.getByRole('button', { name: /用户管理/ })).toHaveCount(0)
})
