# FitCommerce Operations & Inventory Suite — System Design

## 1. System Overview

FitCommerce is a production-grade, offline-first operations and inventory platform for a fitness club. It manages a catalog of gear and supplement items, member-driven group-buy campaigns, supplier and purchase-order workflows, KPI reporting, and a full audit timeline. The system is classified as:

- **Type:** full_stack / web / dockerized / offline_first
- **Startup:** `docker compose up` from `repo/`
- **Target users:** club staff (Admin, Operations Manager, Procurement Specialist, Coach) and club members

---

## 2. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  Browser (React 18 + TypeScript + Vite + Material UI)            │
│  ┌─────────────────────────────────────────────────────────┐     │
│  │  TanStack Query cache  │  Zustand store  │  Dexie (IDB) │     │
│  │  Offline mutation queue │ Service Worker  │ Sync Manager │     │
│  └─────────────────────────────────────────────────────────┘     │
└──────────────────────────┬───────────────────────────────────────┘
                           │ HTTP/JSON (REST)
┌──────────────────────────▼───────────────────────────────────────┐
│  Go 1.23 API Server (Gin)  — port 8080                           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐    │
│  │  auth    │ │  items   │ │ group-   │ │  orders /        │    │
│  │  users   │ │  catalog │ │  buys    │ │  suppliers / PO  │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐    │
│  │ reports  │ │ exports  │ │  audit   │ │  sync endpoints  │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘    │
│                                                                    │
│  Go Worker Process (Redis-backed job queue)                        │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │ group-buy cutoff evaluator │ export jobs │ notification    │   │
│  └────────────────────────────────────────────────────────────┘   │
└──────────────────────────┬───────────────────────────────────────┘
                           │
           ┌───────────────┴──────────────┐
           │                              │
    ┌──────▼──────┐               ┌───────▼──────┐
    │ PostgreSQL  │               │   Redis 7    │
    │ 16          │               │   (jobs +    │
    │ port 5432   │               │    cache)    │
    └─────────────┘               │  port 6379   │
                                  └──────────────┘
```

---

## 3. Domain Model

### Core entities

| Entity | Description |
|---|---|
| `users` | All accounts. Role stored as enum. |
| `locations` | Club locations. First-class filter dimension. |
| `items` | Catalog entries with specs, condition, billing model, deposit, availability windows. |
| `item_availability_windows` | Time-bounded fulfillment windows per item. |
| `inventory_stock` | Per-location stock levels with state tracking (on_hand / reserved / allocated / in_rental / returned / damaged). |
| `suppliers` | Vendor master data. |
| `purchase_orders` | PO header with states: draft → issued → partially_received → received → cancelled / closed. |
| `po_line_items` | Line-level items, quantities, and cost. |
| `goods_receipts` | Receiving records against a PO with discrepancy notes. |
| `group_buys` | Campaigns with min qty, cutoff time, and states: draft → published → active → succeeded / failed / cancelled → fulfilled. |
| `group_buy_participants` | Member commitments to a campaign. |
| `orders` | Individual fulfillment orders generated from group-buys or direct purchases. |
| `order_timeline_events` | Immutable event log per order (adjustments, splits, cancellations, notes). |
| `payment_ledger` | Internal payment state: pending / authorized / captured / refunded / partially_refunded / voided. |
| `export_jobs` | Record of export requests with actor, timestamp, filters, and output file reference. |
| `audit_log` | System-wide immutable event trail with actor, action, entity, before/after snapshot. |
| `kpi_snapshots` | Materialized daily KPI records for dashboard performance. |

### RBAC roles

| Role | Key permissions |
|---|---|
| `administrator` | Full system access, user management, retention config, audit log. |
| `operations_manager` | Catalog CRUD, inventory, reports, exports, group-buy oversight. |
| `procurement_specialist` | Suppliers, purchase orders, receiving, inventory adjustments. |
| `coach` | View own class schedule, fill rate, attendance KPIs, limited reporting. |
| `member` | Browse items, create/join group-buys, view own orders. |

---

## 4. Offline-First Design

### Principles
1. The app must remain usable for core read and create workflows during temporary network loss.
2. All mutations that fail network delivery are queued locally and retried on reconnect.
3. Server is always authoritative. Conflict detection uses server-issued version fields.

### Layers

| Layer | Technology | Responsibility |
|---|---|---|
| Static asset cache | Service Worker (Workbox) | Cache shell, fonts, icons at install. Serve stale while revalidate. |
| Read cache | TanStack Query + Dexie (IndexedDB) | Persist query results to IDB. Hydrate on load. TTL-controlled staleness. |
| Write queue | Zustand + Dexie | Queue mutations when offline. Persist across page reloads. |
| Sync manager | Custom hook + Beacon API | Drain queue on reconnect. Expose sync status in UI. |
| Conflict resolution | Server version field | Last-write-wins for low-risk entities. Protected conflict-review flow for inventory, orders, POs. |

### Offline-capable workflows
- Browse catalog and item details
- View dashboard KPIs (cached)
- Draft new items (saved locally, synced on reconnect)
- Queue group-buy join action
- View own order history (cached)

### Server-authoritative operations (require connectivity)
- Authentication and token refresh
- Inventory stock adjustments
- Group-buy cutoff evaluation
- Purchase order issuance
- Payment ledger mutations

---

## 5. Module Boundaries (Backend)

```
backend/
├── cmd/
│   ├── api/       — main.go: HTTP server, router wiring, graceful shutdown
│   └── worker/    — main.go: Redis consumer, job dispatchers
├── internal/
│   ├── auth/          — JWT issuing, refresh rotation, token validation
│   ├── config/        — env loading, validated config struct
│   ├── database/      — pgx pool, sqlc generated code
│   ├── http/
│   │   ├── handlers/  — one package per domain (items, groupbuys, orders…)
│   │   └── router/    — Gin router wiring, middleware registration
│   ├── middleware/     — auth guard, RBAC, request ID, logger, rate limiter
│   ├── modules/
│   │   ├── items/         — catalog, availability windows, batch edits
│   │   ├── groupbuys/     — campaign lifecycle, participant management, cutoff
│   │   ├── orders/        — order CRUD, timeline events, splits, cancellations
│   │   ├── suppliers/     — vendor master, PO management, receiving
│   │   ├── reports/       — KPI queries, dashboard aggregation, filter logic
│   │   └── audit/         — immutable event writer, audit log reader
│   ├── services/
│   │   ├── inventory/  — stock state machine, reservation, release
│   │   ├── payment/    — internal ledger, deposit handling
│   │   └── notification/ — in-app notification writer
│   ├── sync/          — offline sync endpoint handlers, version checking
│   └── exports/       — CSV writer, PDF generator, export job manager
├── database/
│   ├── migrations/    — goose SQL migration files
│   ├── queries/       — sqlc .sql query files
│   └── seeds/         — seed SQL for roles, locations, and default users
└── tests/
    ├── unit/          — pure unit tests (no DB, no HTTP)
    └── api/           — integration tests using httptest + real DB
```

---

## 6. Synchronization Protocol

### Endpoint contract
- `GET /api/v1/sync/changes?since=<unix_ts>&entities=<csv>` — returns changed records since timestamp
- `POST /api/v1/sync/push` — accepts batched offline mutations with client-generated idempotency keys
- `POST /api/v1/sync/resolve` — submits conflict resolution decisions for protected entities

### Version tracking
Every mutable entity has `updated_at` (timestamp) and `version` (integer, incremented on each write). The client stores the last-known `updated_at` per entity type and sends it as the `since` parameter on reconnect.

### Idempotency
All offline mutations carry a UUID `idempotency_key`. The server deduplicates by key within a 24-hour window, stored in Redis.

---

## 7. Reporting and Exports

### KPI definitions
| Metric | Formula |
|---|---|
| Member growth | Net new active members in period |
| Churn | Memberships ended or not renewed / total active at period start |
| Renewal rate | Renewed memberships / eligible-for-renewal memberships |
| Engagement | Attendance + order + group-buy participation events / active members |
| Class fill rate | Booked seats / class capacity |
| Coach productivity | Completed sessions + positive attendance outcomes attributed to coach |

### Dashboard filters
- Time granularity: daily / weekly / monthly
- Date range (start, end)
- Location
- Coach
- Item category

### Export pipeline
1. Client requests export via `POST /api/v1/exports` with report type + filters.
2. Server enqueues an export job in Redis.
3. Worker generates CSV or PDF in Go.
4. Job record updated with S3/local file path.
5. Client polls `GET /api/v1/exports/:id` and receives a signed download URL.
6. Filename format: `<report-slug>_<scope>_<YYYYMMDD_HHmmss>.<csv|pdf>`

### Access control
Each report type maps to a minimum required role. Coach-scoped reports are automatically filtered to the requesting coach's own data.

---

## 8. Dockerization

All services run in Docker. No host-level dependencies required beyond Docker Engine.

| Service | Image | Port |
|---|---|---|
| `frontend` | Custom (Node build → nginx static) | 5173 → 80 |
| `backend` | Custom (Go multi-stage build) | 8080 |
| `db` | `postgres:16-alpine` | 5432 |
| `redis` | `redis:7-alpine` | 6379 |

### Startup sequence
1. `db` and `redis` start and pass healthchecks.
2. `backend` starts, runs goose migrations, runs seed (idempotent), then starts serving.
3. `frontend` starts and serves the compiled React app.

### Volumes
- `postgres_data` — persistent DB data
- `redis_data` — persistent Redis AOF

---

## 9. Test Strategy

### Coverage target: > 90% meaningful coverage

| Layer | Framework | Scope |
|---|---|---|
| Frontend unit | Vitest + React Testing Library | Components, hooks, store, sync queue, offline logic |
| Backend unit | Go `testing` | Business logic, state machines, formula calculations |
| Backend API | Go `testing` + `httptest` + real PG (test DB) | All HTTP endpoints, auth, RBAC, edge cases |

### Test execution
All tests run inside Docker via `run_tests.sh`. The script:
1. Spins up a test PostgreSQL instance.
2. Runs goose migrations against it.
3. Runs backend unit + API tests with `-race` and `-coverprofile`.
4. Runs frontend Vitest with coverage.
5. Fails if combined coverage falls below 90%.

### Key test areas
- Auth token lifecycle (issue, refresh, revoke)
- RBAC enforcement per endpoint
- Group-buy state machine transitions
- Inventory stock reservation and oversell prevention
- Offline sync push deduplication
- Export job lifecycle
- Audit log immutability
