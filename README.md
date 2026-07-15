# AI-PAI

AI-PAI 是面向开发者的图像 API 中转站。Next.js 提供用户控制台和管理后台，Go 服务继续使用原有数据库结构、上游调度与 OpenAI 兼容接口。

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

- Next.js：`http://127.0.0.1:3000`
- Go API：`http://127.0.0.1:3001`

Next.js 通过 `/api/backend/*` 访问 Go 管理接口，并将公网 `/v1/*` 转发给 Go。

本地开发端口是固定契约：Next.js 使用 `3000`，Go 使用 `3001`。如果 `3001` 已被其他进程占用，开发脚本会明确退出；先停止占用端口的进程后再重新运行，避免代理地址与 Go 实际监听地址不一致。

## 验证

```powershell
npm run lint
npm run build:web
Set-Location go-server
go test ./...
```

## Docker

```bash
docker compose up -d --build
```

默认访问 `http://127.0.0.1:6985`。Compose 保留原 `postgres_data` 数据卷名称，升级时不会创建新的业务数据库卷。

生产环境至少应修改 `DB_PASSWORD`，并备份现有数据库后再升级。

### 全新数据库的首个管理员

初始化 SQL 只包含表结构，不包含账号、密钥或业务数据。全新数据库首次启动后：

1. 打开 `/register` 注册首个账号。
2. 将下面命令中的邮箱替换为刚注册的邮箱后执行：

```bash
docker compose exec postgres psql -U ai_pai -d ai_pai -c "UPDATE users SET role='admin', email_verified_at=COALESCE(email_verified_at, CURRENT_TIMESTAMP) WHERE email='your-admin@example.com';"
```

3. 退出账号并重新登录，即可进入管理后台。若修改了 `DB_USER` 或 `DB_NAME`，同步调整命令中的连接参数。
