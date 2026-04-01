import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { useAuthStore } from '../src/store/authStore';
import { ItemFormPage } from '../src/pages/items/ItemFormPage';

// Mock the API
vi.mock('../src/api/items', () => ({
  createItem: vi.fn(),
  getItem: vi.fn(),
  updateItem: vi.fn(),
}));

import { createItem } from '../src/api/items';

function renderForm() {
  useAuthStore.setState({
    user: { id: '1', email: 'admin@test.com', first_name: 'Admin', last_name: 'User', role: 'administrator' },
    isAuthenticated: true,
    isLoading: false,
    error: null,
  });
  return render(
    <MemoryRouter initialEntries={['/items/new']}>
      <Routes>
        <Route path="/items/new" element={<ItemFormPage />} />
        <Route path="/items" element={<div>Item List</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ItemFormPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders create form with required fields', () => {
    renderForm();
    expect(screen.getByLabelText(/name/i)).toBeTruthy();
    expect(screen.getByLabelText(/category/i)).toBeTruthy();
    expect(screen.getByLabelText(/price/i)).toBeTruthy();
  });

  it('shows validation errors for empty required fields', async () => {
    renderForm();
    fireEvent.click(screen.getByText('Create'));
    await waitFor(() => {
      expect(screen.getByText('Required')).toBeTruthy();
    });
  });

  it('calls createItem on valid submit', async () => {
    vi.mocked(createItem).mockResolvedValue({
      id: '1', name: 'Test', sku: null, category: 'gear', brand: null,
      condition: 'new', description: null, images: [],
      deposit_amount: 50, billing_model: 'one-time', price: 25,
      status: 'draft', location_id: null, created_by: null,
      created_at: '', updated_at: '', version: 1,
    });

    renderForm();
    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'Test' } });
    fireEvent.change(screen.getByLabelText(/category/i), { target: { value: 'gear' } });
    fireEvent.change(screen.getByLabelText(/^price/i), { target: { value: '25' } });
    fireEvent.click(screen.getByText('Create'));

    await waitFor(() => {
      expect(createItem).toHaveBeenCalled();
    });
  });

  it('validates negative price', async () => {
    renderForm();
    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'Test' } });
    fireEvent.change(screen.getByLabelText(/category/i), { target: { value: 'gear' } });
    fireEvent.change(screen.getByLabelText(/^price/i), { target: { value: '-5' } });
    fireEvent.click(screen.getByText('Create'));

    await waitFor(() => {
      expect(screen.getByText('Must be >= 0')).toBeTruthy();
    });
  });
});
