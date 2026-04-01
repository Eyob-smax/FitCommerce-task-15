import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

// Mock the sync manager before importing the component
vi.mock('../src/sync/syncManager', () => ({
  syncManager: {
    getStatus: vi.fn(() => 'online'),
    subscribe: vi.fn((fn: (s: string) => void) => {
      fn('online');
      return () => {};
    }),
    start: vi.fn(),
    stop: vi.fn(),
  },
}));

import { SyncStatusIndicator } from '../src/components/SyncStatusIndicator';

describe('SyncStatusIndicator', () => {
  it('renders online status', () => {
    render(<SyncStatusIndicator />);
    expect(screen.getByText('Online')).toBeTruthy();
  });
});
