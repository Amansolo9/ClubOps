# ClubOps Governance & Finance Portal

Go (Fiber) + HTMX + SQLite implementation for multi-site membership governance, moderation, and finance workflows.

## One-click start

```bash
docker compose -f fullstack/docker-compose.yml up --build
```

App URL: `http://localhost:8080`

Required env:
- `APP_ENCRYPTION_KEY` (set in `docker-compose.yml` for local default)
- `APP_DB_PATH` (defaults to `./fullstack.db` for local runs; Docker sets `/data/fullstack.db`)
- `APP_BOOTSTRAP_ADMIN_PASSWORD` (required when seeding the admin account)

Persistence:
- SQLite DB in Docker volume: `sqlite_data`
- Uploaded review images in: `./fullstack/static/uploads`

## Seeded account

- Username: `admin`
- Initial password: value of `APP_BOOTSTRAP_ADMIN_PASSWORD`

The seeded account is configured for forced password change/reset workflow; use:
- `POST /api/auth/change-password` (for current user)
- `POST /api/auth/admin-reset` (admin-only)

Registration:
- Public `/register` creates `member` accounts only.
- Elevated roles (`team_lead`, `organizer`, `admin`) must be assigned by admin workflows.

## Test commands

```bash
./run_tests.sh
```

Equivalent:

```bash
go test ./fullstack/unit_tests/... -v
go test ./fullstack/API_tests/... -v
```

## Local (non-Docker) run

```bash
export APP_ENCRYPTION_KEY=local-dev-encryption-key-please-change
export APP_DB_PATH=./fullstack.db
export APP_BOOTSTRAP_ADMIN_PASSWORD=change-this-bootstrap-admin-pass
go run ./fullstack/cmd
```

## Security notes

- bcrypt cost `12`
- Session timeout `30m` with sliding refresh on active use
- 5 failed login attempts lock account for 15 minutes
- Club-scoped access for non-admin operators with assigned clubs
- Object-level checks on budget/review mutations
- CSRF protection on authenticated mutations via `csrf_token` cookie + request token
- App-layer encryption for member contact fields (AES-GCM)
- `members.custom_fields` is stored encrypted at rest with an `enc:v1:` prefix; plaintext legacy rows are lazily migrated on read/write paths
- Audit logs are append-only for mutation endpoints and have 2-year retention cleanup
- Audit payload capture is path-allowlisted; sensitive fields (`email`, `phone`, `custom_fields`, `comment`, auth tokens/passwords, identifiers) are redacted by default
- Frontend runtime assets are served locally from `/static/vendor` for offline use

## Sensitive field policy

- Member PII (`email_encrypted`, `phone_encrypted`) and `custom_fields` must only be persisted in encrypted form
- New schema additions carrying identifiers or contact data should follow the same app-layer encryption approach and audit redaction policy

## API error schema

- API endpoints return a normalized error shape: `{ "error": "...", "error_code": "...", "message": "..." }`
- `error_code` values include: `validation_error`, `conflict`, `forbidden`, `not_found`, `bad_request`
- Detailed internal errors are logged server-side; client responses are sanitized for safety and consistency

## Additional workflows

- Reviews must reference a fulfilled order/service record (`fulfilled_order_id`)
- Region hierarchy management is available at `/regions` for admin/organizer users
- Dimension and sales fact import workflows are available at `/mdm`
- Admin user provisioning/role assignment is available at `/users`
- Club avatars can be uploaded locally and are stored under `./fullstack/static/uploads/avatars`
- Budget spend updates can be recorded from `/budgets` to drive execution and alerting
