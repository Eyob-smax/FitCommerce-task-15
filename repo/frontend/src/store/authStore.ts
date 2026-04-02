import { create } from 'zustand';
import { getAccessToken, getRefreshToken, setTokens, clearTokens } from '../api/client';
import { login as apiLogin, logout as apiLogout, getMe } from '../api/auth';
import type { User } from '../types/auth';

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;

  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  restoreSession: () => Promise<void>;
  clearError: () => void;
  // Called by the API client on auth:logout event
  forceLogout: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isAuthenticated: false,
  isLoading: true,
  error: null,

  login: async (email, password) => {
    set({ isLoading: true, error: null });
    try {
      const data = await apiLogin(email, password);
      setTokens(data.access_token, data.refresh_token);
      set({ user: data.user, isAuthenticated: true, isLoading: false, error: null });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : 'Login failed. Check your credentials.';
      set({ isLoading: false, error: message, isAuthenticated: false, user: null });
      throw err;
    }
  },

  logout: async () => {
    const refreshToken = getRefreshToken();
    try {
      if (refreshToken) await apiLogout(refreshToken);
    } catch {
      // Ignore logout API errors
    } finally {
      clearTokens();
      set({ user: null, isAuthenticated: false, isLoading: false, error: null });
    }
  },

  restoreSession: async () => {
    set({ isLoading: true });
    const accessToken = getAccessToken();
    if (!accessToken) {
      set({ isLoading: false, isAuthenticated: false });
      return;
    }
    try {
      const user = await getMe();
      set({ user, isAuthenticated: true, isLoading: false });
    } catch {
      // Token expired or invalid — clearTokens is handled by the API client's
      // refresh flow; if refresh also failed, the auth:logout event fires.
      clearTokens();
      set({ user: null, isAuthenticated: false, isLoading: false });
    }
  },

  clearError: () => set({ error: null }),

  forceLogout: () => {
    clearTokens();
    set({ user: null, isAuthenticated: false, isLoading: false });
  },
}));

// Listen for the global logout event emitted by the API client when refresh fails
if (typeof window !== 'undefined') {
  window.addEventListener('auth:logout', () => {
    useAuthStore.getState().forceLogout();
  });
}
