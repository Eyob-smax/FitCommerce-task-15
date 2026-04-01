import { apiRequest } from './client';

export interface StockRecord {
  id: string;
  item_id: string;
  item_name: string;
  location_id: string;
  on_hand: number;
  reserved: number;
  allocated: number;
  in_rental: number;
  returned: number;
  damaged: number;
  available: number;
  updated_at: string;
}

export interface StockListResponse {
  data: StockRecord[];
  meta: { page: number; per_page: number; total: number };
}

export interface StockAdjustment {
  id: string;
  item_id: string;
  location_id: string;
  quantity_change: number;
  previous_on_hand: number;
  new_on_hand: number;
  reason_code: string;
  notes: string | null;
  adjusted_by: string | null;
  created_at: string;
}

export interface AdjustPayload {
  quantity_change: number;
  reason_code: string;
  notes?: string;
}

export async function listInventory(params?: Record<string, string>): Promise<StockListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<StockListResponse>(`/api/v1/inventory${qs}`, { rawResponse: true });
}

export function getStock(itemId: string): Promise<StockRecord> {
  return apiRequest<StockRecord>(`/api/v1/inventory/${itemId}`);
}

export function adjustStock(itemId: string, payload: AdjustPayload): Promise<StockAdjustment> {
  return apiRequest<StockAdjustment>(`/api/v1/inventory/${itemId}/adjust`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function listAdjustments(itemId: string): Promise<StockAdjustment[]> {
  return apiRequest<StockAdjustment[]>(`/api/v1/inventory/${itemId}/adjustments`);
}
