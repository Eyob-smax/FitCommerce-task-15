import { apiRequest } from './client';

export interface UserItem {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  is_active: boolean;
  created_at: string;
}

export interface UserListResponse {
  data: UserItem[];
  meta: { page: number; per_page: number; total: number };
}

export interface CreateUserPayload {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  role: string;
}

export interface UpdateUserPayload {
  first_name?: string;
  last_name?: string;
  role?: string;
  is_active?: boolean;
}

export function listUsers(params?: Record<string, string>): Promise<UserListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<UserListResponse>(`/api/v1/users${qs}`, { rawResponse: true });
}

export function getUser(id: string): Promise<UserItem> {
  return apiRequest<UserItem>(`/api/v1/users/${id}`);
}

export function createUser(payload: CreateUserPayload): Promise<UserItem> {
  return apiRequest<UserItem>('/api/v1/users', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateUser(id: string, payload: UpdateUserPayload): Promise<UserItem> {
  return apiRequest<UserItem>(`/api/v1/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}

export function deactivateUser(id: string): Promise<void> {
  return apiRequest(`/api/v1/users/${id}`, { method: 'DELETE' });
}
