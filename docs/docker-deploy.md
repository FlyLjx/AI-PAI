# Docker 部署

部署由四个服务组成：`ai-pai`（客户前台及同端口入口）、`admin`（内部管理应用）、`api`（Go）和 `postgres`。两个 Next.js 应用分别部署，`ai-pai` 将 `/sys-admins/*` 转发到内部 `admin:3002`；PostgreSQL 继续保留原有主机端口映射，便于兼容现有运维方式。

## 升级现有实例

先备份数据库：

```bash
docker exec ai-pai-postgres pg_dump -U ai_pai ai_pai > ai-pai-backup.sql
```

然后在项目目录执行：

```bash
docker compose up -d --build
```

Compose 继续使用 `postgres_data` 卷和 `ai_pai` 默认数据库名，现有数据无需导入到新结构。

## 配置

常用 `.env`：

```env
APP_PUBLIC_PORT=6985
APP_PUBLIC_ORIGIN=https://api.example.com
AUTH_ACTION_URLS_IN_RESPONSE=false
ADMIN_COOKIE_SECURE=true
DB_DRIVER=postgres
DB_NAME=ai_pai
DB_USER=ai_pai
DB_PASSWORD=replace-with-a-strong-password
DB_SSLMODE=disable
TZ=Asia/Shanghai
```

连接外部数据库时设置 `DB_HOST` 和 `DB_PORT`。若要移除内置 PostgreSQL 服务，还需通过 Compose override 同步调整 `api.depends_on`。

## 常用命令

```bash
docker compose ps
docker compose logs -f ai-pai admin api
docker compose restart ai-pai admin api
docker compose down
```

`docker compose down` 不会删除数据卷。不要附加 `-v`，除非明确要删除数据库数据。

默认客户前台为 `http://127.0.0.1:6985`，管理入口为 `http://127.0.0.1:6985/sys-admins`。管理容器的 `3002` 端口只在 Compose 网络内开放。生产环境应保持 `AUTH_ACTION_URLS_IN_RESPONSE=false` 并启用 HTTPS。仅在无邮件服务的本地开发环境中临时启用认证操作链接响应。
