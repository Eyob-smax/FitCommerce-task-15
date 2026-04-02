import { enqueue } from '../sync/offlineQueue';
import { ApiError, apiRequest } from './client';

export interface Item {
  id: string;
  name: string;
  sku: string | null;
  category: string;
  brand: string | null;
  condition: string;
  description: string | null;
  images: string[];
  deposit_amount: number;
  billing_model: string;
  price: number;
  status: string;
  location_id: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  version: number;
}

export interface ItemListResponse {
  data: Item[];
  meta: { page: number; per_page: number; total: number };
}

export interface AvailabilityWindow {
  id: string;
  item_id: string;
  starts_at: string;
  ends_at: string;
  created_at: string;
}

export interface CreateItemPayload {
  name: string;
  sku?: string;
  category: string;
  brand?: string;
  condition?: string;
  description?: string;
  images?: string[];
  deposit_amount?: number;
  billing_model?: string;
  price: number;
  location_id?: string;
}

export interface UpdateItemPayload {
  name?: string;
  sku?: string;
  category?: string;
  brand?: string;
  condition?: string;
  description?: string;
  images?: string[];
  deposit_amount?: number;
  billing_model?: string;
  price?: number;
}

export interface BatchUpdatePayload {
  item_ids: string[];
  price?: number;
  availability_windows?: Array<{ starts_at: string; ends_at: string }>;
}

export async function listItems(params?: Record<string, string>): Promise<ItemListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<ItemListResponse>(`/api/v1/items${qs}`, { rawResponse: true });
}

export function getItem(id: string): Promise<Item> {
  return apiRequest<Item>(`/api/v1/items/${id}`);
}

function shouldQueueOffline(err: unknown): boolean {
  if (typeof navigator !== 'undefined' && !navigator.onLine) return true;
  if (err instanceof ApiError) return false;
  return err instanceof TypeError;
}

function optimisticOfflineItem(payload: Partial<CreateItemPayload>, id?: string): Item {
  const now = new Date().toISOString();
  return {
    id: id ?? `offline-item-${Date.now()}`,
    name: payload.name ?? 'Pending Item',
    sku: payload.sku ?? null,
    category: payload.category ?? 'uncategorized',
    brand: payload.brand ?? null,
    condition: payload.condition ?? 'new',
    description: payload.description ?? null,
    images: payload.images ?? [],
    deposit_amount: payload.deposit_amount ?? 50,
    billing_model: payload.billing_model ?? 'one-time',
    price: payload.price ?? 0,
    status: 'draft',
    location_id: payload.location_id ?? null,
    created_by: null,
    created_at: now,
    updated_at: now,
    version: 0,
  };
}

export async function createItem(payload: CreateItemPayload): Promise<Item> {
  try {
    return await apiRequest<Item>('/api/v1/items', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('items', 'create', payload as unknown as Record<string, unknown>);
    return optimisticOfflineItem(payload);
  }
}

export function publishItem(id: string): Promise<Item> {
  return apiRequest<Item>(`/api/v1/items/${id}/publish`, { method: 'POST' });
}

export function unpublishItem(id: string): Promise<Item> {
  return apiRequest<Item>(`/api/v1/items/${id}/unpublish`, { method: 'POST' });
}

export async function updateItem(id: string, payload: UpdateItemPayload): Promise<Item> {
  try {
    return await apiRequest<Item>(`/api/v1/items/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('items', 'update', payload as unknown as Record<string, unknown>, id);
    return optimisticOfflineItem(payload, id);
  }
}

export async function deleteItem(id: string): Promise<void> {
  try {
    await apiRequest(`/api/v1/items/${id}`, { method: 'DELETE' });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    await enqueue('items', 'delete', { id }, id);
  }
}

export async function batchUpdateItems(payload: BatchUpdatePayload): Promise<{ updated: number }> {
  try {
    return await apiRequest('/api/v1/items/batch', {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (!shouldQueueOffline(err)) throw err;
    for (const itemId of payload.item_ids) {
      const mutationPayload: Record<string, unknown> = {};
      if (typeof payload.price === 'number') {
        mutationPayload.price = payload.price;
      }
      if (payload.availability_windows) {
        mutationPayload.availability_windows = payload.availability_windows;
      }
      await enqueue('items', 'update', mutationPayload, itemId);
    }
    return { updated: payload.item_ids.length };
  }
}

export function listAvailabilityWindows(itemId: string): Promise<AvailabilityWindow[]> {
  return apiRequest<AvailabilityWindow[]>(`/api/v1/items/${itemId}/availability-windows`);
}

export function addAvailabilityWindow(
  itemId: string,
  starts_at: string,
  ends_at: string,
): Promise<AvailabilityWindow> {
  return apiRequest<AvailabilityWindow>(`/api/v1/items/${itemId}/availability-windows`, {
    method: 'POST',
    body: JSON.stringify({ starts_at, ends_at }),
  });
}

export function deleteAvailabilityWindow(itemId: string, windowId: string): Promise<void> {
  return apiRequest(`/api/v1/items/${itemId}/availability-windows/${windowId}`, {
    method: 'DELETE',
  });
}
