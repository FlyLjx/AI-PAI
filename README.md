# AI-PAI

AI-PAI 是面向开发者的图像 API 中转站。客户前台与管理后台是两个独立的 Next.js 应用，Go 服务继续使用原有数据库结构、上游调度与 OpenAI 兼容接口。

## 应用边界

- `apps/web`：公开首页、注册登录、客户控制台、API Key、用量、计费与文档
- `apps/admin`：独立管理员登录、用户、上游、模型、套餐、调用与系统管理
- `go-server`：Go API、数据库访问、任务队列、计费和 OpenAI 兼容接口

两个 Next.js 应用拥有独立构建产物、端口、容器和认证会话。管理后台使用 HttpOnly Cookie，不与客户前台共享 localStorage 或管理员令牌。

## 功能

- OpenAI 兼容的 `/v1/models`、`/v1/images/generations`、`/v1/images/edits`
- API Key、并发限制、调用日志和用量统计
- 上游接口、模型映射、成本价和销售价管理
- 有效订阅优先使用订阅额度，其他账户按余额扣费
- 余额充值、订阅套餐、用户和系统日志管理
- PostgreSQL 与 MySQL 兼容，沿用现有迁移和业务数据

项目不提供网页生图、作品广场、提示词库、邀请或抽奖等 C 端功能。

## 本地开发

环境要求：Node.js 24、Go 1.26.4、可用的 PostgreSQL 或 MySQL。

```powershell
npm install
npm run dev
```

默认地址：

- 客户前台：`http://127.0.0.1:3000`
- Go API：`http://127.0.0.1:3001`
- 管理后台：`http://127.0.0.1:3002`

客户前台只代理客户接口、`/api/tasks/*` 和公网 `/v1/*`。管理后台使用独立的后台接口白名单访问 Go，不提供 `/v1` 或客户注册路由。

本地开发端口是固定契约：客户前台使用 `3000`，Go 使用 `3001`，管理后台使用 `3002`。如果端口已被其他进程占用，开发脚本会明确退出。

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
- 管理后台：`http://127.0.0.1:6986`

Compose 保留原 `postgres_data` 数据卷名称，升级时不会创建新的业务数据库卷。

生产环境至少应修改 `DB_PASSWORD`，并备份现有数据库后再升级。为客户站设置 `APP_PUBLIC_ORIGIN` 并保持 `AUTH_ACTION_URLS_IN_RESPONSE=false`；管理后台应使用独立域名，设置 `ADMIN_PUBLIC_ORIGIN`，并在启用 HTTPS 后将 `ADMIN_COOKIE_SECURE` 设置为 `true`。

### 全新数据库的首个管理员

初始化 SQL 只包含表结构，不包含账号、密钥或业务数据。全新数据库首次启动后：

1. 打开 `/register` 注册首个账号。
2. 将下面命令中的邮箱替换为刚注册的邮箱后执行：

```bash
docker compose exec postgres psql -U ai_pai -d ai_pai -c "UPDATE users SET role='admin', email_verified_at=COALESCE(email_verified_at, CURRENT_TIMESTAMP) WHERE email='your-admin@example.com';"
```

3. 打开独立管理后台 `http://127.0.0.1:6986/login`，使用该邮箱和密码登录。若修改了 `DB_USER` 或 `DB_NAME`，同步调整命令中的连接参数。
