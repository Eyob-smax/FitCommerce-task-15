import { apiRequest } from './client';

export interface ClassItem {
  id: string;
  coach_id: string;
  location_id: string;
  name: string;
  description: string | null;
  scheduled_at: string;
  duration_minutes: number;
  capacity: number;
  booked_seats: number;
  status: string;
  created_at: string;
}

export interface ClassListResponse {
  data: ClassItem[];
  meta: { page: number; per_page: number; total: number };
}

export interface CreateClassPayload {
  coach_id: string;
  location_id: string;
  name: string;
  description?: string;
  scheduled_at: string;
  duration_minutes: number;
  capacity: number;
}

export interface UpdateClassPayload {
  name?: string;
  description?: string;
  scheduled_at?: string;
  duration_minutes?: number;
  capacity?: number;
}

export function listClasses(params?: Record<string, string>): Promise<ClassListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<ClassListResponse>(`/api/v1/classes${qs}`, { rawResponse: true });
}

export function getClass(id: string): Promise<ClassItem> {
  return apiRequest<ClassItem>(`/api/v1/classes/${id}`);
}

export function createClass(payload: CreateClassPayload): Promise<ClassItem> {
  return apiRequest<ClassItem>('/api/v1/classes', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateClass(id: string, payload: UpdateClassPayload): Promise<ClassItem> {
  return apiRequest<ClassItem>(`/api/v1/classes/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}

export function cancelClass(id: string): Promise<void> {
  return apiRequest(`/api/v1/classes/${id}/cancel`, { method: 'POST' });
}

export function bookClass(id: string): Promise<{ booking_id: string; class_id: string; member_id: string; status: string; booked_at: string }> {
  return apiRequest(`/api/v1/classes/${id}/book`, { method: 'POST' });
}

export function cancelClassBooking(id: string): Promise<void> {
  return apiRequest(`/api/v1/classes/${id}/book`, { method: 'DELETE' });
}
