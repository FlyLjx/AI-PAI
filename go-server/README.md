# AI-PAI Go Backend

This directory contains the existing Go API service. The Next.js application at
the repository root owns the web UI; Go retains database compatibility, account
and billing logic, provider dispatch, task processing, and OpenAI-compatible
image endpoints.

## Active Scope

- Existing PostgreSQL/MySQL schema compatibility and non-destructive startup.
- User/admin authentication, API Key management, usage logs, and settings.
- Provider and image-model configuration.
- Subscription quota billing and account-balance billing.
- Recharge orders, subscription purchases, and idempotent payment settlement.
- Queue processing, result image delivery, and system logs.
- Public API routes: `/v1/models`, `/v1/images/generations`, and
  `/v1/images/edits`.

Browser image generation, galleries, favorites, prompt tools, invitations,
lottery, OAuth, mail broadcast, and C-end WebSocket routes are not registered.
Their historical tables remain untouched for compatibility with existing data.

## Run Locally

From the repository root, use:

```powershell
npm run dev:go
```

Or run Go directly:

```powershell
Set-Location go-server
go mod download
go run ./cmd/ai-pai
```

The default port is `3001`. Database connection values come from the existing
`DB_*` environment variables.

## Verify

```powershell
go test ./...
go vet ./...
go build ./cmd/ai-pai
```

Keep API response shapes and database behavior compatible with deployed data.
Schema changes require an explicit migration and a verified backup path.
