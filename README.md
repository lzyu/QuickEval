# QuickEval

QuickEval 是面向小团队的轻量级 Agent 人工评测系统，主要用于云市场智慧助手和智能采购 Agent 的黑盒评测。

当前仓库正在实施 V1。产品、数据库、API、前端和实施计划位于 [`docs`](./docs)。

## 技术栈

- 前端：Vue 3、TypeScript、Vite、Element Plus。
- 后端：Go、Gin、GORM。
- 数据：MySQL 8.4 LTS、Redis。
- 部署：Linux 虚机、Nginx、systemd，不依赖 Docker。

## 目录

```text
apps/
├── api/                 Go API、Migration 命令
└── web/                 Vue SPA
config/                  配置模板；真实配置不提交
docs/                    产品与技术设计
migrations/              MySQL Migration
openapi/                 OpenAPI 契约
page_refer/              高保真页面参考
```

## 本地准备

本地需要 Go 1.26.x、Node.js 22 LTS。MySQL 8.4 和 Redis 7 可以使用本机已有服务，也可以使用仓库提供的开发 Compose；生产部署仍不依赖 Docker。

复制配置模板：

```bash
cp config/quickeval.example.yaml config/quickeval.yaml
cp config/secrets.example.yaml config/secrets.yaml
```

修改 MySQL、Redis 和随机 Session Secret，然后执行：

```bash
make web-install
make infra-up
make migrate-up
```

首次启动前创建唯一的初始管理员。密码只通过环境变量传入，不进入命令参数或配置仓库：

```bash
QUICKEVAL_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  make bootstrap-admin
```

分别启动 API 和前端：

```bash
make api-run
make web-dev
```

- Vue：`http://127.0.0.1:5173`
- API liveness：`http://127.0.0.1:8080/health/live`
- API readiness：`http://127.0.0.1:8080/health/ready`

## 验证

```bash
make check
make build
npm --prefix apps/web run test:e2e
```

前端 API 类型由 [`openapi/quickeval-v1.yaml`](./openapi/quickeval-v1.yaml) 生成，不要手工修改生成文件。

API 启动后可以运行 M1～M6 的本机真实链路验收：

```bash
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m1_smoke.sh
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m2_smoke.sh
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m3_smoke.sh
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m4_smoke.sh
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m5_smoke.sh
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m6_smoke.sh
```

M4 验收会使用 `page_refer` 中的 PNG，验证图片真实类型、私有鉴权读取、
`uploads/evaluations/{run}/{result}` 落盘、截图-only 证据、附件与 Badcase 幂等、
删除保护及跨用户权限。

M5 验收会验证业务 Badcase 的幂等登记、多截图落盘、组合详情聚合、负责人和
问题标签维护、状态迁移、处理备注幂等、无效化/重新激活、创建人权限以及不可变
活动时间线。业务截图按 `uploads/badcases/{badcase_id}/attachments` 隔离存放。

M6 验收会创建隔离的 V1/V2 评测样本，核对个人首页、只统计已完成评测的看板口径、
评分与 Badcase 率分母、有效 Badcase 分布、版本对比、空样本、评分/跳过/Badcase
钻取、普通评测回答搜索，以及带 UTF-8 BOM、评测汇总字段和鉴权截图地址的三个
CSV 导出。

停止本地开发依赖时执行 `make infra-down`。该命令保留 MySQL 和 Redis 数据卷；如需删除开发数据，应先明确检查目标卷。
