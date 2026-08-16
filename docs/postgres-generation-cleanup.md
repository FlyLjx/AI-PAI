# Generation Result Cleanup

The API removes inline image data from `generation_tasks.result_json` in small
batches during startup. The cleanup progress is available to an authenticated
administrator at:

```text
GET /api/admin/image-cleanup/status
```

Wait until the response reports `state: "completed"`. Then run
`deploy/repack-generation-tasks.sh` from a host with `psql` and `pg_repack`
installed. The script checks for active generation tasks, reports the remaining
Base64 rows, runs `pg_repack --no-order`, and finishes with `VACUUM (ANALYZE)`.

Example:

```bash
PGHOST=127.0.0.1 \
PGPORT=5432 \
PGUSER=ai_pai \
PGDATABASE=ai_pai \
PGPASSWORD="$DB_PASSWORD" \
./deploy/repack-generation-tasks.sh
```

`pg_repack` needs an available connection to PostgreSQL and extra temporary
disk space. It avoids the long exclusive table lock used by `VACUUM FULL`,
while still requiring short metadata locks at the beginning and end.
