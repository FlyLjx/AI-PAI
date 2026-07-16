# AI-PAI

AI-PAI 是面向开发者的图像 API 中转站。客户前台与管理后台是两个独立的 Next.js 应用，Go 服务继续使用原有数据库结构、上游调度与 OpenAI 兼容接口。

## 应用边界

- `apps/web`：公开首页、注册登录、客户控制台、API Key、用量、计费与文档
- `apps/admin`：独立管理员登录、用户、上游、模型、套餐、调用与系统管理
- `go-server`：Go API、数据库访问、任务队列、计费和 OpenAI 兼容接口

两个 Next.js 应用拥有独立构建产物、进程、容器和认证机制，对外由客户站在同一端口按路径分流。管理端代码不读取客户 localStorage，管理员令牌只保存在 `/sys-admins` 路径限定的 HttpOnly Cookie 中。

## 功能

- OpenAI 兼容的 `/v1/models`、`/v1/images/generations`、`/v1/images/edits`
- API Key、并发限制、调用日志和用量统计
- 上游接口、模型映射、成本价和销售价管理
- 有效订阅优先使用订阅额度，其他账户按余额扣费
- 余额充值、订阅套餐、用户和系统日志管理
- PostgreSQL 与 MySQL 兼容，沿用现有迁移和业务数据

项目不提供网页生图、作品广场、提示词库、邀请或抽奖等 C 端功能。

## 本地开发

环境要求：Node.js 24、Go 1.26.4 和 Docker Desktop。

```powershell
npm install
.\dev.ps1
```

`dev.ps1` 使用混合开发模式：只在 Docker 中运行 PostgreSQL，客户前台、管理后台和 Go API 均在本机运行并热更新。脚本会停止占用开发端口的 `ai-pai`、`admin`、`api` 容器，但不会删除容器、数据库或 `postgres_data` 数据卷。默认将 PostgreSQL 映射到宿主机 `55432`；需要其他端口时可执行 `.\dev.ps1 -PostgresPort 55433`。

也可以使用 `npm run dev:local` 启动相同模式。按 `Ctrl+C` 只停止本机开发进程；后台运行时可执行 `.\dev-stop.ps1` 或 `npm run dev:stop`。PostgreSQL 容器会继续运行，下一次启动无需重新构建镜像。

默认地址：

- 客户前台：`http://127.0.0.1:3000`
- Go API：`http://127.0.0.1:3001`
- 管理后台：`http://127.0.0.1:3000/sys-admins`

客户前台代理客户接口、`/api/tasks/*`、公网 `/v1/*`，并将 `/sys-admins/*` 转发到独立管理应用。管理后台使用独立的后台接口白名单访问 Go，不提供 `/v1` 或客户注册路由。

本地开发对外端口是 `3000`，Go 使用 `3001`，管理应用内部使用 `3002`。Next.js 文件保存后会热更新，Go 文件保存后由 Air 自动编译并重启；如果端口已被其他进程占用，开发脚本会明确退出。

## 验证

```powershell
npm run lint
npm run build:web
npm run build:admin
Set-Location go-server
go test ./...
```

## Docker

```bash
docker compose up -d --build
```

默认地址：

- 客户前台：`http://127.0.0.1:6985`
- 管理后台：`http://127.0.0.1:6985/sys-admins`

Compose 保留原 `postgres_data` 数据卷名称，升级时不会创建新的业务数据库卷。

### 后台手动更新

成功的 `main` 分支 Build 会发布 `build-<Actions run_number>` 版本标签。管理后台“系统设置”会显示当前版本和 GitHub Actions 最新成功版本；检测到新版本后，由管理员确认并手动触发更新，不会定时自动部署。

宿主机更新 worker 收到请求后会拉取同一个 `build-N` 的 Web、Admin 和 API 镜像，先创建并校验 PostgreSQL 逻辑备份，再替换应用容器。健康检查失败时自动恢复更新前的应用镜像，数据库备份保留在 `/opt/ai-pai/backups/manual-updates`，默认保留最近 3 份。

```bash
install -D -m 0750 deploy/ai-pai-update-worker.sh /opt/ai-pai/bin/ai-pai-update-worker
install -D -m 0644 deploy/systemd/ai-pai-update.service /etc/systemd/system/ai-pai-update.service
install -D -m 0644 deploy/systemd/ai-pai-update.path /etc/systemd/system/ai-pai-update.path
mkdir -p /opt/ai-pai/update
chmod 700 /opt/ai-pai/update
systemctl daemon-reload
systemctl enable --now ai-pai-update.path
```

生产环境至少应修改 `DB_PASSWORD`，并备份现有数据库后再升级。将 `APP_PUBLIC_ORIGIN` 设置为唯一的外部来源地址，并保持 `AUTH_ACTION_URLS_IN_RESPONSE=false`；启用 HTTPS 后将 `ADMIN_COOKIE_SECURE` 设置为 `true`。

### 全新数据库的首个管理员

初始化 SQL 只包含表结构，不包含账号、密钥或业务数据。全新数据库首次启动后：

1. 打开 `/register` 注册首个账号。
2. 将下面命令中的邮箱替换为刚注册的邮箱后执行：

```bash
docker compose exec postgres psql -U ai_pai -d ai_pai -c "UPDATE users SET role='admin', email_verified_at=COALESCE(email_verified_at, CURRENT_TIMESTAMP) WHERE email='your-admin@example.com';"
```

3. 打开管理入口 `http://127.0.0.1:6985/sys-admins`，使用该邮箱和密码登录。若修改了 `DB_USER` 或 `DB_NAME`，同步调整命令中的连接参数。
