# questions.md

## 1. Offline-first scope and synchronization model
**The Gap:** The prompt requires offline-first management, but it does not define which actions must work offline, how long data can remain unsynced, or how conflicts are resolved when connectivity returns.

**The Interpretation:** Core operator and member workflows should remain usable during temporary network interruptions inside the local environment. Read-heavy screens should cache aggressively, and create/update actions should queue locally and sync when the app reconnects.

**Proposed Implementation:** Build the React frontend as a PWA with IndexedDB-backed local persistence, a sync queue for mutations, optimistic UI where safe, and server-issued version fields for conflict detection. Use a last-write-wins strategy for low-risk entities such as report filters and draft edits, and a protected conflict-review flow for high-risk records such as inventory adjustments, purchase orders, and order state transitions.

## 2. KPI metric definitions
**The Gap:** The dashboard lists member growth, churn, renewal rate, engagement, class fill rate, and coach productivity, but the exact formulas and source events are not defined.

**The Interpretation:** Use standard fitness-club and commerce analytics definitions so development can proceed without inventing arbitrary business math later.

**Proposed Implementation:** Store a metrics definition table in code and document it in `docs/design.md` and `docs/api-spec.md`. Define growth as net new active members in a period, churn as memberships ended or not renewed in a period, renewal rate as renewed memberships divided by memberships eligible for renewal, engagement as attendance plus order/group-buy participation signals, class fill rate as booked seats divided by capacity, and coach productivity as completed sessions plus attendance/engagement outcomes attributed to the coach.

## 3. Multi-location data model
**The Gap:** Reports must filter by location, but the prompt does not explicitly define whether the fitness club is single-site or multi-site.

**The Interpretation:** The system should support one or more club locations from the start because location is a first-class reporting filter.

**Proposed Implementation:** Add a `locations` table and foreign-key it into members, classes, inventory stock records, purchase orders, orders, and reports. Scope dashboards and operational lists by location permissions where appropriate.

## 4. Item availability window semantics
**The Gap:** Item listings support available time windows, but the prompt does not specify whether these windows mean pickup windows, rental windows, booking windows, or sales visibility windows.

**The Interpretation:** Availability windows represent the pickup or rental access window during which an item can be fulfilled or used, while publication status controls public visibility.

**Proposed Implementation:** Model item publication separately from fulfillment availability. Store one or more item availability windows with start/end timestamps and validate overlaps against inventory commitments for rentals and reservable gear.

## 5. Inventory reservations and oversell prevention
**The Gap:** The prompt requires inventory quantity, rentals, and group-buys, but it does not say how inventory should be reserved before fulfillment.

**The Interpretation:** The platform should reserve stock in stages to prevent overselling while still allowing drafts and pending campaigns.

**Proposed Implementation:** Introduce stock states for on_hand, reserved, allocated, in_rental, returned, damaged, and available. Group-buy commitments do not consume stock until the campaign succeeds; regular orders and approved fulfillment actions reserve stock immediately; rental returns release or reconcile stock after condition inspection.

## 6. Group-buy success and failure rules
**The Gap:** Members can start or join a group-buy and must see success or failure at cutoff time, but the prompt does not define campaign state transitions, cancellation windows, or what happens when the minimum quantity is not reached.

**The Interpretation:** A group-buy becomes successful only when minimum committed quantity is met by cutoff and payment/authorization conditions are satisfied; otherwise it fails automatically.

**Proposed Implementation:** Use explicit campaign states: draft, published, active, succeeded, failed, cancelled, and fulfilled. Run a scheduled cutoff evaluator in the backend to compute status at cutoff, lock the campaign, generate resulting orders for successful campaigns, and create visible failure outcomes plus notification records for failed campaigns.

## 7. Payment, deposit, and refund behavior
**The Gap:** The prompt mentions refundable deposits and group-buy orders but does not define payment timing, refund timing, or whether failed campaigns should capture funds.

**The Interpretation:** The system should support local payment state management even if external payment processing is absent. Deposits are authorized or recorded when an order is placed, captured when fulfillment conditions are met, and refunded according to item return or campaign outcome.

**Proposed Implementation:** Build an internal payment ledger with statuses pending, authorized, captured, refunded, partially_refunded, and voided. For failed group-buys, do not capture charges. For rentals, record the base transaction separately from the deposit ledger so deposits can be refunded after return inspection.

## 8. Purchase-order receiving workflow
**The Gap:** Procurement Specialist responsibilities include suppliers and purchase orders, but the prompt does not describe how ordered stock becomes available inventory.

**The Interpretation:** Purchase orders must support a full receiving workflow with partial receipts and variance handling.

**Proposed Implementation:** Implement suppliers, purchase orders, purchase order line items, goods receipts, and stock adjustments. Support PO states draft, issued, partially_received, received, cancelled, and closed. Receiving updates inventory by location and records cost, lot/batch notes if provided, and discrepancy audit entries.

## 9. Order adjustment, split, and cancellation authorization
**The Gap:** Orders need a visible operation timeline so staff can explain adjustments, splits, or cancellations, but the prompt does not specify who can perform each action.

**The Interpretation:** Operational changes should be permission-based and fully auditable to avoid ambiguous order history.

**Proposed Implementation:** Define granular permissions for order note creation, quantity adjustment, split shipment/order creation, cancellation, refund handling, and timeline visibility. Persist each change as an immutable timeline event with actor, timestamp, before/after snapshot, and reason note.

## 10. Coach reporting scope
**The Gap:** Coaches have class readiness and limited reporting access, but the prompt does not define the boundary of “limited reporting.”

**The Interpretation:** Coaches should only see operational and performance data tied to their own classes and aggregated KPIs that do not expose finance, supplier, or full inventory administration data.

**Proposed Implementation:** Restrict coach-facing reports to their assigned classes, fill rate, attendance, readiness status, and productivity summaries. Enforce both backend query scoping and frontend route-level controls.

## 11. CSV/PDF export content and naming convention
**The Gap:** The prompt requires downloadable CSV/PDF exports with timestamped filenames, but it does not define timezone, naming pattern, or export scope.

**The Interpretation:** Exports should be deterministic, permission-aware, and reflect the current filter context.

**Proposed Implementation:** Generate exports on the backend using a standardized naming convention such as `<report-slug>_<location-or-scope>_<YYYYMMDD_HHmmss>.csv|pdf` in club-local timezone. Persist export jobs and metadata so users can trace who exported what and when.

## 12. Audit and retention policy
**The Gap:** The system requires strong operational control and visible timelines, but the prompt does not define how long operational events, exports, and audit trails should be retained.

**The Interpretation:** Use a practical retention model suitable for business operations without deleting critical compliance-relevant records too aggressively.

**Proposed Implementation:** Keep immutable audit logs and order/group-buy timelines indefinitely unless club policy overrides them; apply configurable retention to generated export files and transient sync job logs; expose retention settings only to Administrators.

---

## Actual Implementation Decisions (Updated during build)

### Authentication
- JWT access tokens (15min TTL) + refresh token rotation (7 day TTL)
- Refresh tokens stored as SHA-256 hashes in `refresh_tokens` table
- bcrypt cost 12 for password hashing
- Auto-refresh interceptor in API client retries on 401

### Offline Sync
- Frontend: Dexie v4 IndexedDB with 7 tables (items, groupBuys, orders, members, mutations, syncMeta, conflicts)
- Queued mutations with idempotency keys (UUID v4) for deduplication
- Retry with backoff: max 5 retries, then marked rejected
- Conflict detection: 409 response triggers conflict record in IndexedDB
- Conflict resolution: server-wins by default, conflicts stored for optional review
- SyncManager: starts on auth, listens for online/offline events, drains queue on reconnect
- SyncStatusIndicator component shows online/offline/syncing/error in sidebar

### Backend Sync Endpoints
- `GET /sync/changes?since=&entities=` — delta pull per entity type, returns records updated since timestamp
- `POST /sync/push` — receives mutation array, deduplicates by idempotency_key, records in sync_mutations table
- `POST /sync/resolve` — conflict resolution (accept_client, accept_server, discard)

### Group-Buy Cutoff
- Worker polls every 30s with `FOR UPDATE SKIP LOCKED` to prevent race conditions
- Marks campaigns succeeded if current_quantity >= min_quantity at cutoff, otherwise failed
- Join handler uses `FOR UPDATE` row lock + cutoff check + duplicate participation check

### Exports
- CSV: real SQL queries per report type, streamed to file
- PDF: minimal valid PDF spec (no external libraries), text-based summaries
- Filenames: `{report_type}_{YYYYMMDD_HHMMSS}.{csv|pdf}`
- Export jobs: queued → processing → completed/failed with file_path and error_msg
- Worker drains export queue every 5s

### Order Timeline
- Append-only `order_timeline_events` table — no UPDATE or DELETE operations
- Event types: creation, status_change, adjustment, split, cancellation, note, refund
- Each event stores actor_id, description, before/after JSONB snapshots

### Security
- Rate limiting: per-IP token bucket (100 req/s burst)
- Secure headers: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy
- CORS: configurable allowed origins
- All protected endpoints server-enforced via Auth + RequireRoles middleware

### Docker Orchestration
- Health checks on all services (pg_isready, redis-cli ping, wget /health)
- `depends_on` with `condition: service_healthy` for proper startup ordering
- Shared volume for export files between backend and worker
