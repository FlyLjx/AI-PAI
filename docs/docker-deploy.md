# Docker 部署

部署由三个服务组成：`ai-pai`（Next.js）、`api`（Go）和 `postgres`。业务入口统一经过 Next.js，管理接口与 `/v1/*` 会转发到内部 Go 服务；PostgreSQL 继续保留原有主机端口映射，便于兼容现有运维方式。

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
docker compose logs -f ai-pai api
docker compose restart ai-pai api
docker compose down
```

`docker compose down` 不会删除数据卷。不要附加 `-v`，除非明确要删除数据库数据。
