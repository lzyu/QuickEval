# KooEval

KooEval 是面向团队的 Agent 人工评测与 Badcase 管理系统。团队可以围绕评测对象、评测集和评测用例开展人工评测，留存评分、回答和截图证据，并持续登记、归类和跟进 Badcase。

## 文档

- [部署与运行手册](./docs/deployment-guide.md)：环境准备、构建打包、配置、迁移、启动、反向代理及升级检查。
- [功能使用说明](./docs/user-guide.md)：面向运营人员、运营管理员和普通成员的功能介绍与推荐流程。
- [产品规格](./docs/quickeval-v1-spec.md)、[API 设计](./docs/quickeval-v1-api-design.md)、[数据库设计](./docs/quickeval-v1-database-design.md)：研发参考资料。

## 技术栈

- 前端：Vue 3、TypeScript、Vite、Element Plus。
- 后端：Go、Gin、GORM。
- 数据：MySQL 8.4 LTS、Redis 7。
- 生产运行：Linux 虚机、Nginx、systemd；生产环境不依赖 Docker。

## 目录

```text
apps/
├── api/                 Go API 与命令行工具
└── web/                 Vue 单页应用
config/                  配置模板；真实配置不提交
docs/                    产品、设计、部署和使用文档
migrations/              MySQL Migration
openapi/                 OpenAPI 契约
page_refer/              页面视觉参考
```

## 本地快速启动

前置条件：Go 1.26.x、Node.js 22 LTS、Docker（仅用于启动本地 MySQL/Redis）。

```bash
cp config/quickeval.example.yaml config/quickeval.yaml
cp config/secrets.example.yaml config/secrets.yaml
make web-install
make infra-up
make migrate-up
```

编辑 `config/secrets.yaml`，使其中的 MySQL、Redis 密码与开发 Compose 一致，并将 `security.session_secret` 替换为至少 32 个字符的随机值。随后创建首次使用的超级管理员：

```bash
QUICKEVAL_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  make bootstrap-admin
```

分别启动 API 和前端：

```bash
make api-run
make web-dev
```

- 前端：`http://127.0.0.1:5173`
- API 存活检查：`http://127.0.0.1:8080/health/live`
- API 就绪检查：`http://127.0.0.1:8080/health/ready`

停止本地依赖：`make infra-down`。该命令保留 MySQL 和 Redis 数据卷，不会删除开发数据。

## 质量验证

```bash
make check
make build
npm --prefix apps/web run test:e2e
```

前端 API 类型由 [`openapi/quickeval-v1.yaml`](./openapi/quickeval-v1.yaml) 生成；不要手工修改 `apps/web/src/api/generated/` 中的生成文件。
