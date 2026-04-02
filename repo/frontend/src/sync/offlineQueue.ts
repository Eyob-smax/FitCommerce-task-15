import { v4 as uuidv4 } from 'uuid';
import { db } from '../db/schema';
import type { EntityType, MutationOperation, QueuedMutation } from './types';

const CLIENT_ID_KEY = 'fitcommerce_client_id';

function getClientId(): string {
  let id = localStorage.getItem(CLIENT_ID_KEY);
  if (!id) {
    id = uuidv4();
    localStorage.setItem(CLIENT_ID_KEY, id);
  }
  return id;
}

/**
 * Enqueues a mutation for eventual sync. Called instead of a direct API call
 * when the app is offline, or as part of an optimistic-update flow.
 */
export async function enqueue(
  entityType: EntityType,
  operation: MutationOperation,
  payload: Record<string, unknown>,
  entityId?: string,
): Promise<number> {
  const mutation: QueuedMutation = {
    idempotency_key: uuidv4(),
    client_id: getClientId(),
    entity_type: entityType,
    entity_id: entityId,
    operation,
    payload,
    status: 'pending',
    retry_count: 0,
    created_at: Date.now(),
  };
  return db.mutations.add(mutation);
}

/** Returns all pending mutations sorted by creation time (drain order). */
export function getPendingMutations(): Promise<QueuedMutation[]> {
  return db.mutations
    .where('status')
    .equals('pending')
    .sortBy('created_at');
}

/** Marks a mutation as successfully applied and removes it from the queue. */
export async function markApplied(seq: number): Promise<void> {
  await db.mutations.delete(seq);
}

/** Marks a mutation as rejected (server refused, no retry). */
export async function markRejected(seq: number, error: string): Promise<void> {
  await db.mutations.update(seq, { status: 'rejected', error });
}

/** Increments the retry count and resets status to pending for retry. */
export async function scheduleRetry(seq: number): Promise<void> {
  const m = await db.mutations.get(seq);
  if (!m) return;
  await db.mutations.update(seq, {
    status: 'pending',
    retry_count: m.retry_count + 1,
    last_attempt_at: Date.now(),
  });
}

/** Returns the total count of pending mutations (badge indicator). */
export function pendingCount(): Promise<number> {
  return db.mutations.where('status').equals('pending').count();
}

/** Clears all applied/rejected mutations older than `maxAgeMs`. */
export async function pruneProcessed(maxAgeMs = 7 * 24 * 60 * 60 * 1000): Promise<void> {
  const cutoff = Date.now() - maxAgeMs;
  await db.mutations
    .where('status')
    .anyOf(['rejected'])
    .and(m => m.created_at < cutoff)
    .delete();
}
