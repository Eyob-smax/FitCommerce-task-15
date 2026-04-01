import { ApiError, apiRequest, getAccessToken } from './client';

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export interface DashboardKPI {
  member_growth: number;
  member_churn: number;
  renewal_rate: number;
  engagement: number;
  class_fill_rate: number;
  coach_productivity: number;
  period: string;
  start_date: string;
  end_date: string;
}

export interface ExportJob {
  id: string;
  report_type: string;
  format: string;
  status: string;
  file_path: string | null;
  error_msg: string | null;
  created_by: string | null;
  created_at: string;
}

export interface ExportListResponse {
  data: ExportJob[];
  meta: { page: number; per_page: number; total: number };
}

export function getDashboard(params?: Record<string, string>): Promise<DashboardKPI> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<DashboardKPI>(`/api/v1/reports/dashboard${qs}`);
}

export function getMemberGrowth(params?: Record<string, string>) {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<{ new_members: number; total_members: number }>(`/api/v1/reports/member-growth${qs}`);
}

export function getChurn(params?: Record<string, string>) {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<{ churned_members: number; total_members: number; churn_rate: number }>(
    `/api/v1/reports/churn${qs}`,
  );
}

export function getCoachReport(coachId: string, params?: Record<string, string>) {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest(`/api/v1/reports/coach/${coachId}${qs}`);
}

export function createExport(reportType: string, format: string, filters?: Record<string, string>): Promise<ExportJob> {
  return apiRequest<ExportJob>('/api/v1/exports', {
    method: 'POST',
    body: JSON.stringify({ report_type: reportType, format, filters: filters ?? {} }),
  });
}

export async function listExports(params?: Record<string, string>): Promise<ExportListResponse> {
  const qs = params ? '?' + new URLSearchParams(params).toString() : '';
  return apiRequest<ExportListResponse>(`/api/v1/exports${qs}`, { rawResponse: true });
}

export function getExport(id: string): Promise<ExportJob> {
  return apiRequest<ExportJob>(`/api/v1/exports/${id}`);
}

export async function downloadExport(id: string): Promise<{ blob: Blob; filename: string }> {
  const token = getAccessToken();
  if (!token) {
    throw new Error('Missing access token');
  }

  const res = await fetch(`${BASE_URL}/api/v1/exports/${id}/download`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!res.ok) {
    let errorBody: unknown;
    try {
      errorBody = await res.json();
    } catch {
      errorBody = null;
    }
    throw new ApiError(res.status, res.statusText, errorBody);
  }

  const contentDisposition = res.headers.get('Content-Disposition') ?? '';
  const match = contentDisposition.match(/filename\*?=(?:UTF-8''|"?)([^";]+)/i);
  const fallback = `export_${id}`;
  const rawName = match?.[1] ?? fallback;
  const filename = decodeURIComponent(rawName.trim());

  const blob = await res.blob();
  return { blob, filename };
}
