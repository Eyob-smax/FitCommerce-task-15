import type { Role, NavItem } from '../types/auth';

export const ALL_ROLES: Role[] = [
  'administrator',
  'operations_manager',
  'procurement_specialist',
  'coach',
  'member',
];

export const STAFF_ROLES: Role[] = [
  'administrator',
  'operations_manager',
  'procurement_specialist',
  'coach',
];

export const NAV_ITEMS: NavItem[] = [
  {
    path: '/dashboard',
    label: 'Dashboard',
    icon: 'Dashboard',
    roles: ALL_ROLES,
  },
  {
    path: '/items',
    label: 'Catalog',
    icon: 'Inventory2',
    roles: ['administrator', 'operations_manager'],
  },
  {
    path: '/inventory',
    label: 'Inventory',
    icon: 'Warehouse',
    roles: ['administrator', 'operations_manager', 'procurement_specialist'],
  },
  {
    path: '/suppliers',
    label: 'Suppliers',
    icon: 'LocalShipping',
    roles: ['administrator', 'procurement_specialist'],
  },
  {
    path: '/purchase-orders',
    label: 'Purchase Orders',
    icon: 'Receipt',
    roles: ['administrator', 'procurement_specialist'],
  },
  {
    path: '/group-buys',
    label: 'Group Buys',
    icon: 'Groups',
    roles: ['administrator', 'operations_manager', 'member'],
  },
  {
    path: '/orders',
    label: 'Orders',
    icon: 'ShoppingCart',
    roles: ['administrator', 'operations_manager', 'member'],
  },
  {
    path: '/reports',
    label: 'Reports',
    icon: 'BarChart',
    roles: ['administrator', 'operations_manager', 'coach'],
  },
  {
    path: '/classes',
    label: 'Classes',
    icon: 'FitnessCenter',
    roles: ['administrator', 'operations_manager', 'coach'],
  },
  {
    path: '/admin',
    label: 'Admin',
    icon: 'Settings',
    roles: ['administrator'],
  },
];
