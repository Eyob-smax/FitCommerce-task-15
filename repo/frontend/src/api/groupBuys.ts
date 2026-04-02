import { enqueue } from '../sync/offlineQueue';
import { ApiError, apiRequest } from './client';

export interface GroupBuy {
  id: string;
  item_id: string;
  location_id: string;
  created_by: string | null;
  title: string;
  description: string | null;
  min_quantity: number;
  current_quantity: number;
  status: string;
  cutoff_at: string;
  price_per_unit: number;
  notes: string | null;
  created_at: string;
  updated_at: string;
  version: number;
  progress: number;
}

export interface GroupBuyListResponse {
  data: GroupBuy[];
  meta: { page: number; per_page: number; total: number };
}

export interface Participant {
  id: string;
  group_buy_id: string;
  member_id: string;
  quantity: number;
  joined_at: string;
  status: string;
}

export interface CreateGroupBuyPayload {
  item_id: string;
  location_id: string;
  title: string;
  description?: string;
  min_quantity: number;
  cutoff_at: string;
  price_per_unit: number;
  notes?: string;
}

export async function listGroupBuys(params?: Record<string, string>): Promise<GroupBuyListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<GroupBuyListResponse>(`/api/v1/group-buys${qs}`, { rawResponse: true });
}

export function getGroupBuy(id: string): Promise<GroupBuy> {
  return apiRequest<GroupBuy>(`/api/v1/group-buys/${id}`);
}

function shouldQueueOffline(err: unknown): boolean {
  if (typeof navigator !== 'undefined' && !navigator.onLine) return true;
  if (err instanceof ApiError) return false;
  return err instanceof TypeError;
}

function optimisticGroupBuy(payload: CreateGroupBuyPayload, id?: string): GroupBuy {
  const now = new Date().toISOString();
  return {
    id: id ?? `offline-gb-${Date.now()}`,
    item_id: payload.item_id,
    location_id: payload.location_id,
    created_by: null,
    title: payload.title,
    description: payload.description ?? null,
    min_quantity: payload.min_quantity,
    current_quantity: 0,
    status: 'published',
    cutoff_at: payload.cutoff_at,
    price_per_unit: payload.price_per_unit,
    notes: payload.notes ?? null,
    created_at: now,
    updated_at: now,
    version: 0,
    progress: 0,
  };
}

export async function createGroupBuy(payload: CreateGroupBuyPayload): Promise<GroupBuy> {
  try {
    return await apiRequest<GroupBuy>('/api/v1/group-buys', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('group_buys', 'create', payload as unknown as Record<string, unknown>);
    return optimisticGroupBuy(payload);
  }
}

export function publishGroupBuy(id: string): Promise<GroupBuy> {
  return apiRequest<GroupBuy>(`/api/v1/group-buys/${id}/publish`, { method: 'POST' });
}

export function cancelGroupBuy(id: string): Promise<void> {
  return apiRequest(`/api/v1/group-buys/${id}/cancel`, { method: 'POST' });
}

export async function joinGroupBuy(id: string, quantity?: number): Promise<Participant> {
  const qty = quantity ?? 1;
  try {
    return await apiRequest<Participant>(`/api/v1/group-buys/${id}/join`, {
      method: 'POST',
      body: JSON.stringify({ quantity: qty }),
    });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('group_buys', 'update', { action: 'join', quantity: qty, group_buy_id: id }, id);
    return {
      id: `offline-part-${Date.now()}`,
      group_buy_id: id,
      member_id: 'pending',
      quantity: qty,
      joined_at: new Date().toISOString(),
      status: 'committed',
    };
  }
}

export async function leaveGroupBuy(id: string): Promise<void> {
  try {
    await apiRequest(`/api/v1/group-buys/${id}/leave`, { method: 'DELETE' });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('group_buys', 'update', { action: 'leave', group_buy_id: id }, id);
  }
}

export function getParticipants(id: string): Promise<Participant[]> {
  return apiRequest<Participant[]>(`/api/v1/group-buys/${id}/participants`);
}
