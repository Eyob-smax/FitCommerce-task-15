# FitCommerce API Specification

**Base URL:** `http://localhost:8080/api/v1`
**Auth:** Bearer JWT in `Authorization` header
**Content-Type:** `application/json`

---

## Authentication

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| POST | `/auth/login` | None | — | Issue access + refresh tokens |
| POST | `/auth/refresh` | Refresh token | — | Rotate refresh token, issue new access token |
| POST | `/auth/logout` | Bearer | Any | Revoke refresh token |

### POST /auth/login
**Request:** `{ "email": string, "password": string }`
**Response:** `{ "access_token": string, "refresh_token": string, "expires_in": 900, "user": UserDTO }`

---

## Users

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/users` | Bearer | admin | List all users |
| POST | `/users` | Bearer | admin | Create user |
| GET | `/users/:id` | Bearer | admin, self | Get user |
| PATCH | `/users/:id` | Bearer | admin, self | Update user |
| DELETE | `/users/:id` | Bearer | admin | Soft-delete user |
| GET | `/users/me` | Bearer | Any | Current user profile |

---

## Locations

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/locations` | Bearer | Any | List locations |
| POST | `/locations` | Bearer | admin | Create location |
| PATCH | `/locations/:id` | Bearer | admin | Update location |

---

## Items (Catalog)

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/items` | Bearer | Any | List items (published: all roles; draft: staff only) |
| POST | `/items` | Bearer | admin, ops_manager | Create item |
| GET | `/items/:id` | Bearer | Any | Get item detail |
| PATCH | `/items/:id` | Bearer | admin, ops_manager | Update item |
| DELETE | `/items/:id` | Bearer | admin, ops_manager | Soft-delete item |
| POST | `/items/:id/publish` | Bearer | admin, ops_manager | Publish item |
| POST | `/items/:id/unpublish` | Bearer | admin, ops_manager | Unpublish item |
| POST | `/items/batch` | Bearer | admin, ops_manager | Batch update price or availability |
| GET | `/items/:id/availability-windows` | Bearer | Any | Get availability windows |
| POST | `/items/:id/availability-windows` | Bearer | admin, ops_manager | Add availability window |
| DELETE | `/items/:id/availability-windows/:wid` | Bearer | admin, ops_manager | Remove availability window |

### Item DTO
```json
{
  "id": "uuid",
  "name": "string",
  "description": "string",
  "category": "string",
  "brand": "string",
  "condition": "new | open-box | used",
  "billing_model": "one-time | monthly-rental",
  "deposit_amount": "50.00",
  "price": "number",
  "quantity": "integer",
  "status": "draft | published | unpublished",
  "location_id": "uuid",
  "availability_windows": [],
  "created_at": "ISO8601",
  "updated_at": "ISO8601",
  "version": "integer"
}
```

---

## Group-Buys

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/group-buys` | Bearer | Any | List campaigns |
| POST | `/group-buys` | Bearer | member, ops_manager, admin | Create group-buy |
| GET | `/group-buys/:id` | Bearer | Any | Get campaign detail with progress |
| PATCH | `/group-buys/:id` | Bearer | admin, ops_manager | Update campaign |
| POST | `/group-buys/:id/publish` | Bearer | admin, ops_manager | Publish campaign |
| POST | `/group-buys/:id/cancel` | Bearer | admin, ops_manager | Cancel campaign |
| POST | `/group-buys/:id/join` | Bearer | member | Join campaign |
| DELETE | `/group-buys/:id/leave` | Bearer | member | Leave campaign (before cutoff) |
| GET | `/group-buys/:id/participants` | Bearer | admin, ops_manager | List participants |

### Campaign states
`draft → published → active → succeeded | failed | cancelled → fulfilled`

---

## Orders

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/orders` | Bearer | admin, ops_manager, member(own) | List orders |
| GET | `/orders/:id` | Bearer | admin, ops_manager, member(own) | Get order detail |
| PATCH | `/orders/:id` | Bearer | admin, ops_manager | Adjust order |
| POST | `/orders/:id/cancel` | Bearer | admin, ops_manager | Cancel order |
| POST | `/orders/:id/split` | Bearer | admin, ops_manager | Split order |
| GET | `/orders/:id/timeline` | Bearer | admin, ops_manager, member(own) | Order timeline events |
| POST | `/orders/:id/notes` | Bearer | admin, ops_manager | Add timeline note |

---

## Suppliers

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/suppliers` | Bearer | admin, ops_manager, procurement | List suppliers |
| POST | `/suppliers` | Bearer | admin, procurement | Create supplier |
| GET | `/suppliers/:id` | Bearer | admin, ops_manager, procurement | Get supplier |
| PATCH | `/suppliers/:id` | Bearer | admin, procurement | Update supplier |

---

## Purchase Orders

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/purchase-orders` | Bearer | admin, ops_manager, procurement | List POs |
| POST | `/purchase-orders` | Bearer | admin, procurement | Create PO |
| GET | `/purchase-orders/:id` | Bearer | admin, ops_manager, procurement | Get PO |
| PATCH | `/purchase-orders/:id` | Bearer | admin, procurement | Update PO |
| POST | `/purchase-orders/:id/issue` | Bearer | admin, procurement | Issue PO to supplier |
| POST | `/purchase-orders/:id/cancel` | Bearer | admin, procurement | Cancel PO |
| POST | `/purchase-orders/:id/receive` | Bearer | admin, procurement | Record goods receipt |

### PO states
`draft → issued → partially_received → received → cancelled | closed`

---

## Reports & KPIs

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/reports/dashboard` | Bearer | admin, ops_manager, coach(scoped) | KPI dashboard data |
| GET | `/reports/member-growth` | Bearer | admin, ops_manager | Member growth report |
| GET | `/reports/churn` | Bearer | admin, ops_manager | Churn report |
| GET | `/reports/inventory` | Bearer | admin, ops_manager, procurement | Inventory report |
| GET | `/reports/group-buys` | Bearer | admin, ops_manager | Group-buy performance |
| GET | `/reports/coach/:id` | Bearer | admin, ops_manager, coach(self) | Coach productivity |

### Query parameters (common)
`?granularity=daily|weekly|monthly&from=YYYY-MM-DD&to=YYYY-MM-DD&location_id=uuid&category=string&coach_id=uuid`

---

## Exports

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| POST | `/exports` | Bearer | admin, ops_manager, procurement | Enqueue export job |
| GET | `/exports` | Bearer | admin, ops_manager | List export jobs |
| GET | `/exports/:id` | Bearer | admin, ops_manager | Get job status + download URL |

### POST /exports body
```json
{
  "report_type": "member-growth | churn | inventory | group-buys | coach",
  "format": "csv | pdf",
  "filters": { "from": "YYYY-MM-DD", "to": "YYYY-MM-DD", "location_id": "uuid" }
}
```

---

## Sync

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/sync/changes` | Bearer | Any | Get changed records since timestamp |
| POST | `/sync/push` | Bearer | Any | Push queued offline mutations |
| POST | `/sync/resolve` | Bearer | Any | Submit conflict resolutions |

### GET /sync/changes query params
`?since=<unix_timestamp>&entities=items,orders,group-buys`

---

## Audit Log

| Method | Path | Auth | Roles | Description |
|---|---|---|---|---|
| GET | `/audit` | Bearer | admin | Paginated audit log |
| GET | `/audit/:entity/:id` | Bearer | admin, ops_manager | Audit trail for entity |

---

## Common response shapes

### Success
```json
{ "data": <payload>, "meta": { "page": 1, "per_page": 20, "total": 100 } }
```

### Error
```json
{ "error": { "code": "VALIDATION_ERROR", "message": "string", "fields": {} } }
```

### HTTP status codes
| Code | Meaning |
|---|---|
| 200 | OK |
| 201 | Created |
| 400 | Validation error |
| 401 | Unauthenticated |
| 403 | Forbidden (RBAC) |
| 404 | Not found |
| 409 | Conflict (version mismatch, duplicate) |
| 422 | Unprocessable entity |
| 500 | Internal server error |
