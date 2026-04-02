import { apiRequest } from './client';
import type { TokenPair, User } from '../types/auth';

export async function login(email: string, password: string): Promise<TokenPair> {
  return apiRequest<TokenPair>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
    skipAuth: true,
  });
}

export async function logout(refreshToken: string): Promise<void> {
  await apiRequest('/api/v1/auth/logout', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

export async function getMe(): Promise<User> {
  return apiRequest<User>('/api/v1/users/me');
}
