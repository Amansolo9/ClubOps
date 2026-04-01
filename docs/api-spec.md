# API Spec (Core)

- `POST /login`
- `POST /register`
- `POST /logout`
- `POST /api/auth/change-password`
- `POST /api/auth/admin-reset`
- `POST /api/budgets`
- `POST /api/budgets/:id/change`
- `POST /api/budget_change_requests/:id/review`
- `GET /api/budgets/:id/projection`
- `POST /api/reviews`
- `POST /api/reviews/:id/appeal`
- `POST /api/reviews/:id/moderate`
- `POST /api/credit_rules`
- `POST /api/credits/issue`
- `POST /api/regions/import`
- `GET /api/regions/:id`
- `POST /api/regions/:id`
- `POST /api/mdm/dimensions/import`
- `POST /api/mdm/sales-facts/import`
- `POST /api/members`
- `POST /api/members/import`
- `GET /api/members/export`
- `POST /api/clubs/:id/profile`
- `POST /api/flags`
- `GET /api/flags/evaluate/:key`
- `GET /clubs/recruiting` (public)

All mutating endpoints are captured by audit middleware.
