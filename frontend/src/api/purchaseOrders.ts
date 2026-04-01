import { apiRequest } from './client';

export interface POLine {
  id: string;
  po_id: string;
  item_id: string;
  quantity: number;
  unit_cost: number;
  received_quantity: number;
}

export interface PurchaseOrder {
  id: string;
  supplier_id: string;
  location_id: string;
  status: string;
  notes: string | null;
  issued_at: string | null;
  expected_at: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  version: number;
  lines?: POLine[];
}

export interface POListResponse {
  data: PurchaseOrder[];
  meta: { page: number; per_page: number; total: number };
}

export interface CreatePOLinePayload {
  item_id: string;
  quantity: number;
  unit_cost: number;
}

export interface CreatePOPayload {
  supplier_id: string;
  location_id: string;
  notes?: string;
  expected_at?: string;
  lines: CreatePOLinePayload[];
}

export interface ReceiveLinePayload {
  po_line_item_id: string;
  quantity_received: number;
  discrepancy_notes?: string;
}

export interface ReceivePayload {
  notes?: string;
  lines: ReceiveLinePayload[];
}

export async function listPurchaseOrders(params?: Record<string, string>): Promise<POListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<POListResponse>(`/api/v1/purchase-orders${qs}`, { rawResponse: true });
}

export function getPurchaseOrder(id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/api/v1/purchase-orders/${id}`);
}

export function createPurchaseOrder(payload: CreatePOPayload): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>('/api/v1/purchase-orders', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function issuePurchaseOrder(id: string): Promise<PurchaseOrder> {
  return apiRequest<PurchaseOrder>(`/api/v1/purchase-orders/${id}/issue`, { method: 'POST' });
}

export function cancelPurchaseOrder(id: string): Promise<void> {
  return apiRequest(`/api/v1/purchase-orders/${id}/cancel`, { method: 'POST' });
}

export function receivePurchaseOrder(id: string, payload: ReceivePayload): Promise<unknown> {
  return apiRequest(`/api/v1/purchase-orders/${id}/receive`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
