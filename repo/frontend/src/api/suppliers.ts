import { apiRequest } from './client';

export interface Supplier {
  id: string;
  name: string;
  contact_name: string | null;
  email: string | null;
  phone: string | null;
  address: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface SupplierListResponse {
  data: Supplier[];
  meta: { page: number; per_page: number; total: number };
}

export interface CreateSupplierPayload {
  name: string;
  contact_name?: string;
  email?: string;
  phone?: string;
  address?: string;
  is_active?: boolean;
}

export interface UpdateSupplierPayload {
  name?: string;
  contact_name?: string;
  email?: string;
  phone?: string;
  address?: string;
  is_active?: boolean;
}

export async function listSuppliers(params?: Record<string, string>): Promise<SupplierListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<SupplierListResponse>(`/api/v1/suppliers${qs}`, { rawResponse: true });
}

export function getSupplier(id: string): Promise<Supplier> {
  return apiRequest<Supplier>(`/api/v1/suppliers/${id}`);
}

export function createSupplier(payload: CreateSupplierPayload): Promise<Supplier> {
  return apiRequest<Supplier>('/api/v1/suppliers', {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function updateSupplier(id: string, payload: UpdateSupplierPayload): Promise<Supplier> {
  return apiRequest<Supplier>(`/api/v1/suppliers/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload),
  });
}
