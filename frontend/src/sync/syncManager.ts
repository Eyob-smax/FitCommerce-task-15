import { db } from '../db/schema';
import { getPendingMutations, markApplied, markRejected, scheduleRetry } from './offlineQueue';
import type { EntityType, SyncStatus } from './types';

const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';
const SYNC_ENTITIES: EntityType[] = ['items', 'group_buys', 'orders', 'members'];
const MAX_RETRIES = 5;

type StatusListener = (status: SyncStatus) => void;

class SyncManager {
  private status: SyncStatus = 'offline';
  private listeners = new Set<StatusListener>();
  private draining = false;
  private onlineHandler: () => void;
  private offlineHandler: () => void;

  constructor() {
    this.onlineHandler = () => this.handleOnline();
    this.offlineHandler = () => this.handleOffline();
  }

  start(): void {
    window.addEventListener('online', this.onlineHandler);
    window.addEventListener('offline', this.offlineHandler);

    // Bootstrap — check current connectivity
    if (navigator.onLine) {
      this.setStatus('online');
      void this.drainQueue();
      void this.pullChanges();
    } else {
      this.setStatus('offline');
    }
  }

  stop(): void {
    window.removeEventListener('online', this.onlineHandler);
    window.removeEventListener('offline', this.offlineHandler);
  }

  subscribe(fn: StatusListener): () => void {
    this.listeners.add(fn);
    fn(this.status); // emit current status immediately
    return () => this.listeners.delete(fn);
  }

  getStatus(): SyncStatus {
    return this.status;
  }

  // ── Internal ───────────────────────────────────────────────────────────────

  private setStatus(s: SyncStatus): void {
    if (this.status === s) return;
    this.status = s;
    this.listeners.forEach(fn => fn(s));
  }

  private handleOnline(): void {
    this.setStatus('online');
    void this.drainQueue();
    void this.pullChanges();
  }

  private handleOffline(): void {
    this.setStatus('offline');
  }

  /**
   * Drain the local mutation queue by pushing pending mutations to the server.
   * Runs serially to maintain causality.
   */
  async drainQueue(): Promise<void> {
    if (this.draining || !navigator.onLine) return;
    this.draining = true;
    this.setStatus('syncing');

    try {
      const pending = await getPendingMutations();
      for (const mutation of pending) {
        if (!navigator.onLine) break;
        if (mutation.retry_count >= MAX_RETRIES) {
          await markRejected(mutation.seq!, 'max retries exceeded');
          continue;
        }

        try {
          const res = await fetch(`${API_BASE}/api/v1/sync/push`, {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              Authorization: `Bearer ${getAccessToken()}`,
            },
            body: JSON.stringify({
              mutations: [
                {
                  idempotency_key: mutation.idempotency_key,
                  client_id: mutation.client_id,
                  entity_type: mutation.entity_type,
                  entity_id: mutation.entity_id,
                  operation: mutation.operation,
                  payload: mutation.payload,
                },
              ],
            }),
          });

          if (res.ok) {
            const body = await res.json().catch(() => null) as { data?: Array<Record<string, unknown>> } | null;
            const result = Array.isArray(body?.data) ? body!.data![0] : undefined;
            const status = typeof result?.status === 'string' ? result.status : '';

            if (status === 'applied') {
              await markApplied(mutation.seq!);
            } else if (status === 'conflict') {
              const conflictPayload = (result?.conflict_data as Record<string, unknown> | undefined) ?? {};
              await db.conflicts.add({
                mutation_seq: mutation.seq!,
                entity_type: mutation.entity_type,
                entity_id: mutation.entity_id ?? '',
                client_payload: mutation.payload,
                server_payload: conflictPayload,
                detected_at: Date.now(),
              });
              await markRejected(mutation.seq!, 'conflict');
            } else if (status === 'rejected') {
              const err = typeof result?.error === 'string' ? result.error : 'server rejected mutation';
              await markRejected(mutation.seq!, err);
            } else {
              await scheduleRetry(mutation.seq!);
            }
          } else if (res.status === 409) {
            // Conflict — server has a newer version
            const body = await res.json();
            await db.conflicts.add({
              mutation_seq: mutation.seq!,
              entity_type: mutation.entity_type,
              entity_id: mutation.entity_id ?? '',
              client_payload: mutation.payload,
              server_payload: body?.data ?? {},
              detected_at: Date.now(),
            });
            await markRejected(mutation.seq!, 'conflict');
          } else if (res.status >= 400 && res.status < 500) {
            await markRejected(mutation.seq!, `server rejected: ${res.status}`);
          } else {
            await scheduleRetry(mutation.seq!);
          }
        } catch {
          await scheduleRetry(mutation.seq!);
        }
      }
    } finally {
      this.draining = false;
      this.setStatus(navigator.onLine ? 'online' : 'offline');
    }
  }

  /**
   * Pull server-side changes since the last known timestamp for each entity.
   */
  async pullChanges(): Promise<void> {
    if (!navigator.onLine) return;

    for (const entityType of SYNC_ENTITIES) {
      try {
        const meta = await db.syncMeta.get(entityType);
        const since = meta?.last_synced_at ?? 0;

        const res = await fetch(
          `${API_BASE}/api/v1/sync/changes?since=${Math.floor(since / 1000)}&entities=${entityType}`,
          {
            headers: { Authorization: `Bearer ${getAccessToken()}` },
          },
        );

        if (!res.ok) continue;

        const { data, synced_at } = await res.json();

        // Server returns { data: { items: [...], orders: [...], ... }, synced_at }
        // Extract the array for the specific entity type being synced.
        const records = data?.[entityType];
        if (Array.isArray(records)) {
          await hydrate(entityType, records);
        }

        // Update sync cursor
        await db.syncMeta.put({ entity_type: entityType, last_synced_at: synced_at * 1000 });
      } catch {
        // Network failure during pull — silently continue; we'll retry on next reconnect
      }
    }
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function getAccessToken(): string {
  return localStorage.getItem('fc_access_token') ?? '';
}

async function hydrate(entityType: EntityType, records: Record<string, unknown>[]): Promise<void> {
  const now = Date.now();
  const withTimestamp = records.map(r => ({ ...r, cached_at: now }));

  switch (entityType) {
    case 'items':
      await db.items.bulkPut(withTimestamp as never);
      break;
    case 'group_buys':
      await db.groupBuys.bulkPut(withTimestamp as never);
      break;
    case 'orders':
      await db.orders.bulkPut(withTimestamp as never);
      break;
    case 'members':
      await db.members.bulkPut(withTimestamp as never);
      break;
  }
}

// ── Singleton export ──────────────────────────────────────────────────────────
export const syncManager = new SyncManager();
