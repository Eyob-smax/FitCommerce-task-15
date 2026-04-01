const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

const TOKEN_KEY = 'fc_access_token';
const REFRESH_KEY = 'fc_refresh_token';

export function getAccessToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}
export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}
export function setTokens(access: string, refresh: string): void {
  localStorage.setItem(TOKEN_KEY, access);
  localStorage.setItem(REFRESH_KEY, refresh);
}
export function clearTokens(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

let isRefreshing = false;
let refreshSubscribers: Array<(token: string) => void> = [];

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token));
  refreshSubscribers = [];
}

async function tryRefresh(): Promise<string | null> {
  const refreshToken = getRefreshToken();
  if (!refreshToken) return null;

  const res = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });

  if (!res.ok) {
    clearTokens();
    return null;
  }

  const data = await res.json();
  const newAccess: string = data.data.access_token;
  const newRefresh: string = data.data.refresh_token;
  setTokens(newAccess, newRefresh);
  return newAccess;
}

type RequestOptions = RequestInit & { skipAuth?: boolean; rawResponse?: boolean };

export async function apiRequest<T = unknown>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { skipAuth = false, rawResponse = false, ...fetchOptions } = options;
  const url = `${BASE_URL}${path}`;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(fetchOptions.headers as Record<string, string>),
  };

  if (!skipAuth) {
    const token = getAccessToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;
  }

  let res = await fetch(url, { ...fetchOptions, headers });

  // Auto-refresh on 401
  if (res.status === 401 && !skipAuth) {
    if (isRefreshing) {
      const newToken = await new Promise<string>(resolve => {
        refreshSubscribers.push(resolve);
      });
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(url, { ...fetchOptions, headers });
    } else {
      isRefreshing = true;
      const newToken = await tryRefresh();
      isRefreshing = false;
      if (!newToken) {
        // Refresh failed — dispatch logout event
        window.dispatchEvent(new CustomEvent('auth:logout'));
        throw new ApiError(401, 'Session expired');
      }
      onRefreshed(newToken);
      headers['Authorization'] = `Bearer ${newToken}`;
      res = await fetch(url, { ...fetchOptions, headers });
    }
  }

  if (!res.ok) {
    let errorBody: unknown;
    try { errorBody = await res.json(); } catch { errorBody = null; }
    throw new ApiError(res.status, res.statusText, errorBody);
  }

  // 204 No Content
  if (res.status === 204) return undefined as T;

  const json = await res.json();
  // rawResponse: return the full body (used for paginated list endpoints that need meta)
  if (rawResponse) return json as T;
  return (json.data ?? json) as T;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public body?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}
