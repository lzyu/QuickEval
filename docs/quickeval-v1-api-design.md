# QuickEval V1 REST API 设计

> 状态：设计基线已确认
>
> 调用者：QuickEval Vue SPA
>
> 认证：同域 Cookie Session + CSRF
>
> 更新日期：2026-07-25

## 1. 设计范围

本文定义 QuickEval V1 的 REST API 路径、调用约定、权限、错误模型和关键事务边界。领域和数据库约束见 [QuickEval V1 领域模型与数据库设计](./quickeval-v1-database-design.md)。

V1 API 只服务同域部署的 Vue SPA，不承诺为外部系统提供长期兼容的公共接口，不实现 API Key、OAuth2、个人访问令牌、Agent API 调用或 Python Worker 认证。

## 2. 接口形态

V1 采用“稳定资源接口＋显式工作流命令＋少量只读页面聚合”的混合设计。

```text
/api/v1/{resources}                  常规资源查询与内容编辑
/api/v1/{resources}/{id}:action      生命周期或跨表事务命令
/api/v1/pages/*                      Vue 页面聚合读取
```

设计规则：

- 普通字段通过 `POST/PATCH/DELETE` 管理。
- 发布、归档、完成、重开、作废、分配、解决和无效化等状态迁移使用显式命令。
- 不提供通用 `PATCH status` 或万能 `transition` 接口。
- `pages/*` 只读取，不承担写操作。
- 页面聚合复用稳定资源 Schema，不形成第二套领域模型。
- 所有业务规则由服务端重新校验，前端的按钮状态不构成权限保证。

## 3. 全局协议

### 3.1 基础路径与格式

```text
/api/v1
```

- JSON 字段使用 `snake_case`。
- UUID 在路径和 JSON 中使用标准字符串。
- 时间使用 ISO 8601 UTC，例如 `2026-07-25T06:20:00.000Z`。
- JSON 请求使用 `Content-Type: application/json`。
- 文件上传使用 `multipart/form-data`。
- CSV 下载使用 `text/csv; charset=utf-8` 并写入 UTF-8 BOM。

### 3.2 单条响应

```json
{
  "data": {
    "id": "019c...",
    "lock_version": 3
  },
  "meta": {
    "request_id": "req_..."
  }
}
```

### 3.3 列表与分页

V1 使用页码分页，以支持总数和跳页。

```http
GET /api/v1/badcases?page=1&page_size=20
```

```json
{
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 135
  },
  "meta": {
    "request_id": "req_..."
  }
}
```

- 默认每页 20 条。
- 最大每页 100 条。
- 列表必须使用稳定排序；默认按业务时间倒序，再按 UUID 排序。
- 非法页码和超出限制的 `page_size` 返回 `400`。

### 3.4 乐观锁

资源响应返回 `lock_version`。所有修改资源或执行状态命令的 JSON 请求携带：

```json
{
  "expected_lock_version": 3
}
```

版本不一致返回 `409 LOCK_VERSION_CONFLICT`，并在错误详情中返回当前锁版本。批量排序采用全有或全无事务，任何一项冲突时整体失败。

上传使用 multipart 字段 `expected_owner_lock_version`。删除附件通过查询参数携带所属记录锁版本。

### 3.5 错误响应

```json
{
  "error": {
    "code": "LOCK_VERSION_CONFLICT",
    "message": "数据已被其他用户更新，请刷新后重试",
    "field_errors": [],
    "details": {
      "current_lock_version": 4
    }
  },
  "meta": {
    "request_id": "req_..."
  }
}
```

状态码：

| 状态码 | 含义 |
| --- | --- |
| `400` | JSON、路径或查询参数格式错误 |
| `401` | 未登录或 Session 已失效 |
| `403` | 权限不足或 CSRF 校验失败 |
| `404` | 资源不存在或对当前用户不可见 |
| `409` | 乐观锁、重名、重复资源或非法状态迁移 |
| `413` | 上传文件过大 |
| `415` | 文件类型不支持 |
| `422` | 请求格式正确但字段或业务语义不合法 |
| `429` | 登录或接口请求超过限制 |
| `500` | 未预期服务端错误 |

服务端日志使用同一 `request_id`，不向客户端返回堆栈、SQL、磁盘路径或内部错误。

### 3.6 幂等

以下请求要求 `Idempotency-Key` 请求头：

- 创建 EvaluationRun。
- 创建业务 Badcase。
- 标记评测 Badcase。
- 提交 CSV 导入。
- 新增 Badcase 处理备注。

Redis 按“用户＋方法＋路径＋Key”保存首次结果 24 小时。相同 Key 和相同请求返回首次结果；相同 Key 但请求摘要不同返回 `409 IDEMPOTENCY_KEY_REUSED`。

发布、归档和完成等状态命令主要依赖状态机与乐观锁，不强制幂等 Key。

## 4. 认证与用户

### 4.1 Session

```http
POST   /api/v1/auth/login
GET    /api/v1/auth/session
DELETE /api/v1/auth/session
POST   /api/v1/auth/change-password
```

登录请求：

```json
{
  "username": "zhangsan",
  "password": "..."
}
```

`GET /auth/session` 是 SPA 启动接口，一次返回当前用户、权限、功能开关、上传策略和 CSRF Token：

```json
{
  "data": {
    "user": {
      "id": "019c...",
      "display_name": "张三",
      "email": "zhangsan@example.com",
      "role": "member"
    },
    "permissions": {
      "manage_users": false,
      "manage_catalog": false,
      "evaluate": true,
      "manage_badcases": true
    },
    "features": {
      "oa_login_enabled": false
    },
    "upload_policy": {
      "allowed_media_types": ["image/png", "image/jpeg", "image/webp"],
      "max_file_size": 10485760,
      "max_files_per_owner": 10
    },
    "csrf_token": "..."
  },
  "meta": {
    "request_id": "req_..."
  }
}
```

所有非安全方法携带 `X-CSRF-Token`。退出、修改密码和管理员重置密码后，使目标用户已有 Redis Session 失效。

### 4.2 用户管理

```http
GET   /api/v1/users
POST  /api/v1/users
GET   /api/v1/users/{user_id}
PATCH /api/v1/users/{user_id}

POST /api/v1/users/{user_id}:disable
POST /api/v1/users/{user_id}:enable
POST /api/v1/users/{user_id}:reset-password
```

- 仅管理员可调用。
- `PATCH` 只修改姓名、邮箱和角色。
- 状态和密码使用显式命令。
- V1 由管理员输入临时密码，不发送邮件，不实现首次登录强制改密。
- OA 身份没有本地密码时，改密和重置密码返回 `IDENTITY_NOT_LOCAL`。
- 密码、密码哈希和 Session 永远不进入响应、应用日志或审计 JSON。

## 5. 权限矩阵

| 能力 | 普通成员 | 管理员 |
| --- | --- | --- |
| 查看启用的评测目录与标签 | 是 | 是 |
| 管理用户、对象、场景、标签、评测集 | 否 | 是 |
| 创建独立评测 | 是 | 是 |
| 编辑、完成、重开或作废自己的评测 | 是 | 是 |
| 维护其他人的评测 | 否 | 是 |
| 登记和查看 Badcase | 是 | 是 |
| 为有效 Badcase 添加备注和截图 | 是 | 是 |
| 分配、调整状态和维护问题标签 | 是 | 是 |
| 修改 Badcase 原始问题内容 | 仅创建人 | 是 |
| 无效化或重新激活 Badcase | 仅创建人 | 是 |
| 查看审计日志 | 否 | 是 |

无效 Badcase 只能查看或重新激活，不能继续处理或添加定位材料。

## 6. 评测目录

### 6.1 评测对象

```http
GET   /api/v1/evaluation-targets
POST  /api/v1/evaluation-targets
GET   /api/v1/evaluation-targets/{target_id}
PATCH /api/v1/evaluation-targets/{target_id}

POST /api/v1/evaluation-targets/{target_id}:disable
POST /api/v1/evaluation-targets/{target_id}:enable
```

### 6.2 场景

```http
GET   /api/v1/scenarios
POST  /api/v1/scenarios
GET   /api/v1/scenarios/{scenario_id}
PATCH /api/v1/scenarios/{scenario_id}

POST /api/v1/scenarios/{scenario_id}:disable
POST /api/v1/scenarios/{scenario_id}:enable
```

列表支持 `evaluation_target_id/status/keyword`。场景产生历史数据后，`PATCH` 不允许修改评测对象归属。

评测对象是评测集和 Badcase 的强归属边界；只有启用对象可以创建评测集、维护与发布草稿、
开始新评测或登记业务 Badcase。场景是对象内的可选分类：选择场景时必须属于同一对象且处于
启用状态，但对象没有场景或暂不选择场景都不阻塞上述流程。停用不级联改写历史数据。

### 6.3 用例标签

用例标签分为 `global` 和 `scenario` 两种不可变作用域。全局标签不携带
`scenario_id`，场景标签必须携带。场景可用标签是启用的全局标签与当前场景启用
标签的并集。

```http
GET   /api/v1/case-tags
POST  /api/v1/case-tags
GET   /api/v1/scenarios/{scenario_id}/case-tags
POST  /api/v1/scenarios/{scenario_id}/case-tags
GET   /api/v1/scenarios/{scenario_id}/available-case-tags
PATCH /api/v1/case-tags/{tag_id}

POST /api/v1/case-tags/{tag_id}:disable
POST /api/v1/case-tags/{tag_id}:enable
POST /api/v1/scenarios/{scenario_id}/case-tags:reorder
```

### 6.4 问题标签

```http
GET   /api/v1/issue-tags
POST  /api/v1/issue-tags
PATCH /api/v1/issue-tags/{tag_id}

POST /api/v1/issue-tags/{tag_id}:disable
POST /api/v1/issue-tags/{tag_id}:enable
POST /api/v1/issue-tags:reorder
```

配置实体不提供 `DELETE`。普通成员只读取启用项，管理员可筛选停用项。新关联只能选择启用标签，历史关联继续展示停用标签。

批量排序请求：

```json
{
  "items": [
    {
      "id": "019c...",
      "sort_order": 10,
      "expected_lock_version": 2
    }
  ]
}
```

## 7. 评测集、版本与用例

### 7.1 评测集

```http
GET   /api/v1/datasets
POST  /api/v1/datasets
GET   /api/v1/datasets/{dataset_id}
PATCH /api/v1/datasets/{dataset_id}

POST /api/v1/datasets/{dataset_id}/archive
POST /api/v1/datasets/{dataset_id}/restore
```

创建评测集只选择 `evaluation_target_id`，在同一事务中自动创建首个草稿，并同时返回两者摘要。
列表支持 `evaluation_target_id/scenario_id/status/keyword`；其中 `scenario_id` 表示评测集内至少包含
一个归入该场景的用例，不代表评测集归属于场景。

### 7.2 评测集版本

```http
GET    /api/v1/datasets/{dataset_id}/versions
GET    /api/v1/dataset-versions/{version_id}
DELETE /api/v1/dataset-versions/{version_id}

POST /api/v1/datasets/{dataset_id}/drafts
POST /api/v1/dataset-versions/{version_id}/publish
POST /api/v1/dataset-versions/{version_id}/archive
```

创建草稿：

```json
{
  "base_version_id": "019c...",
  "expected_dataset_lock_version": 3
}
```

发布：

```json
{
  "release_note": "补充预算和交付周期类问题",
  "expected_lock_version": 7
}
```

- 同一个评测集最多一个草稿。
- 发布事务校验至少一个启用用例、固化标签名称、分配连续版本号并写审计。
- 只有草稿允许删除。
- 已发布和已归档版本内容不可修改或删除。

### 7.3 版本用例

```http
GET    /api/v1/dataset-versions/{version_id}/cases
POST   /api/v1/dataset-versions/{version_id}/cases
GET    /api/v1/version-cases/{case_id}
PATCH  /api/v1/version-cases/{case_id}
DELETE /api/v1/version-cases/{case_id}

POST /api/v1/dataset-versions/{version_id}/cases/reorder
```

- 只有草稿版本允许写入。
- `case_key` 和所属版本不可修改。
- 内容、启用状态、标签和可选 `scenario_id` 通过 `PATCH` 更新。
- 未选择场景时归类状态为 `unclassified`；人工选择场景后为 `confirmed`。
- 选择的场景必须属于评测集的对象。未归类用例只能选择全局用例标签。
- 用例列表分页加载，不一次返回最多 5,000 条。
- 排序操作全有或全无。

### 7.4 CSV

```http
GET  /api/v1/case-import-template.csv
POST /api/v1/dataset-versions/{version_id}/case-imports/preview
POST /api/v1/dataset-versions/{version_id}/case-imports/commit
GET  /api/v1/dataset-versions/{version_id}/cases.csv
```

预览使用 `multipart/form-data`，返回短期 `import_token`、草稿锁版本、预览行和精确到行及字段的错误。提交时重新验证 Token、草稿状态和锁版本，使用单个数据库事务追加全部用例。

Token 一次使用、短期有效，预览数据保存在 Redis。V1 只追加，不覆盖；存在错误行时不能提交。
CSV 不包含场景字段，导入用例统一以 `unclassified` 写入，后续可在草稿编辑器逐条归类。

## 8. 人工评测

### 8.1 EvaluationRun

```http
GET    /api/v1/evaluation-runs
POST   /api/v1/evaluation-runs
GET    /api/v1/evaluation-runs/{run_id}
PATCH  /api/v1/evaluation-runs/{run_id}
DELETE /api/v1/evaluation-runs/{run_id}

POST /api/v1/evaluation-runs/{run_id}/complete
POST /api/v1/evaluation-runs/{run_id}/reopen
POST /api/v1/evaluation-runs/{run_id}/void
```

创建请求：

```json
{
  "dataset_version_id": "019c...",
  "agent_version": "2026.07.3",
  "environment": "staging",
  "purpose_note": "提示词优化后复测",
  "config_note": "知识库版本 KB-42"
}
```

创建事务校验版本已发布，并为全部启用用例预生成 `pending` CaseResult。`PATCH` 只允许在进行中修改 Agent 版本、环境和备注。

完成命令必须确认不存在 `pending` 结果。重开和作废必须填写原因。删除仅允许从未完成且从未产生 Badcase 的评测。

### 8.2 评测工作台

```http
GET /api/v1/pages/evaluation-runs/{run_id}/workbench?page=1&page_size=50
```

响应聚合：

- 对象、评测集和版本上下文，以及每条用例各自的可选场景。
- EvaluationRun、锁版本和允许动作。
- 总数、待评、已评、跳过、已评分和 Badcase 数量。
- 当前页用例、CaseResult、附件和 Badcase 摘要。

### 8.3 CaseResult

```http
GET   /api/v1/evaluation-runs/{run_id}/case-results
GET   /api/v1/case-results/{result_id}
PATCH /api/v1/case-results/{result_id}
```

已评测：

```json
{
  "status": "evaluated",
  "answer_text": "Agent 实际回答",
  "score": 4,
  "comment": "基本正确，但缺少价格依据",
  "expected_lock_version": 2
}
```

跳过：

```json
{
  "status": "skipped",
  "skip_reason": "测试环境无采购权限",
  "expected_lock_version": 2
}
```

保存响应同时返回最新评测进度。已完成或已作废评测中的结果不可编辑，必须先显式重开。

### 8.4 标记评测 Badcase

```http
POST /api/v1/case-results/{result_id}/mark-badcase
```

```json
{
  "expected_result_lock_version": 2,
  "result_patch": {
    "status": "evaluated",
    "answer_text": "Agent 实际回答",
    "score": 2,
    "comment": "推荐了已下架商品"
  },
  "badcase": {
    "title": "推荐已下架商品",
    "description": "推荐结果当前无法购买",
    "issue_tag_ids": ["019c..."]
  }
}
```

`result_patch` 可省略。该事务保存结果、校验评语、创建或恢复唯一 Badcase、复制评测对象、
用例的可选场景、归类状态及 Agent 现场信息，创建标签和时间线，并返回结果、Badcase 摘要和评测进度。

## 9. Badcase 中心

### 9.1 列表、创建与内容修改

```http
GET   /api/v1/badcases
POST  /api/v1/badcases
GET   /api/v1/badcases/{badcase_id}
PATCH /api/v1/badcases/{badcase_id}
PUT   /api/v1/badcases/{badcase_id}/issue-tags
```

`POST /badcases` 只创建业务来源；评测来源必须通过 CaseResult 命令创建。列表支持对象、场景、来源、状态、负责人、标签、环境、Agent 版本、时间和关键词筛选，默认排除无效记录。
业务 Badcase 的标题必填，问题描述可省略或传 `null`；空白描述会归一化为 `null`。

业务登记请求：

```json
{
  "evaluation_target_id": "019c...",
  "scenario_id": null,
  "title": "采购助手未识别预算条件",
  "description": null,
  "agent_response_text": "...",
  "agent_version": "2026.07.3",
  "environment": "production",
  "occurred_at": "2026-07-25T06:20:00.000Z",
  "business_reference": "ORDER-20260725-001",
  "session_id": "chat-88af",
  "issue_tag_ids": ["019c..."]
}
```

`evaluation_target_id` 必填，`scenario_id` 可省略或传 `null`；选择时必须属于该对象。创建后对象和来源
不可修改；`PATCH` 可修改标题、描述、回答现场、可选场景、Agent 版本、环境、发生时间、业务单号和
会话 ID。场景传 `null` 可恢复为待归类状态，状态、负责人和有效性仍通过专用命令修改。

### 9.2 Badcase 页面聚合

```http
GET /api/v1/pages/badcases/{badcase_id}
```

响应包含 Badcase、对象与可选场景、来源评测上下文、原始回答与截图、补充截图、问题标签、处理时间线、候选负责人、候选标签和允许动作。

### 9.3 处理命令

```http
POST /api/v1/badcases/{badcase_id}:assign
POST /api/v1/badcases/{badcase_id}:unassign

POST /api/v1/badcases/{badcase_id}:start-processing
POST /api/v1/badcases/{badcase_id}:resolve
POST /api/v1/badcases/{badcase_id}:defer
POST /api/v1/badcases/{badcase_id}:reopen

POST /api/v1/badcases/{badcase_id}:add-note
POST /api/v1/badcases/{badcase_id}:invalidate
POST /api/v1/badcases/{badcase_id}:reactivate
```

所有命令更新当前记录、增加不可变时间线并返回新锁版本。状态变化、无效化和重新激活必须有说明。无效记录必须先重新激活才能继续处理。

## 10. 附件

```http
POST   /api/v1/case-results/{result_id}/attachments
POST   /api/v1/badcases/{badcase_id}/attachments
GET    /api/v1/attachments/{attachment_id}/content
DELETE /api/v1/attachments/{attachment_id}

POST /api/v1/case-results/{result_id}/attachments/reorder
POST /api/v1/badcases/{badcase_id}/attachments/reorder
```

- 上传使用 multipart 字段 `files` 和 `expected_owner_lock_version`。
- 一次可以上传多张，但所属记录累计最多 10 张，单张最大 10 MB。
- 后端验证实际图片类型并生成文件名，响应不返回 `storage_path`。
- 附件不单独维护锁；上传、删除和排序递增所属 CaseResult 或 Badcase 的锁版本。
- 删除使用 `?expected_owner_lock_version={version}`。
- 没有回答文本时，删除 `evaluated` 结果的最后一张截图返回 `409 EVIDENCE_REQUIRED`。
- 评测来源 Badcase 展示 CaseResult 原始截图，不复制文件。
- 图片内容需要 Session 鉴权，并使用私有缓存策略。

权限：评测截图由评测创建人或管理员维护；所有成员可以为有效 Badcase 补充定位截图；Badcase 截图只能由上传者、Badcase 创建人或管理员删除。

## 11. 首页、看板、搜索与导出

### 11.1 页面聚合

```http
GET /api/v1/pages/home
GET /api/v1/pages/dashboard
GET /api/v1/pages/datasets/{dataset_id}/version-comparison
GET /api/v1/evaluation-results
```

首页返回当前用户进行中的评测、分配给自己的 Badcase、最近发布的评测集和近期活动。

看板和版本比较支持：

```text
evaluation_target_id
scenario_id
dataset_id
dataset_version_id
evaluator_id
agent_version
environment
source_type
badcase_status
issue_tag_id
from
to
```

统计口径：

- 评测平均分、评分分布和评测 Badcase 率只统计已完成评测。
- 评测 Badcase 率只统计评测来源，分母为已完成评测的已评用例。
- Badcase 中心统计全部有效 Badcase，包括业务来源和进行中评测发现的问题。
- 无效 Badcase 不进入正式统计。
- 跳过结果不进入平均分和 Badcase 率分母。

`/evaluation-results` 只返回已完成评测的用例结果，沿用上述评测维度，并额外支持
`result_status`、`score`、`skip_reason`、`has_badcase`、`scored` 和 `keyword`。
响应同时返回命中的结果数和去重后的已完成评测次数，用于从指标卡、评分分布、
跳过原因及版本对比精确核对明细。

### 11.2 搜索

```http
GET /api/v1/search?q={keyword}&types=target,scenario,dataset,case,evaluation_result,badcase&page=1&page_size=20
```

全局搜索用于顶部快速定位评测对象、场景、评测集、用例、普通评测回答和 Badcase。
用例结果精确跳转到只读快照或对应评测结果；复杂 Badcase 查询使用 `/badcases`
组合筛选。

### 11.3 CSV 导出

```http
GET /api/v1/exports/evaluation-results.csv
GET /api/v1/exports/badcases.csv
GET /api/v1/exports/badcase-distribution.csv
```

- 沿用列表和看板筛选参数。
- 同步流式生成 UTF-8 BOM CSV。
- 单次最多 50,000 行，超出返回 `422 EXPORT_TOO_LARGE`。
- 截图列只返回鉴权 API URL，不返回服务器目录。
- 导出接口必须登录，不生成公开下载链接。

## 12. 主要业务错误码

| 错误码 | HTTP | 含义 |
| --- | ---: | --- |
| `AUTH_REQUIRED` | 401 | Session 不存在或已失效 |
| `CSRF_INVALID` | 403 | CSRF Token 缺失或无效 |
| `FORBIDDEN` | 403 | 当前用户无操作权限 |
| `RESOURCE_NOT_FOUND` | 404 | 资源不存在或不可见 |
| `VALIDATION_FAILED` | 422 | 字段或业务语义校验失败 |
| `LOCK_VERSION_CONFLICT` | 409 | 乐观锁冲突 |
| `NAME_CONFLICT` | 409 | 当前作用域名称重复 |
| `INVALID_STATE_TRANSITION` | 409 | 不允许的生命周期迁移 |
| `RESOURCE_DISABLED` | 409 | 资源已停用，不能执行新操作 |
| `RELATIONSHIP_LOCKED` | 409 | 历史数据存在，不能修改归属 |
| `DRAFT_ALREADY_EXISTS` | 409 | 评测集已有草稿 |
| `PUBLISHED_VERSION_IMMUTABLE` | 409 | 已发布版本不可编辑 |
| `DATASET_VERSION_EMPTY` | 422 | 发布版本没有启用用例 |
| `IMPORT_TOKEN_EXPIRED` | 409 | CSV 预览 Token 已过期 |
| `IMPORT_SOURCE_CHANGED` | 409 | 预览后草稿发生变化 |
| `EVALUATION_HAS_PENDING_CASES` | 409 | 仍有待评用例，不能完成 |
| `EVIDENCE_REQUIRED` | 409 | 已评结果缺少文本和截图证据 |
| `SKIP_REASON_REQUIRED` | 422 | 跳过结果未填写原因 |
| `BADCASE_ALREADY_EXISTS` | 409 | 用例结果已产生 Badcase |
| `BADCASE_INVALIDATED` | 409 | 无效 Badcase 不能继续处理 |
| `ATTACHMENT_LIMIT_EXCEEDED` | 413 | 截图数量超过限制 |
| `UNSUPPORTED_MEDIA_TYPE` | 415 | 文件不是允许的图片类型 |
| `EXPORT_TOO_LARGE` | 422 | 导出行数超过同步限制 |
| `IDEMPOTENCY_KEY_REUSED` | 409 | 同一幂等 Key 被用于不同请求 |
| `IDENTITY_NOT_LOCAL` | 409 | OA 身份不能执行本地密码操作 |

字段错误使用稳定的字段路径，例如 `badcase.issue_tag_ids[0]` 或 `items[3].sort_order`。

## 13. 服务端事务边界

以下操作必须在 Go Service 中形成清晰事务，不能由前端拆成多个写请求自行编排：

- 创建评测集并创建首个草稿。
- 从已发布版本复制草稿和用例标签快照。
- 发布版本、分配连续版本号并固化内容。
- CSV 提交并追加全部用例。
- 创建 EvaluationRun 并生成全部 CaseResult。
- 完成、重开和作废 EvaluationRun。
- 保存结果并标记或恢复评测 Badcase。
- Badcase 状态、负责人、有效性和时间线同步更新。
- 批量标签及排序替换。

文件系统不能参与 MySQL 事务。附件上传使用临时文件、内容校验、数据库元数据提交、原子移动和失败补偿；定期任务清理孤立文件。

## 14. OpenAPI 落地要求

正式实现时生成 `openapi/quickeval-v1.yaml`，要求：

- 每个资源 DTO、列表、错误、命令请求和聚合响应定义可复用 Schema。
- Cookie Session 与 `X-CSRF-Token` 分别定义安全方案和 Header 参数。
- 文件上传和 CSV 下载定义准确 Content-Type。
- 状态与类型字段使用字符串枚举。
- UUID、UTC 时间和字段长度定义格式约束。
- 所有命令列出可能的 `409/422` 业务错误码。
- 由 OpenAPI 生成前端 TypeScript 类型和客户端基础代码。
- 后端 DTO 与数据库模型分离，不直接将 GORM 实体暴露为 API Schema。
