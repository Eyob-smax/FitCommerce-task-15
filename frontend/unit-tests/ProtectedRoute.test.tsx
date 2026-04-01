import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { ProtectedRoute, RoleGuard } from '../src/components/auth/ProtectedRoute';
import { useAuthStore } from '../src/store/authStore';
import type { User } from '../src/types/auth';

const adminUser: User = {
  id: '1',
  email: 'admin@test.com',
  first_name: 'Admin',
  last_name: 'User',
  role: 'administrator',
};

const memberUser: User = {
  id: '2',
  email: 'member@test.com',
  first_name: 'Member',
  last_name: 'User',
  role: 'member',
};

function resetStore(overrides: Partial<ReturnType<typeof useAuthStore.getState>> = {}) {
  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    isLoading: false,
    error: null,
    ...overrides,
  });
}

function renderWithRouter(ui: React.ReactElement, initialPath = '/protected') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/login" element={<div>Login Page</div>} />
        <Route path="/forbidden" element={<div>Forbidden Page</div>} />
        <Route path="/protected" element={ui} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ProtectedRoute', () => {
  beforeEach(() => resetStore());

  it('shows loading spinner while isLoading', () => {
    resetStore({ isLoading: true });
    renderWithRouter(
      <ProtectedRoute>
        <div>Secret</div>
      </ProtectedRoute>,
    );
    expect(screen.queryByText('Secret')).toBeNull();
    expect(screen.getByRole('progressbar')).toBeTruthy();
  });

  it('redirects to /login when not authenticated', () => {
    resetStore({ isLoading: false, isAuthenticated: false });
    renderWithRouter(
      <ProtectedRoute>
        <div>Secret</div>
      </ProtectedRoute>,
    );
    expect(screen.getByText('Login Page')).toBeTruthy();
  });

  it('renders children when authenticated with no role requirement', () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: adminUser });
    renderWithRouter(
      <ProtectedRoute>
        <div>Secret</div>
      </ProtectedRoute>,
    );
    expect(screen.getByText('Secret')).toBeTruthy();
  });

  it('renders children when user has matching role', () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: adminUser });
    renderWithRouter(
      <ProtectedRoute roles={['administrator']}>
        <div>Admin Area</div>
      </ProtectedRoute>,
    );
    expect(screen.getByText('Admin Area')).toBeTruthy();
  });

  it('redirects to /forbidden when user lacks required role', () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: memberUser });
    renderWithRouter(
      <ProtectedRoute roles={['administrator']}>
        <div>Admin Area</div>
      </ProtectedRoute>,
    );
    expect(screen.getByText('Forbidden Page')).toBeTruthy();
  });
});

describe('RoleGuard', () => {
  beforeEach(() => resetStore());

  it('renders children when user has role', () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: adminUser });
    renderWithRouter(
      <RoleGuard roles={['administrator']}>
        <div>Admin Section</div>
      </RoleGuard>,
    );
    expect(screen.getByText('Admin Section')).toBeTruthy();
  });

  it('renders nothing when user lacks role', () => {
    resetStore({ isLoading: false, isAuthenticated: true, user: memberUser });
    renderWithRouter(
      <RoleGuard roles={['administrator']}>
        <div>Admin Section</div>
      </RoleGuard>,
    );
    expect(screen.queryByText('Admin Section')).toBeNull();
  });

  it('renders nothing when no user', () => {
    resetStore();
    renderWithRouter(
      <RoleGuard roles={['administrator']}>
        <div>Admin Section</div>
      </RoleGuard>,
    );
    expect(screen.queryByText('Admin Section')).toBeNull();
  });
});
