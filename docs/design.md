# ClubOps Design

- Backend: Go + Fiber, layered handlers/services/store.
- DB: SQLite with startup auto-migrations.
- Auth: bcrypt cost 12, minimum 12-char passwords, 30-minute DB-backed session cookies with sliding refresh, lockout after 5 failed attempts for 15 minutes, password rotation support.
- RBAC: admin, organizer, team_lead, member; team leads are club-scoped in middleware.
- Finance: monthly/quarterly budgets with account/campus/project dimensions, 85% threshold worker, >10% change requires approval request reviewed by admin.
- MDM: versioned region hierarchies with import plus edit/update workflow, product/customer/channel/time dimension versions with per-dimension fixed-length alphanumeric code validation (product/customer/channel/time=5, region=4), and sales fact imports up to 5,000 rows.
- Credit Engine: versioned rules with effective date windows; issued credit rows are immutable per (member, rule_version).
- Moderation: 1-5 star reviews tied to fulfilled orders, controlled tags, local image storage with 5-file max and 2MB/file cap (JPG/PNG), appeals within 7 days, hide/reinstate workflow.
- Membership: member directory with encrypted contact fields, join date/position/active status, custom fields JSON, grouping/search/sort, CSV import/export.
- Club profile: name, tags, local avatar upload, recruitment toggle, description.
- Feature flags: role/club/percentage rollout semantics with evaluation endpoint and admin update path.
- Audit: append-only insert logs for POST/PUT/DELETE with before/after snapshots, CSRF protection for authenticated mutations, and 2-year retention cleanup worker.
