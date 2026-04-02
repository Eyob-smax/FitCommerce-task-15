import { apiRequest } from './client';

export interface OrderLine {
  id: string;
  order_id: string;
  item_id: string;
  quantity: number;
  unit_price: number;
  deposit_per_unit: number;
}

export interface Order {
  id: string;
  group_buy_id: string | null;
  member_id: string;
  location_id: string;
  status: string;
  total_amount: number;
  deposit_amount: number;
  notes: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  version: number;
  lines?: OrderLine[];
}

export interface OrderListResponse {
  data: Order[];
  meta: { page: number; per_page: number; total: number };
}

export interface OrderNote {
  id: string;
  order_id: string;
  author_id: string;
  content: string;
  created_at: string;
}

export interface TimelineEvent {
  id: string;
  order_id: string;
  actor_id: string | null;
  event_type: string;
  description: string;
  before_snapshot: unknown;
  after_snapshot: unknown;
  occurred_at: string;
}

export async function listOrders(params?: Record<string, string>): Promise<OrderListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<OrderListResponse>(`/api/v1/orders${qs}`, { rawResponse: true });
}

export function getOrder(id: string): Promise<Order> {
  return apiRequest<Order>(`/api/v1/orders/${id}`);
}

export function cancelOrder(id: string, reason?: string): Promise<void> {
  return apiRequest(`/api/v1/orders/${id}/cancel`, {
    method: 'POST',
    body: JSON.stringify({ reason: reason ?? 'Cancelled' }),
  });
}

export function adjustOrder(id: string, lineId: string, newQuantity: number, reason: string): Promise<Order> {
  return apiRequest<Order>(`/api/v1/orders/${id}/adjust`, {
    method: 'POST',
    body: JSON.stringify({ line_id: lineId, new_quantity: newQuantity, reason }),
  });
}

export function splitOrder(
  id: string,
  lines: { line_id: string; quantity: number }[],
  reason: string,
): Promise<{ original_order_id: string; new_order_id: string; split_total: number }> {
  return apiRequest(`/api/v1/orders/${id}/split`, {
    method: 'POST',
    body: JSON.stringify({ lines, reason }),
  });
}

export function changeOrderStatus(id: string, status: string, reason: string): Promise<Order> {
  return apiRequest<Order>(`/api/v1/orders/${id}/status`, {
    method: 'POST',
    body: JSON.stringify({ status, reason }),
  });
}

export function getTimeline(id: string): Promise<TimelineEvent[]> {
  return apiRequest<TimelineEvent[]>(`/api/v1/orders/${id}/timeline`);
}

export function addNote(id: string, content: string): Promise<OrderNote> {
  return apiRequest<OrderNote>(`/api/v1/orders/${id}/notes`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  });
}

export function listNotes(id: string): Promise<OrderNote[]> {
  return apiRequest<OrderNote[]>(`/api/v1/orders/${id}/notes`);
}
