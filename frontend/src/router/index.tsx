import { createBrowserRouter, Navigate } from 'react-router-dom';
import { ProtectedRoute } from '../components/auth/ProtectedRoute';
import { AppLayout } from '../layouts/AppLayout';
import { LoginPage } from '../pages/LoginPage';
import { ForbiddenPage } from '../pages/ForbiddenPage';
import { ItemListPage } from '../pages/items/ItemListPage';
import { ItemFormPage } from '../pages/items/ItemFormPage';
import { ItemDetailPage } from '../pages/items/ItemDetailPage';
import { InventoryListPage } from '../pages/inventory/InventoryListPage';
import { SupplierListPage } from '../pages/suppliers/SupplierListPage';
import { SupplierFormPage } from '../pages/suppliers/SupplierFormPage';
import { POListPage } from '../pages/purchaseorders/POListPage';
import { POFormPage } from '../pages/purchaseorders/POFormPage';
import { PODetailPage } from '../pages/purchaseorders/PODetailPage';
import { GroupBuyListPage } from '../pages/groupbuys/GroupBuyListPage';
import { GroupBuyDetailPage } from '../pages/groupbuys/GroupBuyDetailPage';
import { GroupBuyCreatePage } from '../pages/groupbuys/GroupBuyCreatePage';
import { OrderListPage } from '../pages/orders/OrderListPage';
import { OrderDetailPage } from '../pages/orders/OrderDetailPage';
import { DashboardPage } from '../pages/DashboardPage';
import { ReportsPage } from '../pages/ReportsPage';
import { ClassListPage } from '../pages/classes/ClassListPage';
import { AdminUsersPage } from '../pages/admin/AdminUsersPage';

export const router = createBrowserRouter([
  // ── Public routes ──────────────────────────────────────────────────────────
  { path: '/login', element: <LoginPage /> },
  { path: '/forbidden', element: <ForbiddenPage /> },

  // ── Protected shell ────────────────────────────────────────────────────────
  {
    path: '/',
    element: (
      <ProtectedRoute>
        <AppLayout />
      </ProtectedRoute>
    ),
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },

      { path: 'dashboard', element: <DashboardPage /> },

      // ── Catalog (items) ──────────────────────────────────────────────────
      {
        path: 'items',
        element: <ItemListPage />,
      },
      {
        path: 'items/new',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager']}>
            <ItemFormPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'items/:id',
        element: <ItemDetailPage />,
      },
      {
        path: 'items/:id/edit',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager']}>
            <ItemFormPage />
          </ProtectedRoute>
        ),
      },

      // ── Inventory ────────────────────────────────────────────────────────
      {
        path: 'inventory',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'procurement_specialist']}>
            <InventoryListPage />
          </ProtectedRoute>
        ),
      },

      // ── Suppliers ────────────────────────────────────────────────────────
      {
        path: 'suppliers',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <SupplierListPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'suppliers/new',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <SupplierFormPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'suppliers/:id/edit',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <SupplierFormPage />
          </ProtectedRoute>
        ),
      },

      // ── Purchase Orders ──────────────────────────────────────────────────
      {
        path: 'purchase-orders',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <POListPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'purchase-orders/new',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <POFormPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'purchase-orders/:id',
        element: (
          <ProtectedRoute roles={['administrator', 'procurement_specialist']}>
            <PODetailPage />
          </ProtectedRoute>
        ),
      },

      // ── Group Buys ─────────────────────────────────────────────────────
      {
        path: 'group-buys',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'member']}>
            <GroupBuyListPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'group-buys/new',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'member']}>
            <GroupBuyCreatePage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'group-buys/:id',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'member']}>
            <GroupBuyDetailPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'orders',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'member']}>
            <OrderListPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'orders/:id',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'member']}>
            <OrderDetailPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'reports',
        element: (
          <ProtectedRoute roles={['administrator', 'operations_manager', 'procurement_specialist', 'coach']}>
            <ReportsPage />
          </ProtectedRoute>
        ),
      },
      {
        path: 'classes',
        element: <ClassListPage />,
      },
      {
        path: 'admin',
        element: (
          <ProtectedRoute roles={['administrator']}>
            <AdminUsersPage />
          </ProtectedRoute>
        ),
      },
    ],
  },

  // Catch-all
  { path: '*', element: <Navigate to="/dashboard" replace /> },
]);
