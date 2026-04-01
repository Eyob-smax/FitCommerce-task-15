import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useAuthStore } from '../src/store/authStore';

vi.mock('../src/api/reports', () => ({
  getDashboard: vi.fn().mockResolvedValue({
    member_growth: 5,
    member_churn: 1,
    renewal_rate: 85.0,
    engagement: 42,
    class_fill_rate: 75.5,
    coach_productivity: 12,
    period: 'monthly',
    start_date: '2026-04-01',
    end_date: '2026-04-30',
  }),
  createExport: vi.fn(),
}));

import { DashboardPage } from '../src/pages/DashboardPage';

function renderDashboard() {
  useAuthStore.setState({
    user: { id: '1', email: 'admin@test.com', first_name: 'Admin', last_name: 'User', role: 'administrator' },
    isAuthenticated: true,
    isLoading: false,
    error: null,
  });
  return render(
    <MemoryRouter>
      <DashboardPage />
    </MemoryRouter>,
  );
}

describe('DashboardPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders KPI cards after loading', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText('Member Growth')).toBeTruthy();
      expect(screen.getByText('5')).toBeTruthy();
    });
  });

  it('renders all 6 KPI labels', async () => {
    renderDashboard();
    await waitFor(() => {
      const labels = ['Member Growth', 'Member Churn', 'Renewal Rate', 'Engagement Events', 'Class Fill Rate', 'Coach Productivity'];
      labels.forEach((label) => {
        expect(screen.getByText(label)).toBeTruthy();
      });
    });
  });

  it('shows period info', async () => {
    renderDashboard();
    await waitFor(() => {
      expect(screen.getByText(/2026-04-01/)).toBeTruthy();
    });
  });
});
