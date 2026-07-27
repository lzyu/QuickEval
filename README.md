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

API 启动后可以运行 M1 的本机真实链路验收：

```bash
QUICKEVAL_SMOKE_PASSWORD='your-admin-password' scripts/m1_smoke.sh
```

停止本地开发依赖时执行 `make infra-down`。该命令保留 MySQL 和 Redis 数据卷；如需删除开发数据，应先明确检查目标卷。
