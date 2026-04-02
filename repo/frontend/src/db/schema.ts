import Dexie, { type Table } from 'dexie';
import type { QueuedMutation, SyncMeta, SyncConflict } from '../sync/types';

// ── Cached entity shapes (subset of server DTOs) ─────────────────────────────

export interface CachedItem {
  id: string;
  name: string;
  description?: string;
  category: string;
  brand?: string;
  condition: 'new' | 'open-box' | 'used';
  billing_model: 'one-time' | 'monthly-rental';
  deposit_amount: number;
  price: number;
  status: 'draft' | 'published' | 'unpublished';
  location_id?: string;
  version: number;
  updated_at: string;
  cached_at: number; // unix ms
}

export interface CachedGroupBuy {
  id: string;
  item_id: string;
  item_name: string;
  title: string;
  description?: string;
  min_quantity: number;
  current_quantity: number;
  status: string;
  cutoff_at: string;
  price_per_unit: number;
  location_id: string;
  version: number;
  updated_at: string;
  cached_at: number;
}

export interface CachedOrder {
  id: string;
  member_id: string;
  status: string;
  total_amount: number;
  deposit_amount: number;
  group_buy_id?: string;
  version: number;
  updated_at: string;
  cached_at: number;
}

export interface CachedMember {
  id: string;
  user_id: string;
  location_id?: string;
  membership_type: string;
  status: string;
  version: number;
  updated_at: string;
  cached_at: number;
}

// ── Database class ────────────────────────────────────────────────────────────

class FitCommerceDB extends Dexie {
  items!: Table<CachedItem, string>;
  groupBuys!: Table<CachedGroupBuy, string>;
  orders!: Table<CachedOrder, string>;
  members!: Table<CachedMember, string>;

  // Sync primitives
  mutations!: Table<QueuedMutation, number>;
  syncMeta!: Table<SyncMeta, string>;
  conflicts!: Table<SyncConflict, number>;

  constructor() {
    super('fitcommerce_v1');

    this.version(1).stores({
      // Cached entities — primary key first, then indexed fields
      items:      'id, category, status, location_id, updated_at, cached_at',
      groupBuys:  'id, status, item_id, location_id, cutoff_at, cached_at',
      orders:     'id, member_id, status, group_buy_id, cached_at',
      members:    'id, user_id, location_id, status, cached_at',

      // Mutation queue — auto-increment PK (seq), indexed for drain order
      mutations:  '++seq, idempotency_key, entity_type, status, created_at',

      // One record per entity type — tracks last successful pull timestamp
      syncMeta:   'entity_type',

      // Conflict records awaiting user resolution
      conflicts:  '++seq, mutation_seq, entity_type, detected_at',
    });
  }
}

export const db = new FitCommerceDB();
