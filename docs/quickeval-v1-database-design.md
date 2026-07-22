# QuickEval V1 领域模型与数据库设计

> 状态：设计基线已确认
>
> 数据库：MySQL 8.4 LTS
>
> 更新日期：2026-07-23
>
> 配套 DDL：[000001_initial_schema.up.sql](../migrations/000001_initial_schema.up.sql)

## 1. 设计目标

本设计支撑 QuickEval V1 的人工评测、评测集版本、统一 Badcase、截图附件、汇总查询和审计能力。

核心原则：

- MySQL 是业务数据的唯一事实来源，Redis 不保存不可恢复的业务数据。
- 使用模块化单体，不按服务拆分数据库。
- 所有表使用应用生成的 UUIDv7 主键，API 使用标准 UUID 字符串，MySQL 使用 `BINARY(16)`。
- 已发布版本保存不可变用例快照；历史数据通过状态保留，不使用全局软删除。
- 单行可验证的规则使用 MySQL 约束，跨表规则由 Go Service 在事务中保证。
- V1 基于明细表实时聚合，不维护统计汇总表或 Redis 业务缓存。

## 2. 容量假设

| 数据 | V1 设计容量 |
| --- | ---: |
| 用户 | 100 以内 |
| 评测对象 | 100 以内 |
| 场景 | 1,000 以内 |
| 版本用例快照 | 100,000 以内 |
| 评测记录 | 100,000 以内 |
| 用例结果 | 1,000,000 以内 |
| Badcase | 100,000 以内 |
| 附件元数据 | 1,000,000 以内 |
| 单个评测集的启用用例 | 5,000 以内 |
| 单次 CSV 导入 | 10,000 行以内 |

在此容量下不采用分库分表、表分区、全文索引或预计算统计。所有列表默认每页 20 条，最大 100 条。

## 3. 领域关系

```text
User
└── UserIdentity

EvaluationTarget
└── Scenario
    ├── CaseTag
    ├── Dataset
    │   └── DatasetVersion
    │       └── VersionCase
    │           └── VersionCaseTag
    └── Badcase (business source)

DatasetVersion
└── EvaluationRun
    └── CaseResult
        ├── Attachment
        └── Badcase (evaluation source)

Badcase
├── BadcaseIssueTag ── IssueTag
├── BadcaseActivity
└── Attachment
```

领域术语的权威定义见仓库根目录 [CONTEXT.md](../CONTEXT.md)。

## 4. 通用数据库规范

### 4.1 主键与 UUID

- 所有表的 `id` 为 `BINARY(16)`。
- UUIDv7 由 Go 应用生成，不使用数据库函数生成。
- Repository 负责标准 UUID 字符串与 16 字节值的转换。
- `version_cases.case_key` 也是 UUIDv7，用于保持同一用例跨版本的逻辑身份。
- 不使用 `CHAR(36)` UUID，避免扩大聚簇索引和所有二级索引。

### 4.2 字符集与时间

- 默认字符集：`utf8mb4`。
- 默认排序规则：`utf8mb4_0900_ai_ci`。
- 数据库时间统一使用 UTC `DATETIME(3)`。
- API 时间使用 ISO 8601 UTC 格式，前端负责本地时区展示。
- `created_at/updated_at` 是记录时间；`occurred_at/completed_at/published_at` 等表达业务事件时间。

### 4.3 状态与类型

- 状态、角色、来源和环境使用 `VARCHAR + CHECK`。
- 不使用 MySQL `ENUM` 或难以理解的数字状态码。
- Go 代码使用自定义字符串类型与常量。

### 4.4 删除与历史保留

- 不增加通用 `deleted_at`。
- 从未形成历史价值的草稿可以物理删除。
- 已发布版本只能归档。
- 已完成或曾经完成的评测只能作废。
- Badcase 可以无效化，不物理删除。
- 用户、评测对象、场景和标签只能停用或归档。
- 外键对历史主数据使用 `RESTRICT`，草稿内部关系可以 `CASCADE`。

### 4.5 乐观锁

主要可变实体使用：

```text
lock_version INT UNSIGNED NOT NULL DEFAULT 0
```

更新语句携带读取时的版本并递增。影响行数为零时，API 返回 `409 Conflict`。

### 4.6 文本类型

| 内容 | 类型 |
| --- | --- |
| 名称、版本、业务单号、会话 ID | `VARCHAR` |
| 简短说明、跳过及作废原因 | `TEXT` |
| 问题、回答、参考内容、定位描述、处理备注 | `MEDIUMTEXT` |

核心文本不存入 JSON。应用层限制名称 200 字符、普通说明 10,000 字符、问题和回答类文本 1,000,000 字符；数据库启用严格模式，不允许静默截断。

## 5. 表清单

| 模块 | 表 | 职责 |
| --- | --- | --- |
| 账号 | `users` | QuickEval 内部用户与角色 |
| 账号 | `user_identities` | 本地或 OA 登录身份 |
| 评测结构 | `evaluation_targets` | 被评测的 Agent 产品 |
| 评测结构 | `scenarios` | 评测与 Badcase 的业务分类边界 |
| 评测结构 | `case_tags` | 场景级用例分类 |
| 评测集 | `datasets` | 跨版本稳定的评测集 |
| 评测集 | `dataset_versions` | 草稿、发布及归档版本 |
| 评测集 | `version_cases` | 版本中的用例内容快照 |
| 评测集 | `version_case_tags` | 用例标签及名称快照 |
| 人工评测 | `evaluation_runs` | 一名人员的一次独立评测 |
| 人工评测 | `case_results` | 每条用例的状态、证据和判断 |
| Badcase | `badcases` | 两种来源的统一质量问题记录 |
| Badcase | `issue_tags` | 全局问题分类 |
| Badcase | `badcase_issue_tags` | Badcase 与问题标签的多对多关系 |
| Badcase | `badcase_activities` | 用户可见的不可变处理时间线 |
| 文件 | `attachments` | 截图元数据和所属关系 |
| 审计 | `audit_logs` | 管理和关键状态操作审计 |

V1 不建立 Session、角色权限、Agent 版本、环境字典、统计汇总或 CSV 任务表。

## 6. 聚合与状态模型

### 6.1 评测集版本

```text
draft ── publish ──> published ── archive ──> archived
```

- 一个评测集最多存在一个草稿。
- 草稿的 `version_no` 为空，发布时事务内分配连续版本号。
- 已发布和已归档版本内容不可修改。
- 草稿可以从任意已发布版本复制，`base_version_id` 记录来源。
- 发布版本至少包含一条启用用例。
- 已有用例复制时保留 `case_key`，新增用例生成新值。
- 发布快照包括内容、顺序、启用状态、用例标签关联和标签显示名称。

同一评测集最多一个草稿是跨行规则，由 Service 事务保证。建议复制草稿时锁定 `datasets` 记录，发布分配版本号时使用相同锁，避免并发生成两个草稿或相同版本号。

### 6.2 人工评测

```text
in_progress ── complete ──> completed
     ^                         │
     └──────── reopen ─────────┘

in_progress/completed ── void ──> voided
```

- 创建评测时在同一事务中为所有启用用例生成 `pending` 结果。
- `case_results` 状态为 `pending/evaluated/skipped`。
- `evaluated` 必须有回答文本或至少一张截图，评分可空。
- `skipped` 必须有原因，不评分、不能产生 Badcase。
- 没有 `pending` 结果时才能完成评测。
- 完成后结果锁定；重新打开后退出正式汇总并可修改。
- `first_completed_at` 永久标记该记录是否曾经完成。只有从未完成且从未产生 Badcase 的记录允许物理删除；已经产生 Badcase 的未完成评测也只能作废。
- `voided` 是终态。

### 6.3 Badcase

```text
pending ──> processing ──> resolved
    └───────────────> deferred
```

- `source_type` 为 `evaluation/business`。
- 评测来源必须唯一关联一个 `case_result`；业务来源不关联结果。
- `badcases` 是用例结果是否为 Badcase 的唯一事实来源，`case_results` 不保存重复布尔值。
- `scenario_id` 直接保存在 Badcase 中；评测来源必须与结果所属场景一致。
- Agent 版本和环境保存为 Badcase 发现时的现场快照。
- 无效化独立于处理状态；无效 Badcase 不计入任何正式统计。
- 当前状态保存在 `badcases`，变化历史追加到 `badcase_activities`。

## 7. 关键一致性规则

### 7.1 数据库直接保证

- UUID 主键、唯一名称和外键完整性。
- 分数为空或在 1～5 范围内。
- 附件只能属于 CaseResult 或 Badcase 其中之一。
- 评测来源 Badcase 必须关联 CaseResult，业务来源不能关联。
- 跳过结果必须填写原因且不能评分。
- 一条 CaseResult 最多产生一条 Badcase。
- 状态、角色、来源和环境值在允许集合内。
- 关键状态与事件时间字段组合合法。

### 7.2 Go Service 事务保证

- 场景产生历史数据后不能更换评测对象。
- 评测集产生已发布版本后不能更换场景。
- 同一评测集最多存在一个草稿。
- 发布版本号连续且不复用。
- 发布版本至少包含一条启用用例。
- 已发布版本内容不可修改。
- 用例标签必须属于评测集所在场景。
- 评测来源 Badcase 的场景与 CaseResult 一致。
- `evaluated` 结果具备回答文本或截图。
- 标记 Badcase 时用例结果评语非空。
- 所有结果处理完成后才能完成 EvaluationRun。
- 跳过结果不能产生 Badcase。
- 从未完成但已经产生 Badcase 的 EvaluationRun 不能物理删除。
- 每条结果或 Badcase 最多 10 张截图，单张最大 10 MB。

## 8. 搜索与统计口径

### 8.1 搜索

- 名称、状态、人员、版本、环境和时间使用普通索引。
- 业务单号和会话 ID 使用等值或前缀查询。
- 问题、回答和 Badcase 描述使用 `%关键词%`。
- V1 不使用 `FULLTEXT`，所有列表强制分页。
- 查询先通过场景、版本或时间缩小范围，再搜索长文本。

### 8.2 汇总

- 只统计 `evaluation_runs.status = 'completed'`。
- 平均分只包含 `case_results.status = 'evaluated' AND score IS NOT NULL`。
- 跳过结果不进入评分和 Badcase 比例分母。
- Badcase 只统计 `invalidated_at IS NULL`。
- 一个 Badcase 可以计入多个问题标签分布。
- 所有看板从明细表实时聚合，不预计算、不写 Redis。

## 9. 附件一致性

- `attachments.storage_path` 保存相对于程序同级 `uploads` 的路径。
- 评测截图关联 `case_result_id`；业务或补充定位截图关联 `badcase_id`。
- 评测来源 Badcase 通过 CaseResult 展示原始截图，不复制文件。
- 数据库事务与文件系统无法组成原子事务：应用先写临时文件，校验成功后提交元数据并原子移动文件。
- 删除未完成草稿时先记录待清理路径；数据库提交后删除物理文件，失败则由定期孤儿清理任务补偿。
- 文件名使用 UUID，上传时验证扩展名、Content-Type 和实际解码结果。

## 10. 审计边界

- `badcase_activities` 是用户可见的业务时间线。
- `audit_logs` 是管理员审计记录，两者不能合并。
- 审计发布、归档、完成、重新打开、作废、停用、密码重置和 Badcase 无效化等关键动作。
- 不审计每次自动保存，不将密码哈希、Session、密钥或整段大文本写入 JSON。
- 审计记录只追加，不更新、不删除。

## 11. 索引设计

### 11.1 评测结构

```text
users:                  UNIQUE(email), (status, created_at)
user_identities:        UNIQUE(provider, provider_subject), UNIQUE(user_id, provider)
evaluation_targets:     UNIQUE(name), (status, updated_at)
scenarios:              UNIQUE(evaluation_target_id, name), (evaluation_target_id, status, updated_at)
case_tags:              UNIQUE(scenario_id, name), (scenario_id, status, sort_order)
datasets:               UNIQUE(scenario_id, name), (scenario_id, status, updated_at)
dataset_versions:       UNIQUE(dataset_id, version_no), (dataset_id, status, created_at)
version_cases:          UNIQUE(dataset_version_id, case_key), (dataset_version_id, is_enabled, sort_order), (case_key, dataset_version_id)
version_case_tags:      UNIQUE(version_case_id, case_tag_id), (case_tag_id, version_case_id)
```

### 11.2 评测与 Badcase

```text
evaluation_runs:        (dataset_version_id, status, completed_at)
                        (evaluator_id, status, completed_at)
                        (status, completed_at)
                        (environment, status, completed_at)
                        (agent_version, status, completed_at)

case_results:           UNIQUE(evaluation_run_id, version_case_id)
                        (evaluation_run_id, status, score)
                        (version_case_id, status)

badcases:               UNIQUE(case_result_id)
                        (scenario_id, invalidated_at, status, occurred_at)
                        (assignee_id, invalidated_at, status, updated_at)
                        (source_type, invalidated_at, occurred_at)
                        (environment, invalidated_at, occurred_at)
                        (agent_version, invalidated_at, occurred_at)
                        (scenario_id, business_reference)
                        (scenario_id, session_id)

issue_tags:             UNIQUE(name), (status, sort_order)
badcase_issue_tags:     UNIQUE(badcase_id, issue_tag_id), (issue_tag_id, badcase_id)
badcase_activities:     (badcase_id, created_at, id), (actor_id, created_at)
```

### 11.3 文件与审计

```text
attachments:            UNIQUE(storage_path)
                        (case_result_id, sort_order)
                        (badcase_id, sort_order)
                        (sha256)

audit_logs:             (entity_type, entity_id, created_at)
                        (actor_user_id, created_at)
                        (request_id)
                        (created_at)
```

上线后使用慢查询日志和 `EXPLAIN ANALYZE` 验证索引；不预先增加没有真实查询支撑的组合索引。

## 12. 迁移与上线规则

- 生产环境不使用 GORM `AutoMigrate`。
- 数据结构通过版本化 SQL migration 显式变更。
- 迁移执行与应用启动分离；部署先备份、执行迁移，再切换程序版本。
- DDL 失败时停止发布，不允许应用在未知结构上启动。
- 应用启动只检查数据库连接和已应用 schema 版本，不自动修改表。
- 初始化管理员通过独立 CLI 创建，`users.created_by` 在该场景允许为空。
