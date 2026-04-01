export type MutationOperation = 'create' | 'update' | 'delete';

export type MutationStatus = 'pending' | 'syncing' | 'applied' | 'rejected' | 'conflict';

export type EntityType =
  | 'items'
  | 'group_buys'
  | 'orders'
  | 'members'
  | 'classes'
  | 'inventory';

export interface QueuedMutation {
  /** Auto-incremented local sequence number (Dexie PK) */
  seq?: number;
  /** UUID generated client-side for server-side deduplication */
  idempotency_key: string;
  /** Stable device/session identifier */
  client_id: string;
  entity_type: EntityType;
  entity_id?: string;
  operation: MutationOperation;
  payload: Record<string, unknown>;
  status: MutationStatus;
  retry_count: number;
  created_at: number; // unix ms
  last_attempt_at?: number;
  error?: string;
}

export interface SyncMeta {
  entity_type: EntityType;
  last_synced_at: number; // unix ms — sent as `since` on next pull
}

export interface SyncConflict {
  mutation_seq: number;
  entity_type: EntityType;
  entity_id: string;
  client_payload: Record<string, unknown>;
  server_payload: Record<string, unknown>;
  detected_at: number;
}

export type SyncStatus = 'online' | 'offline' | 'syncing' | 'error';
