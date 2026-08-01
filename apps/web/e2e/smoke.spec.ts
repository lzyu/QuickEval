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

const targetId = '019fa2a2-ed09-7660-988d-38cb279d5101'
const scenarioId = '019fa2a2-ed09-7660-988d-38cb279d5102'
const datasetId = '019fa2a2-ed09-7660-988d-38cb279d5103'
const draftId = '019fa2a2-ed09-7660-988d-38cb279d5104'
const caseId = '019fa2a2-ed09-7660-988d-38cb279d5105'
const caseKey = '019fa2a2-ed09-7660-988d-38cb279d5106'
const tagId = '019fa2a2-ed09-7660-988d-38cb279d5107'
const meta = { request_id: 'e2e-dataset' }
const homeData = {
  metrics: [
    { key: 'in_progress', label: '我的进行中评测', value: 1, url: '/evaluations?status=in_progress' },
    { key: 'completed', label: '我的已完成评测', value: 2, url: '/evaluations?status=completed' },
    { key: 'assigned_badcases', label: '分配给我的未关闭 Badcase', value: 1, url: '/badcases?assigned_to_me=1&open=1' },
  ],
  continue_evaluations: [],
  assigned_badcases: [],
  recent_datasets: [],
  recent_activities: [],
}

async function mockHome(page: Page) {
  await page.route('**/api/v1/pages/home', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: homeData, meta }),
    })
  })
}

const target = {
  id: targetId,
  name: '智能采购 Agent',
  description: null,
  status: 'active',
  lock_version: 0,
}
const scenario = {
  ...target,
  id: scenarioId,
  name: '采购询价',
  evaluation_target_id: targetId,
  evaluation_target_name: target.name,
}
const dataset = {
  id: datasetId,
  scenario_id: scenarioId,
  scenario_name: scenario.name,
  evaluation_target_id: targetId,
  evaluation_target_name: target.name,
  name: '采购助手基础能力',
  description: '覆盖预算和交付周期',
  status: 'active',
  lock_version: 0,
  created_at: '2026-07-28T00:00:00Z',
  updated_at: '2026-07-28T00:00:00Z',
  latest_version_no: null,
  published_version_count: 0,
  draft_version_id: draftId,
  draft_case_count: 1,
}
const draft = {
  id: draftId,
  dataset_id: datasetId,
  base_version_id: null,
  version_no: null,
  status: 'draft',
  release_note: null,
  lock_version: 1,
  published_at: null,
  archived_at: null,
  created_at: '2026-07-28T00:00:00Z',
  updated_at: '2026-07-28T00:00:00Z',
  case_count: 1,
  enabled_count: 1,
}
const versionCase = {
  id: caseId,
  dataset_version_id: draftId,
  case_key: caseKey,
  name: '预算追问',
  user_prompt: '预算 10 万元，请推荐采购方案',
  precondition: null,
  expected_result: null,
  judging_guide: '不得虚构商品参数',
  sort_order: 10,
  is_enabled: true,
  lock_version: 0,
  tags: [{ id: tagId, name: '事实准确性' }],
  created_at: '2026-07-28T00:00:00Z',
  updated_at: '2026-07-28T00:00:00Z',
}

async function mockAdminSession(page: Page) {
  await page.route('**/api/v1/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(adminSession),
    })
  })
  await page.route('**/api/v1/issue-tags*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          items: [
            {
              id: tagId,
              name: '事实准确性',
              description: null,
              status: 'active',
              lock_version: 0,
              sort_order: 10,
            },
          ],
        },
        meta,
      }),
    })
  })
}

async function mockDraftReads(page: Page) {
  await page.route(`**/api/v1/dataset-versions/${draftId}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: draft, meta }),
    })
  })
  await page.route(`**/api/v1/datasets/${datasetId}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { dataset, versions: [draft] }, meta }),
    })
  })
  await page.route(`**/api/v1/dataset-versions/${draftId}/cases?*`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { items: [versionCase], page: 1, page_size: 100, total: 1 },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/scenarios/${scenarioId}/available-case-tags`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          global: [
            {
              id: tagId,
              name: '事实准确性',
              scope: 'global',
              scenario_id: null,
              status: 'active',
              lock_version: 0,
              sort_order: 10,
            },
          ],
          scenario: [],
        },
        meta,
      }),
    })
  })
}

test('logs in, restores the shell, and exposes admin navigation', async ({ page }) => {
  let authenticated = false
  await mockHealth(page)
  await mockHome(page)
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

  await expect(page.getByText('我的进行中评测')).toBeVisible()
  await expect(page.getByText('我的已完成评测')).toBeVisible()
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

test('actively registers badcases continuously and retries only a failed screenshot upload', async ({ page }) => {
  await mockAdminSession(page)
  const disabledTargetId = '019fa2a2-ed09-7660-988d-38cb279d5201'
  const issueTagId = '019fa2a2-ed09-7660-988d-38cb279d5202'
  const badcaseId = '019fa2a2-ed09-7660-988d-38cb279d5203'
  let createCount = 0
  let uploadCount = 0

  await page.route('**/api/v1/evaluation-targets*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          items: [target, { ...target, id: disabledTargetId, name: '合同审核助手' }],
          page: 1,
          page_size: 100,
          total: 2,
        },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/scenarios*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { items: [scenario], page: 1, page_size: 100, total: 1 },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/badcase-options', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          assignees: [],
          issue_tags: [
            {
              id: issueTagId,
              name: '事实准确性',
              description: null,
              status: 'active',
              lock_version: 0,
              sort_order: 10,
              scope: 'global',
            },
          ],
        },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/badcases', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.fallback()
      return
    }
    createCount += 1
    const request = route.request().postDataJSON()
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          id: badcaseId,
          source_type: 'business',
          scenario_id: scenarioId,
          scenario_name: scenario.name,
          evaluation_target_id: targetId,
          evaluation_target_name: target.name,
          title: request.title,
          description: request.description,
          agent_response_text: request.agent_response_text,
          agent_version: request.agent_version,
          environment: request.environment,
          occurred_at: request.occurred_at,
          business_reference: request.business_reference,
          session_id: request.session_id,
          status: 'pending',
          assignee_id: null,
          assignee_name: null,
          resolved_at: null,
          invalidated_at: null,
          invalidated_by: null,
          invalidator_name: null,
          invalid_reason: null,
          lock_version: 0,
          created_by: adminSession.data.user.id,
          creator_name: adminSession.data.user.display_name,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          issue_tags: [{ id: issueTagId, name: '事实准确性' }],
          evaluation: null,
          original_attachments: [],
          attachments: [],
          activities: [],
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/badcases/${badcaseId}/attachments`, async (route) => {
    uploadCount += 1
    if (uploadCount === 1) {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: { message: '模拟对象存储故障' } }),
      })
      return
    }
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [], owner_lock_version: 1 }, meta }),
    })
  })

  await page.goto('/badcases/register')
  await expect(page.getByRole('heading', { name: '选择评测对象' })).toBeVisible()
  await page.getByPlaceholder('搜索评测对象').fill('合同')
  await expect(page.getByRole('button', { name: /合同审核助手/ })).toBeDisabled()
  await page.getByPlaceholder('搜索评测对象').fill('智能采购')
  await page.getByRole('button', { name: /智能采购 Agent/ }).click()

  await page.getByPlaceholder('请输入 Badcase 标题').fill('采购推荐违反预算约束')
  await page.getByPlaceholder('请简要描述问题现象、影响范围、期望结果等').fill('预算为 10 万元')
  await page.getByPlaceholder('例如 2026.07.30').fill('agent-v2')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'evidence.png',
    mimeType: 'image/png',
    buffer: Buffer.from('89504e470d0a1a0a', 'hex'),
  })

  await page.reload()
  await expect(page.getByPlaceholder('请输入 Badcase 标题')).toHaveValue('采购推荐违反预算约束')
  await expect(page.getByText(/截图需要重新选择/)).toBeVisible()
  await page.locator('input[type="file"]').setInputFiles({
    name: 'evidence.png',
    mimeType: 'image/png',
    buffer: Buffer.from('89504e470d0a1a0a', 'hex'),
  })
  await page.getByText('至少选择一个问题标签', { exact: true }).click()
  await page.getByRole('option', { name: '事实准确性' }).click()
  await page.getByRole('button', { name: '登记并继续' }).click()

  await expect(page.getByText('Badcase 已创建，截图尚未上传')).toBeVisible()
  expect(createCount).toBe(1)
  expect(uploadCount).toBe(1)
  await page.getByRole('button', { name: '重试上传' }).click()
  await expect(page.getByText('采购推荐违反预算约束')).toBeVisible()
  await expect(page.getByPlaceholder('请输入 Badcase 标题')).toHaveValue('')
  await expect(page.getByPlaceholder('例如 2026.07.30')).toHaveValue('agent-v2')
  expect(createCount).toBe(1)
  expect(uploadCount).toBe(2)
})

test('admin manages global and scenario case tags by scope', async ({ page }) => {
  await mockAdminSession(page)
  let createdGlobalTag = false
  const scenarioTagId = '019fa2a2-ed09-7660-988d-38cb279d5117'
  await page.route('**/api/v1/evaluation-targets?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [target], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })
  await page.route('**/api/v1/scenarios?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [scenario], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })
  await page.route('**/api/v1/case-tags?*', async (route) => {
    const url = new URL(route.request().url())
    const scope = url.searchParams.get('scope')
    const item = scope === 'global'
      ? {
          id: tagId,
          name: '意图识别',
          description: null,
          scope: 'global',
          scenario_id: null,
          status: 'active',
          lock_version: 0,
          sort_order: 10,
        }
      : {
          id: scenarioTagId,
          name: '供应商比较',
          description: null,
          scope: 'scenario',
          scenario_id: scenarioId,
          scenario_name: scenario.name,
          status: 'active',
          lock_version: 0,
          sort_order: 10,
        }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [item] }, meta }),
    })
  })
  await page.route('**/api/v1/case-tags', async (route) => {
    if (route.request().method() !== 'POST') {
      await route.fallback()
      return
    }
    expect(route.request().postDataJSON()).toMatchObject({
      scope: 'global',
      scenario_id: null,
      name: '指令遵循',
    })
    createdGlobalTag = true
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          id: '019fa2a2-ed09-7660-988d-38cb279d5118',
          name: '指令遵循',
          description: null,
          scope: 'global',
          scenario_id: null,
          status: 'active',
          lock_version: 0,
          sort_order: 20,
        },
        meta,
      }),
    })
  })

  await page.goto('/admin/catalog')
  await page.getByRole('tab', { name: '用例标签' }).click()
  await expect(page.getByText('意图识别')).toBeVisible()
  await page.getByText('场景标签', { exact: true }).click()
  await expect(page.getByText('供应商比较')).toBeVisible()
  await expect(page.locator('tbody').getByText(scenario.name)).toBeVisible()
  await page.getByText('全局标签', { exact: true }).click()
  await page.getByRole('button', { name: '新建目录项' }).click()
  await page.getByLabel('名称').fill('指令遵循')
  await page.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => createdGlobalTag).toBe(true)
})

test('admin creates a dataset and enters its initial draft', async ({ page }) => {
  await mockAdminSession(page)
  await page.route('**/api/v1/evaluation-targets?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [target], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })
  await page.route('**/api/v1/scenarios?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [scenario], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })
  await page.route(/\/api\/v1\/datasets(?:\?.*)?$/, async (route) => {
    if (route.request().method() === 'POST') {
      const payload = route.request().postDataJSON()
      expect(payload).toMatchObject({ scenario_id: scenarioId, name: '采购助手基础能力' })
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ data: { dataset, draft }, meta }),
      })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [], page: 1, page_size: 100, total: 0 }, meta }),
    })
  })
  await mockDraftReads(page)

  await page.goto('/datasets')
  await expect(page.getByRole('heading', { name: '评测集', exact: true })).toBeVisible()
  await page.getByRole('button', { name: '新建评测集' }).click()
  await page.getByLabel('评测集名称').fill('采购助手基础能力')
  await page.getByRole('button', { name: '创建并编辑草稿' }).click()

  await expect(page).toHaveURL(`/dataset-versions/${draftId}/edit`)
  await expect(page.getByRole('heading', { name: '编辑草稿' })).toBeVisible()
  await expect(page.getByText('预算追问')).toBeVisible()
})

test('draft editor previews CSV, commits it, and publishes the version', async ({ page }) => {
  await mockAdminSession(page)
  await mockDraftReads(page)
  let committed = false
  let published = false
  await page.route(`**/api/v1/dataset-versions/${draftId}/case-imports/preview`, async (route) => {
    expect(route.request().postDataBuffer()?.length).toBeGreaterThan(0)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          import_token: 'one-time-token',
          version_lock_version: 1,
          rows: [
            {
              row_number: 2,
              name: '交付周期',
              user_prompt: '多久交付',
              precondition: '',
              expected_result: '',
              judging_guide: '',
              tag_names: [],
              is_enabled: true,
              errors: [],
            },
          ],
          has_errors: false,
          valid_row_count: 1,
          error_row_count: 0,
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/dataset-versions/${draftId}/case-imports/commit`, async (route) => {
    expect(route.request().postDataJSON()).toEqual({ import_token: 'one-time-token' })
    committed = true
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ data: { created_count: 1 }, meta }),
    })
  })
  await page.route(`**/api/v1/dataset-versions/${draftId}/publish`, async (route) => {
    expect(route.request().postDataJSON()).toMatchObject({ expected_lock_version: 1 })
    published = true
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { ...draft, status: 'published', version_no: 1, lock_version: 2 },
        meta,
      }),
    })
  })

  await page.goto(`/dataset-versions/${draftId}/edit`)
  await expect(page.getByText('预算追问')).toBeVisible()
  await page.getByRole('button', { name: '导入 CSV' }).click()
  await page.locator('input[type="file"]').setInputFiles({
    name: 'cases.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from('用例名称,用户问题\n交付周期,多久交付\n'),
  })
  await page.getByRole('button', { name: '上传并校验' }).click()
  await expect(page.getByText('校验通过')).toBeVisible()
  await page.getByRole('button', { name: '确认追加 1 条' }).click()
  await expect.poll(() => committed).toBe(true)

  await page.getByRole('button', { name: '发布版本' }).click()
  await page.getByLabel('发布说明').fill('首次发布')
  await page.getByRole('button', { name: '确认发布 V1' }).click()
  await expect.poll(() => published).toBe(true)
  await expect(page).toHaveURL(`/datasets/${datasetId}`)
})

const runId = '019fa2a2-ed09-7660-988d-38cb279d5201'
const secondResultId = '019fa2a2-ed09-7660-988d-38cb279d5202'
const runProgress = {
  total_count: 2,
  pending_count: 2,
  evaluated_count: 0,
  skipped_count: 0,
  scored_count: 0,
  badcase_count: 0,
  average_score: null,
  completion_rate: 0,
}
const evaluationRun = {
  id: runId,
  dataset_version_id: draftId,
  dataset_id: datasetId,
  dataset_name: dataset.name,
  version_no: 1,
  scenario_id: scenarioId,
  scenario_name: scenario.name,
  evaluation_target_id: targetId,
  evaluation_target_name: target.name,
  evaluator_id: adminSession.data.user.id,
  evaluator_name: adminSession.data.user.display_name,
  agent_version: '2026.07.28',
  environment: 'staging',
  purpose_note: null,
  config_note: null,
  status: 'in_progress',
  lock_version: 0,
  first_completed_at: null,
  completed_at: null,
  voided_at: null,
  void_reason: null,
  created_at: '2026-07-28T00:00:00Z',
  updated_at: '2026-07-28T00:00:00Z',
  progress: runProgress,
}
const secondCaseResult = {
  ...versionCase,
  id: secondResultId,
  evaluation_run_id: runId,
  version_case_id: '019fa2a2-ed09-7660-988d-38cb279d5203',
  case_key: '019fa2a2-ed09-7660-988d-38cb279d5204',
  name: '交付周期',
  user_prompt: '预计多久可以交付？',
  status: 'pending',
  answer_text: null,
  score: null,
  comment: null,
  skip_reason: null,
  has_badcase: false,
  attachments: [],
  badcase: null,
}
const firstCaseResult = {
  ...versionCase,
  evaluation_run_id: runId,
  version_case_id: versionCase.id,
  status: 'pending',
  answer_text: null,
  score: null,
  comment: null,
  skip_reason: null,
  has_badcase: false,
  attachments: [],
  badcase: null,
}

test('starts an independent evaluation from a published dataset version', async ({ page }) => {
  await mockAdminSession(page)
  const publishedVersion = { ...draft, status: 'published', version_no: 1 }
  await page.route(`**/api/v1/datasets/${datasetId}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { dataset: { ...dataset, latest_version_no: 1 }, versions: [publishedVersion] },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/evaluation-runs', async (route) => {
    expect(route.request().headers()['idempotency-key']).toBeTruthy()
    expect(route.request().postDataJSON()).toMatchObject({
      dataset_version_id: draftId,
      agent_version: '2026.07.28',
      environment: 'staging',
    })
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ data: evaluationRun, meta }),
    })
  })
  await page.route(`**/api/v1/pages/evaluation-runs/${runId}/workbench?*`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          run: evaluationRun,
          results: { items: [firstCaseResult, secondCaseResult], page: 1, page_size: 100, total: 2 },
        },
        meta,
      }),
    })
  })

  await page.goto(`/datasets/${datasetId}`)
  await page.getByRole('button', { name: '开始评测' }).first().click()
  await page.getByLabel('Agent 版本').fill('2026.07.28')
  await page.locator('.el-drawer').getByRole('button', { name: '开始评测' }).click()

  await expect(page).toHaveURL(`/evaluation-runs/${runId}/workbench`)
  await expect(page.getByRole('heading', { name: `${dataset.name} V1` })).toBeVisible()
  await expect(page.getByRole('heading', { name: '预算追问' })).toBeVisible()
})

test('lists the current user evaluations and continues an in-progress run', async ({ page }) => {
  await mockAdminSession(page)
  await page.route('**/api/v1/evaluation-runs?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { items: [evaluationRun], page: 1, page_size: 100, total: 1 },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/pages/evaluation-runs/${runId}/workbench?*`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          run: evaluationRun,
          results: { items: [firstCaseResult, secondCaseResult], page: 1, page_size: 100, total: 2 },
        },
        meta,
      }),
    })
  })

  await page.goto('/evaluations')
  await expect(page.getByRole('heading', { name: '我的评测' })).toBeVisible()
  await expect(page.getByText(dataset.name)).toBeVisible()
  await expect(page.getByText('0/2')).toBeVisible()
  await page.getByRole('button', { name: '继续评测' }).click()
  await expect(page).toHaveURL(`/evaluation-runs/${runId}/workbench`)
})

test('saves, skips, completes and renders an evaluation read-only', async ({ page }) => {
  await mockAdminSession(page)
  let currentRun = structuredClone(evaluationRun)
  const currentResults = [structuredClone(firstCaseResult), structuredClone(secondCaseResult)]
  await page.route(`**/api/v1/pages/evaluation-runs/${runId}/workbench?*`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          run: currentRun,
          results: { items: currentResults, page: 1, page_size: 100, total: 2 },
        },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/case-results/*', async (route) => {
    const payload = route.request().postDataJSON()
    const id = route.request().url().split('/').at(-1)
    const index = currentResults.findIndex((item) => item.id === id)
    currentResults[index] = {
      ...currentResults[index],
      ...payload,
      lock_version: currentResults[index].lock_version + 1,
    }
    const evaluated = currentResults.filter((item) => item.status === 'evaluated')
    const skipped = currentResults.filter((item) => item.status === 'skipped')
    currentRun.lock_version += 1
    currentRun.progress = {
      ...currentRun.progress,
      pending_count: 2 - evaluated.length - skipped.length,
      evaluated_count: evaluated.length,
      skipped_count: skipped.length,
      scored_count: evaluated.filter((item) => item.score).length,
      average_score: evaluated[0]?.score || null,
      completion_rate: (evaluated.length + skipped.length) / 2,
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          result: currentResults[index],
          progress: currentRun.progress,
          run_lock_version: currentRun.lock_version,
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/evaluation-runs/${runId}/complete`, async (route) => {
    expect(route.request().postDataJSON()).toEqual({ expected_lock_version: 2 })
    currentRun = {
      ...currentRun,
      status: 'completed',
      lock_version: 3,
      first_completed_at: '2026-07-28T01:00:00Z',
      completed_at: '2026-07-28T01:00:00Z',
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: currentRun, meta }),
    })
  })

  await page.goto(`/evaluation-runs/${runId}/workbench`)
  await page.getByLabel('Agent 回答').fill('这是 Agent 的完整回答')
  await page.locator('.score-picker button').nth(3).click()
  await page.getByRole('button', { name: '保存并下一条' }).click()
  await expect(page.getByRole('heading', { name: '交付周期' })).toBeVisible()

  await page.getByRole('button', { name: '跳过此用例' }).click()
  await page.getByText('当前账号无权限', { exact: true }).click()
  await page.getByRole('button', { name: '确认跳过' }).click()
  await expect(page.getByText('待评').first()).toBeVisible()

  await page.getByRole('button', { name: '完成评测' }).click()
  await page.getByRole('button', { name: '确认完成' }).click()
  await expect(page).toHaveURL(`/evaluation-runs/${runId}/result`)
  await expect(page.getByText('已完成', { exact: true }).first()).toBeVisible()
  await expect(page.getByRole('button', { name: '保存并下一条' })).toHaveCount(0)
})

test('uploads screenshot evidence and marks the same result as a Badcase', async ({ page }) => {
  await mockAdminSession(page)
  const result = structuredClone(firstCaseResult)
  const run = structuredClone(evaluationRun)
  const attachment = {
    id: '019fa2a2-ed09-7660-988d-38cb279d5301',
    original_name: 'agent-proof.png',
    media_type: 'image/png',
    file_size: 8,
    width: 1,
    height: 1,
    sort_order: 10,
    content_url: '/api/v1/attachments/019fa2a2-ed09-7660-988d-38cb279d5301/content',
    created_by: adminSession.data.user.id,
    created_at: '2026-07-28T01:00:00Z',
  }
  await page.route(`**/api/v1/pages/evaluation-runs/${runId}/workbench?*`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          run: { ...run, progress: { ...run.progress, total_count: 1, pending_count: 1 } },
          results: { items: [result], page: 1, page_size: 100, total: 1 },
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/case-results/${result.id}/attachments`, async (route) => {
    expect(route.request().headers()['idempotency-key']).toBeTruthy()
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        data: { items: [attachment], owner_lock_version: 1 },
        meta,
      }),
    })
  })
  await page.route(`**${attachment.content_url}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'image/png',
      body: Buffer.from('fake-png'),
    })
  })
  await page.route(`**/api/v1/case-results/${result.id}/mark-badcase`, async (route) => {
    const payload = route.request().postDataJSON()
    expect(payload.expected_result_lock_version).toBe(1)
    expect(payload.result_patch.answer_text).toBeNull()
    expect(payload.badcase.issue_tag_ids).toEqual([tagId])
    const badcase = {
      id: '019fa2a2-ed09-7660-988d-38cb279d5302',
      title: payload.badcase.title,
      description: payload.badcase.description,
      status: 'pending',
      issue_tags: [{ id: tagId, name: '事实准确性' }],
    }
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          badcase,
          result: {
            ...result,
            status: 'evaluated',
            score: 2,
            comment: '截图中的参数与事实不符',
            has_badcase: true,
            attachments: [attachment],
            badcase,
            lock_version: 2,
          },
          progress: {
            ...run.progress,
            total_count: 1,
            pending_count: 0,
            evaluated_count: 1,
            scored_count: 1,
            badcase_count: 1,
            completion_rate: 1,
          },
          run_lock_version: 1,
        },
        meta,
      }),
    })
  })

  await page.goto(`/evaluation-runs/${runId}/workbench`)
  await page.getByRole('button', { name: /上传截图/ }).click()
  await page.locator('input[type=file]').setInputFiles({
    name: 'agent-proof.png',
    mimeType: 'image/png',
    buffer: Buffer.from('fake-png'),
  })
  await expect(page.getByText('agent-proof.png')).toBeVisible()
  await page.locator('.score-picker button').nth(1).click()
  await page.getByLabel('评语').fill('截图中的参数与事实不符')
  await page.getByText('标记为 Badcase', { exact: true }).click()
  await page.getByPlaceholder('问题标题').fill('Agent 回答事实错误')
  await page.getByPlaceholder('描述具体问题与复现表现').fill('截图中的商品参数与实际约束冲突')
  await page.locator('.badcase-fields .el-select').click()
  await page.locator('.el-select-dropdown__item').filter({ hasText: '事实准确性' }).click()
  await page.getByRole('button', { name: '保存并下一条' }).click()

  await expect(page.getByText('已标记 Badcase')).toBeVisible()
  await expect(page.getByRole('button', { name: '查看详情' })).toBeVisible()
})

test('registers a business Badcase and advances its processing timeline', async ({ page }) => {
  await mockAdminSession(page)
  const badcaseId = '019fa2a2-ed09-7660-988d-38cb279d5401'
  const assigneeId = adminSession.data.user.id
  let lockVersion = 0
  let status = 'pending'
  let assignee: string | null = null
  const activities: Array<Record<string, unknown>> = [
    {
      id: '019fa2a2-ed09-7660-988d-38cb279d5402',
      activity_type: 'created',
      note: null,
      actor_id: assigneeId,
      actor_name: '系统管理员',
      from_status: null,
      to_status: null,
      from_assignee_id: null,
      from_assignee_name: null,
      to_assignee_id: null,
      to_assignee_name: null,
      created_at: '2026-07-28T02:00:00Z',
    },
  ]
  const currentBadcase = () => ({
    id: badcaseId,
    source_type: 'business',
    scenario_id: scenarioId,
    scenario_name: scenario.name,
    evaluation_target_id: targetId,
    evaluation_target_name: target.name,
    title: '采购助手忽略预算上限',
    description: null,
    agent_response_text: '建议采购高配服务器',
    agent_version: '2026.07.28',
    environment: 'production',
    occurred_at: '2026-07-28T02:00:00Z',
    status,
    assignee_id: assignee,
    assignee_name: assignee ? '系统管理员' : null,
    resolved_at: null,
    business_reference: 'ORDER-E2E-001',
    session_id: 'SESSION-E2E-001',
    invalidated_at: null,
    invalidated_by: null,
    invalidator_name: null,
    invalid_reason: null,
    lock_version: lockVersion,
    created_by: assigneeId,
    creator_name: '系统管理员',
    created_at: '2026-07-28T02:00:00Z',
    updated_at: '2026-07-28T02:00:00Z',
    issue_tags: [{ id: tagId, name: '事实准确性' }],
    evaluation: null,
    original_attachments: [],
    attachments: [],
    activities,
  })

  await page.route('**/api/v1/evaluation-targets?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [target], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })

  await page.route('**/api/v1/scenarios?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [scenario], page: 1, page_size: 100, total: 1 }, meta }),
    })
  })
  await page.route('**/api/v1/badcase-options', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          assignees: [{ id: assigneeId, display_name: '系统管理员' }],
          issue_tags: [{ id: tagId, name: '事实准确性', status: 'active', scope: 'global' }],
        },
        meta,
      }),
    })
  })
  await page.route(/\/api\/v1\/badcases(?:\?.*)?$/, async (route) => {
    if (route.request().method() === 'POST') {
      expect(route.request().headers()['idempotency-key']).toBeTruthy()
      expect(route.request().postDataJSON()).toMatchObject({
        scenario_id: scenarioId,
        title: '采购助手忽略预算上限',
        description: null,
        issue_tag_ids: [tagId],
      })
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ data: currentBadcase(), meta }),
      })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { items: [], page: 1, page_size: 20, total: 0 }, meta }),
    })
  })
  await page.route(`**/api/v1/pages/badcases/${badcaseId}`, async (route) => {
    const allowedActions = status === 'pending'
      ? ['edit', 'invalidate', 'assign', 'unassign', 'update_tags', 'add_note', 'add_attachment', 'start_processing', 'resolve', 'defer']
      : ['edit', 'invalidate', 'assign', 'unassign', 'update_tags', 'add_note', 'add_attachment', 'resolve', 'defer']
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          ...currentBadcase(),
          candidate_assignees: [{ id: assigneeId, display_name: '系统管理员' }],
          candidate_issue_tags: [{ id: tagId, name: '事实准确性' }],
          allowed_actions: allowedActions,
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/badcases/${badcaseId}/assign`, async (route) => {
    expect(route.request().postDataJSON()).toMatchObject({
      assignee_id: assigneeId,
      expected_lock_version: 0,
    })
    assignee = assigneeId
    lockVersion += 1
    activities.push({
      id: '019fa2a2-ed09-7660-988d-38cb279d5403',
      activity_type: 'assignee_changed',
      actor_id: assigneeId,
      actor_name: '系统管理员',
      to_assignee_id: assigneeId,
      to_assignee_name: '系统管理员',
      created_at: '2026-07-28T02:01:00Z',
    })
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: currentBadcase(), meta }),
    })
  })
  await page.route(`**/api/v1/badcases/${badcaseId}/add-note`, async (route) => {
    expect(route.request().headers()['idempotency-key']).toBeTruthy()
    expect(route.request().postDataJSON()).toMatchObject({
      note: '已确认预算过滤条件未生效',
      expected_lock_version: 1,
    })
    lockVersion += 1
    activities.push({
      id: '019fa2a2-ed09-7660-988d-38cb279d5404',
      activity_type: 'note_added',
      note: '已确认预算过滤条件未生效',
      actor_id: assigneeId,
      actor_name: '系统管理员',
      created_at: '2026-07-28T02:02:00Z',
    })
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: currentBadcase(), meta }),
    })
  })
  await page.route(`**/api/v1/badcases/${badcaseId}/start-processing`, async (route) => {
    expect(route.request().postDataJSON()).toMatchObject({
      reason: '开始定位预算过滤逻辑',
      expected_lock_version: 2,
    })
    status = 'processing'
    lockVersion += 1
    activities.push({
      id: '019fa2a2-ed09-7660-988d-38cb279d5405',
      activity_type: 'status_changed',
      note: '开始定位预算过滤逻辑',
      actor_id: assigneeId,
      actor_name: '系统管理员',
      from_status: 'pending',
      to_status: 'processing',
      created_at: '2026-07-28T02:03:00Z',
    })
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: currentBadcase(), meta }),
    })
  })

  await page.goto('/badcases')
  await page.getByRole('button', { name: '主动登记 Badcase' }).click()
  await page.getByRole('button', { name: /智能采购 Agent/ }).click()
  await page.getByPlaceholder('请输入 Badcase 标题').fill('采购助手忽略预算上限')
  await page.getByPlaceholder('请粘贴 Agent 的完整回答文本，便于复现与分析').fill('建议采购高配服务器')
  await page.getByText('至少选择一个问题标签', { exact: true }).click()
  await page.getByRole('option', { name: '事实准确性', exact: true }).last().click()
  await page.getByRole('button', { name: '登记并继续' }).click()
  await page.getByRole('button', { name: '查看详情' }).click()

  await expect(page).toHaveURL(`/badcases/${badcaseId}`)
  await expect(page.getByRole('heading', { name: '采购助手忽略预算上限' })).toBeVisible()
  await page.getByText('未分配', { exact: true }).last().click()
  await page.locator('.el-select-dropdown__item').filter({ hasText: '系统管理员' }).click()
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await page.getByPlaceholder('补充定位结论、处理进展或后续计划').fill('已确认预算过滤条件未生效')
  await page.getByRole('button', { name: '添加备注' }).click()
  await page.getByRole('button', { name: '开始处理' }).click()
  await page.locator('.el-message-box textarea').fill('开始定位预算过滤逻辑')
  await page.getByRole('button', { name: 'OK', exact: true }).click()

  await expect(page.getByText('处理中', { exact: true }).first()).toBeVisible()
  await expect(page.getByText('已确认预算过滤条件未生效')).toBeVisible()
})

test('renders personal home, dashboard metrics, charts, and global search', async ({ page }) => {
  await mockAdminSession(page)
  await mockHome(page)
  const searchBadcaseId = '019fa2a2-ed09-7660-988d-38cb279d5501'
  const evaluationResultId = '019fa2a2-ed09-7660-988d-38cb279d5502'
  const dashboardData = {
    metrics: {
      completed_run_count: 2,
      evaluated_case_count: 3,
      scored_case_count: 3,
      average_score: 3.67,
      evaluation_badcase_count: 1,
      evaluation_badcase_rate: 1 / 3,
      valid_badcase_count: 2,
      skipped_case_count: 1,
    },
    score_distribution: [1, 2, 3, 4, 5].map((score) => ({
      key: String(score),
      label: `${score} 分`,
      count: score === 4 ? 2 : score === 3 ? 1 : 0,
    })),
    issue_tag_distribution: [{ key: tagId, label: '事实准确性', count: 2 }],
    status_distribution: [{ key: 'pending', label: '待处理', count: 2 }],
    source_distribution: [
      { key: 'evaluation', label: '评测发现', count: 1 },
      { key: 'business', label: '业务登记', count: 1 },
    ],
    skip_reason_distribution: [{ key: '当前账号无权限', label: '当前账号无权限', count: 1 }],
    version_comparison: [],
    options: {
      evaluation_targets: [{ id: targetId, name: target.name }],
      scenarios: [{ id: scenarioId, name: scenario.name, parent_id: targetId }],
      datasets: [{ id: datasetId, name: dataset.name, parent_id: scenarioId }],
      dataset_versions: [{ id: draftId, name: `${dataset.name} V1`, parent_id: datasetId }],
      evaluators: [{ id: adminSession.data.user.id, name: '系统管理员' }],
      agent_versions: ['2026.07.28'],
      issue_tags: [{ id: tagId, name: '事实准确性' }],
    },
  }
  await page.route('**/api/v1/pages/dashboard*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: dashboardData, meta }),
    })
  })
  await page.route('**/api/v1/evaluation-results*', async (route) => {
    const params = new URL(route.request().url()).searchParams
    expect(
      params.get('score') === '4' || params.get('result_status') === 'evaluated',
    ).toBe(true)
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          items: [{
            id: evaluationResultId,
            evaluation_run_id: runId,
            evaluation_target_name: target.name,
            scenario_name: scenario.name,
            dataset_name: dataset.name,
            version_no: 1,
            evaluator_name: '系统管理员',
            agent_version: '2026.07.28',
            environment: 'production',
            completed_at: '2026-07-28T02:00:00Z',
            case_name: '预算约束',
            user_prompt: '预算 10 万元，推荐采购方案',
            result_status: 'evaluated',
            answer_text: '建议采购总价 12 万元的商品',
            score: 3,
            comment: '超出预算',
            skip_reason: null,
            has_badcase: false,
            badcase_id: null,
            badcase_title: null,
            case_tags: '',
            evidence_count: 0,
            result_detail_url: `/evaluation-runs/${runId}/result?result_id=${evaluationResultId}`,
          }],
          page: 1,
          page_size: 20,
          total: 1,
          completed_run_count: 1,
        },
        meta,
      }),
    })
  })
  await page.route('**/api/v1/search?*', async (route) => {
    expect(new URL(route.request().url()).searchParams.get('q')).toBe('预算错误')
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        data: {
          items: [{
            type: 'badcase',
            id: searchBadcaseId,
            title: '采购助手忽略预算',
            subtitle: '智能采购 Agent / 采购询价',
            snippet: '预算为 10 万元时仍推荐超预算商品',
            url: `/badcases/${searchBadcaseId}`,
          }],
          page: 1,
          page_size: 12,
          total: 1,
        },
        meta,
      }),
    })
  })
  await page.route(`**/api/v1/pages/badcases/${searchBadcaseId}`, async (route) => {
    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'RESOURCE_NOT_FOUND', message: 'not found' }, meta }),
    })
  })

  await page.goto('/')
  await expect(page.getByText('我的进行中评测')).toBeVisible()
  await expect(page.getByText('分配给我的未关闭 Badcase')).toBeVisible()

  await page.getByRole('button', { name: /数据看板/ }).click()
  await expect(page).toHaveURL('/dashboard')
  await expect(page.getByRole('heading', { name: '数据看板' })).toBeVisible()
  await expect(page.getByText('3.67')).toBeVisible()
  await expect(page.locator('canvas')).toHaveCount(4)

  await page.getByRole('button', { name: /已评用例/ }).click()
  await expect(page).toHaveURL(/\/evaluation-results\?.*result_status=evaluated/)
  await expect(page.getByRole('heading', { name: '评测结果明细' })).toBeVisible()
  await expect(page.getByText('预算 10 万元，推荐采购方案')).toBeVisible()

  await page.getByPlaceholder('搜索对象、场景、用例、回答或 Badcase').fill('预算错误')
  await expect(page.getByText('采购助手忽略预算')).toBeVisible()
  await page.getByText('采购助手忽略预算').click()
  await expect(page).toHaveURL(`/badcases/${searchBadcaseId}`)
})
