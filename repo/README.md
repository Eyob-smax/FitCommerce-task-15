# FitCommerce Operations & Inventory Suite

A production-grade, offline-first operations platform for a fitness club. Manages catalog, group-buy campaigns, orders, inventory, suppliers, purchase orders, KPI reporting, and audit trails.

---

## Prerequisites

- **Docker Engine** >= 24.0
- **Docker Compose plugin** >= 2.20

No other local tools or runtime dependencies are required.

---

## Startup

> The **only** supported way to run this system is `docker compose up`.
> Do not run the frontend or backend directly on the host machine.

```bash
# 1. Clone and enter the repo root
cd TASK-15/repo

# 2. Copy environment configuration (edit secrets if deploying to production)
cp .env.example .env

# 3. Build and start all services (first time)
docker compose up --build

# 4. On subsequent starts (images already built)
docker compose up
```

All services will start automatically in the correct order:
- PostgreSQL and Redis start first and must pass healthchecks.
- Backend runs database migrations and seeds default data, then serves the API.
- Frontend serves the compiled React app.

---

## Service URLs

| Service | URL |
|---|---|
| **Frontend** | http://localhost:5173 |
| **Backend API** | http://localhost:8080 |
| **Health check** | http://localhost:8080/health |
| PostgreSQL | localhost:5432 |
| Redis | localhost:6379 |

---

## Default Accounts

| Email | Password | Role |
|---|---|---|
| admin@fitcommerce.dev | Password123! | Administrator |
| ops@fitcommerce.dev | Password123! | Operations Manager |
| procurement@fitcommerce.dev | Password123! | Procurement Specialist |
| coach@fitcommerce.dev | Password123! | Coach |
| member@fitcommerce.dev | Password123! | Member |

---

## Stopping the System

```bash
docker compose down          # stop containers, keep database volumes
docker compose down -v       # stop containers AND delete all volumes (resets database)
```

---

## Running Tests

```bash
./run_tests.sh
```

This script runs the full test suite (backend unit + API tests, frontend unit tests) inside Docker and reports coverage. The build fails if coverage falls below 90%.

---

## Project Structure

```
repo/
├── frontend/          React 18 + TypeScript + Vite + Material UI
│   ├── src/           Application source
│   ├── public/        Static assets
│   └── unit-tests/    Vitest + React Testing Library tests
├── backend/           Go 1.23 + Gin
│   ├── cmd/
│   │   ├── api/       HTTP server entry point
│   │   └── worker/    Background job worker entry point
│   ├── internal/      Business logic (auth, items, group-buys, orders…)
│   ├── database/
│   │   ├── migrations/ goose SQL migrations
│   │   ├── queries/    sqlc query definitions
│   │   └── seeds/      Seed data SQL
│   └── tests/
│       ├── unit/       Pure unit tests
│       └── api/        Integration API tests (httptest + real DB)
├── docker-compose.yml
├── .env.example
├── README.md
└── run_tests.sh
```

---

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend | React 18, TypeScript, Vite, Material UI, TanStack Query, Zustand, React Router v6 |
| Offline | Dexie (IndexedDB), Service Worker, queued mutations, sync manager |
| Backend | Go 1.23, Gin, pgx, sqlc |
| Migrations | goose |
| Jobs | Redis + Go worker |
| Database | PostgreSQL 16 |
| Exports | Go CSV + PDF generation |
| Tests | Vitest (frontend), Go testing + httptest (backend) |
| Containers | Docker Compose |

---

## Verification Checklist

1. Run `docker compose up --build` and wait for all services to be healthy.
2. Navigate to http://localhost:5173 — the login screen should appear.
3. Log in as `admin@fitcommerce.dev` / `Password123!`.
4. Confirm the **KPI dashboard** loads with 6 metric cards (growth, churn, renewal, engagement, fill rate, productivity).
5. Toggle between daily/weekly/monthly views on the dashboard.
6. Navigate to **Catalog** — confirm seeded items are visible with status chips.
7. Create a new item, publish it, then unpublish it — verify status transitions.
8. Navigate to **Inventory** — confirm stock records with adjust dialog.
9. Navigate to **Suppliers** — create a supplier, verify it appears in the list.
10. Navigate to **Purchase Orders** — create a PO with line items, issue it, record a partial receipt.
11. Navigate to **Group Buys** — confirm the seeded active group buy shows progress bar.
12. Navigate to **Orders** — create an order, add a note, verify timeline shows both events.
13. Test order adjustment, split, and cancellation — verify timeline is append-only.
14. Navigate to **Reports** — generate a CSV and PDF export, verify download works.
15. Log out and log in as `member@fitcommerce.dev` — confirm member sees only published items, own orders, and group buys (no draft/admin views).
16. Log in as `coach@fitcommerce.dev` — confirm coach sees only own report, no supplier/PO access.
17. Check `GET http://localhost:8080/health` returns `{"status":"ok"}`.
18. Check `GET http://localhost:8080/health/ready` pings DB + Redis.
19. Verify the **sync status indicator** shows Online/Offline/Syncing in the sidebar.

---

## Export Files

- Exports are generated server-side in Go (no client-side generation).
- CSV files contain real queried data; PDF files contain summary text.
- Filenames follow the pattern: `{report_type}_{YYYYMMDD_HHMMSS}.{csv|pdf}`
- Export jobs are persisted in `export_jobs` table with status tracking.
- The worker process drains queued export jobs every 5 seconds.

---

## Offline-First Behavior

- The frontend caches items, group buys, orders, and members in IndexedDB (Dexie).
- When offline, cached data is displayed and mutations are queued locally.
- On reconnect, the SyncManager drains the mutation queue (max 5 retries per mutation).
- Conflict detection: 409 from server stores conflict in IndexedDB for review.
- Conflict resolution: server-wins by default; conflicts viewable for manual resolution.
- The SyncStatusIndicator in the sidebar shows current connectivity state.
