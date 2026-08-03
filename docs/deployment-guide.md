# KooEval 部署与运行手册

本文档说明如何在 Linux 虚机上构建、安装、配置并运行当前版本的 KooEval。生产环境由 Nginx 提供静态页面和反向代理，Go API 由 systemd 托管；MySQL 与 Redis 使用已有服务，不需要 Docker。

## 1. 部署拓扑与职责

```text
浏览器 ── HTTPS ── Nginx ── /            → Vue 静态文件
                           ├─ /api/*     → Go API（127.0.0.1:8080）
                           └─ /health/*  → Go 健康检查

Go API ── MySQL 8.4（业务数据）
       ├─ Redis 7（会话、限流、导入预览与幂等记录）
       └─ /opt/kooeval/uploads（私有截图附件）
```

`uploads` 不应由 Nginx 作为静态目录公开；附件必须经 API 的登录和权限校验读取。

## 2. 前置条件

### 构建机

- Go 1.26.x（项目指定 toolchain 为 Go 1.26.4）。
- Node.js 22 LTS 与 npm。
- 可访问依赖源。

### 运行服务器

- Linux、systemd、Nginx。
- 可访问 MySQL 8.4 与 Redis 7；应用服务器不需要安装 Go、Node.js 或 Docker。
- 为上传文件、配置和日志准备持久磁盘空间。上传单文件上限默认为 10 MB，建议按实际截图量预留容量并纳入备份。
- 仅向外暴露 Nginx 的 80/443 端口；Go API 监听回环地址。

### 数据服务准备

- 创建 UTF-8 数据库和专用账号。执行 Migration 的账号必须拥有当前迁移所需的 DDL 权限；应用运行账号至少应拥有业务库所需的数据读写权限。
- Redis 必须启用密码，并允许应用服务器访问。
- 应用、MySQL、Redis 的时间建议统一使用 UTC，避免时间筛选和报表出现歧义。

## 3. 构建与打包

在仓库根目录的构建机执行以下命令。`make build` 会构建 API 服务与前端静态资源；Migration 和首次管理员工具需额外构建，供服务器侧执行。

```bash
make web-install
make check
make build

mkdir -p bin
(cd apps/api && go build -o ../../bin/kooeval-migrate ./cmd/migrate)
(cd apps/api && go build -o ../../bin/kooeval-bootstrap-admin ./cmd/bootstrap-admin)

tar -czf kooeval-release.tar.gz \
  bin/quickeval \
  bin/kooeval-migrate \
  bin/kooeval-bootstrap-admin \
  apps/web/dist \
  migrations \
  config/quickeval.example.yaml \
  config/secrets.example.yaml
```

发布包不包含真实的 `config/quickeval.yaml`、`config/secrets.yaml`、`uploads/` 和 `logs/`；这些文件和目录必须在服务器上独立保留。

## 4. 安装目录与运行用户

以下示例使用 `/opt/kooeval`，并以独立的 `kooeval` 用户运行服务。首次安装时由具备 sudo 权限的账号执行：

```bash
sudo useradd --system --home /opt/kooeval --shell /usr/sbin/nologin kooeval
sudo install -d -o kooeval -g kooeval -m 0750 \
  /opt/kooeval /opt/kooeval/web /opt/kooeval/migrations \
  /opt/kooeval/config /opt/kooeval/uploads /opt/kooeval/logs

sudo tar -xzf kooeval-release.tar.gz -C /tmp
sudo install -o kooeval -g kooeval -m 0755 /tmp/bin/quickeval /opt/kooeval/quickeval
sudo install -o kooeval -g kooeval -m 0755 /tmp/bin/kooeval-migrate /opt/kooeval/kooeval-migrate
sudo install -o kooeval -g kooeval -m 0755 /tmp/bin/kooeval-bootstrap-admin /opt/kooeval/kooeval-bootstrap-admin
sudo rsync -a --delete /tmp/apps/web/dist/ /opt/kooeval/web/
sudo rsync -a --delete /tmp/migrations/ /opt/kooeval/migrations/
sudo chown -R kooeval:kooeval /opt/kooeval/web /opt/kooeval/migrations
```

Go 程序会以二进制所在目录作为运行根目录，因此二进制、`config/`、`migrations/`、`uploads/` 和 `logs/` 必须同级放置。升级时不要对 `config/`、`uploads/` 或 `logs/` 使用 `--delete`。

## 5. 应用配置

复制模板并编辑服务器上的真实配置：

```bash
sudo install -o kooeval -g kooeval -m 0640 \
  /tmp/config/quickeval.example.yaml /opt/kooeval/config/quickeval.yaml
sudo install -o kooeval -g kooeval -m 0600 \
  /tmp/config/secrets.example.yaml /opt/kooeval/config/secrets.yaml
```

生产配置的关键项如下：

```yaml
# /opt/kooeval/config/quickeval.yaml
app:
  environment: production

http:
  address: 127.0.0.1:8080

mysql:
  host: mysql.internal.example
  port: 3306
  user: kooeval
  database: kooeval
  parameters: charset=utf8mb4

redis:
  host: redis.internal.example
  port: 6379
  database: 0

paths:
  migrations: migrations
  uploads: uploads

security:
  session_cookie: kooeval_session
  session_ttl: 12h
  cookie_secure: true
  login_max_attempts: 5
  login_window: 15m
  password_min_length: 10
```

```yaml
# /opt/kooeval/config/secrets.yaml
mysql:
  password: replace-with-mysql-password

redis:
  password: replace-with-redis-password

security:
  # 例如：openssl rand -hex 48
  session_secret: replace-with-at-least-32-random-characters
```

注意事项：

- `security.session_secret` 必须至少 32 个字符；泄露或更换该值会使既有登录会话失效。
- HTTPS 部署必须设置 `cookie_secure: true`。仅在纯 HTTP 的本地调试时使用 `false`。
- 配置中的 `migrations`、`uploads` 与 `log.file` 仅允许相对路径，真实密钥文件权限应为 `0600`。
- 当前允许上传 PNG、JPEG、WebP；默认单个文件不超过 10 MB、每条记录最多 10 个附件。若修改限制，Nginx 的 `client_max_body_size` 也要同步调整。

## 6. 数据库迁移与首次管理员

先确认 MySQL、Redis 可用，再执行迁移。迁移应与应用切换分开执行：先备份，再迁移，最后发布新版本。

```bash
sudo -u kooeval /opt/kooeval/kooeval-migrate -direction up
```

全新安装后创建唯一的超级管理员。密码只通过环境变量传递，不写入命令历史参数或配置文件：

```bash
sudo -u kooeval env QUICKEVAL_BOOTSTRAP_PASSWORD='replace-with-a-strong-password' \
  /opt/kooeval/kooeval-bootstrap-admin \
  --username admin \
  --display-name '系统超级管理员' \
  --email admin@example.com
```

首次管理员创建成功后，后续用户由“用户管理”创建。新建用户的初始密码固定为 `123456`，首次登录必须自行设置新密码。

## 7. systemd 服务

创建 `/etc/systemd/system/kooeval.service`：

```ini
[Unit]
Description=KooEval API
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=kooeval
Group=kooeval
WorkingDirectory=/opt/kooeval
ExecStart=/opt/kooeval/quickeval
Restart=on-failure
RestartSec=5
TimeoutStopSec=20
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/kooeval/uploads /opt/kooeval/logs

[Install]
WantedBy=multi-user.target
```

启用并确认服务状态：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now kooeval
sudo systemctl status kooeval
sudo journalctl -u kooeval -f
```

应用日志默认还会写入 `/opt/kooeval/logs/quickeval.log`。应通过 logrotate 或日志平台对其轮转与保留。

## 8. Nginx 与 HTTPS

以下示例将域名替换为实际域名，并将证书路径替换为证书管理工具生成的路径。将配置保存为 `/etc/nginx/conf.d/kooeval.conf`：

```nginx
server {
    listen 80;
    server_name kooeval.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name kooeval.example.com;

    ssl_certificate     /etc/nginx/certs/kooeval.fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/kooeval.privkey.pem;
    client_max_body_size 10m;

    root /opt/kooeval/web;
    index index.html;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

不要配置 `location /uploads/`。应用中的附件内容通过 `/api/v1/attachments/{id}/content` 受保护地返回。

检查并重载 Nginx：

```bash
sudo nginx -t
sudo systemctl reload nginx
curl -fsS https://kooeval.example.com/health/live
curl -fsS https://kooeval.example.com/health/ready
```

## 9. 启动验收与日常检查

部署后至少确认：

1. `/health/live` 返回存活，`/health/ready` 在 MySQL 与 Redis 连通后返回就绪。
2. 使用超级管理员账号能登录、能创建用户；新用户首次登录会进入改密流程。
3. Vue 路由刷新任意业务页面仍能正常返回应用，而 API 不会被 `index.html` 误响应。
4. 评测截图可以上传、在登录态下查看，且直接访问 `/uploads/` 不可用。
5. Nginx、应用与数据库时钟一致，日志中可按 `request_id` 定位失败请求。

常用命令：

```bash
sudo systemctl status kooeval
sudo systemctl restart kooeval
sudo journalctl -u kooeval --since '30 minutes ago'
curl -fsS http://127.0.0.1:8080/health/ready
```

## 10. 升级、回滚与备份

### 升级顺序

1. 记录版本号并备份 MySQL 与 `/opt/kooeval/uploads`，两者使用同一备份批次时间。
2. 在构建机完成 `make check` 和构建，传输新的发布包。
3. 停止服务，替换二进制、`web/` 与 `migrations/`；保留 `config/`、`uploads/`、`logs/`。
4. 执行 `kooeval-migrate -direction up`。
5. 启动服务，执行健康检查和登录冒烟验证。

### 回滚原则

- 应用回滚只回退二进制与前端静态资源，不自动执行破坏性的 Down Migration。
- Migration 失败时停止发布并恢复旧应用版本；先分析数据库状态和迁移版本再决定后续操作。
- 定期恢复演练 MySQL 与上传目录，并核对附件元数据与文件是否一致。

## 11. 本地开发补充

本地可使用仓库中的开发 Compose 启动 MySQL 和 Redis：

```bash
make infra-up
make migrate-up
make api-run
make web-dev
```

开发前端默认访问 `http://127.0.0.1:5173`，Vite 将 `/api` 和 `/health` 代理到 `VITE_API_PROXY_TARGET`（默认 `http://127.0.0.1:8080`）。
