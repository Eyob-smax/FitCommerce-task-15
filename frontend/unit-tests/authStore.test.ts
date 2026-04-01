import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useAuthStore } from '../src/store/authStore';
import * as client from '../src/api/client';
import * as authApi from '../src/api/auth';
import type { TokenPair, User } from '../src/types/auth';

vi.mock('../src/api/client', () => ({
  getAccessToken: vi.fn(),
  getRefreshToken: vi.fn(),
  setTokens: vi.fn(),
  clearTokens: vi.fn(),
}));

vi.mock('../src/api/auth', () => ({
  login: vi.fn(),
  logout: vi.fn(),
  getMe: vi.fn(),
}));

const mockUser: User = {
  id: '1',
  email: 'admin@test.com',
  first_name: 'Admin',
  last_name: 'User',
  role: 'administrator',
};

const mockTokenPair: TokenPair = {
  access_token: 'at',
  refresh_token: 'rt',
  expires_in: 900,
  user: mockUser,
};

function resetStore() {
  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    isLoading: false,
    error: null,
  });
}

describe('authStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetStore();
  });

  it('login sets user and tokens on success', async () => {
    vi.mocked(authApi.login).mockResolvedValue(mockTokenPair);

    await useAuthStore.getState().login('admin@test.com', 'pass');

    const state = useAuthStore.getState();
    expect(state.user).toEqual(mockUser);
    expect(state.isAuthenticated).toBe(true);
    expect(state.isLoading).toBe(false);
    expect(client.setTokens).toHaveBeenCalledWith('at', 'rt');
  });

  it('login sets error on failure', async () => {
    vi.mocked(authApi.login).mockRejectedValue(new Error('bad creds'));

    await expect(useAuthStore.getState().login('a@b.com', 'wrong')).rejects.toThrow();

    const state = useAuthStore.getState();
    expect(state.user).toBeNull();
    expect(state.isAuthenticated).toBe(false);
    expect(state.error).toBe('bad creds');
  });

  it('logout clears state and calls API', async () => {
    useAuthStore.setState({ user: mockUser, isAuthenticated: true });
    vi.mocked(client.getRefreshToken).mockReturnValue('rt');
    vi.mocked(authApi.logout).mockResolvedValue(undefined);

    await useAuthStore.getState().logout();

    const state = useAuthStore.getState();
    expect(state.user).toBeNull();
    expect(state.isAuthenticated).toBe(false);
    expect(client.clearTokens).toHaveBeenCalled();
  });

  it('logout clears state even if API call fails', async () => {
    useAuthStore.setState({ user: mockUser, isAuthenticated: true });
    vi.mocked(client.getRefreshToken).mockReturnValue('rt');
    vi.mocked(authApi.logout).mockRejectedValue(new Error('network'));

    await useAuthStore.getState().logout();

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(client.clearTokens).toHaveBeenCalled();
  });

  it('restoreSession fetches user when access token exists', async () => {
    vi.mocked(client.getAccessToken).mockReturnValue('existing-token');
    vi.mocked(authApi.getMe).mockResolvedValue(mockUser);

    await useAuthStore.getState().restoreSession();

    const state = useAuthStore.getState();
    expect(state.user).toEqual(mockUser);
    expect(state.isAuthenticated).toBe(true);
    expect(state.isLoading).toBe(false);
  });

  it('restoreSession does not authenticate when no token', async () => {
    vi.mocked(client.getAccessToken).mockReturnValue(null);

    await useAuthStore.getState().restoreSession();

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(useAuthStore.getState().isLoading).toBe(false);
  });

  it('restoreSession clears state when getMe fails', async () => {
    vi.mocked(client.getAccessToken).mockReturnValue('stale-token');
    vi.mocked(authApi.getMe).mockRejectedValue(new Error('expired'));

    await useAuthStore.getState().restoreSession();

    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(client.clearTokens).toHaveBeenCalled();
  });

  it('forceLogout clears everything', () => {
    useAuthStore.setState({ user: mockUser, isAuthenticated: true });

    useAuthStore.getState().forceLogout();

    expect(useAuthStore.getState().user).toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
    expect(client.clearTokens).toHaveBeenCalled();
  });

  it('clearError only resets the error field', () => {
    useAuthStore.setState({ error: 'some error', user: mockUser, isAuthenticated: true });

    useAuthStore.getState().clearError();

    expect(useAuthStore.getState().error).toBeNull();
    expect(useAuthStore.getState().user).toEqual(mockUser);
  });
});
