export type Role =
  | 'administrator'
  | 'operations_manager'
  | 'procurement_specialist'
  | 'coach'
  | 'member';

export interface User {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: Role;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: User;
}

export interface LoginCredentials {
  email: string;
  password: string;
}

// Navigation items filtered by role
export interface NavItem {
  path: string;
  label: string;
  icon: string;
  roles: Role[];
}

// Permissions mirrored from backend — used for fine-grained UI hiding
export type Permission =
  | 'system:config' | 'user:manage'
  | 'catalog:read' | 'catalog:write'
  | 'inventory:read' | 'inventory:adjust'
  | 'supplier:read' | 'supplier:write'
  | 'po:read' | 'po:write' | 'po:receive'
  | 'report:dashboard' | 'report:full' | 'report:coach'
  | 'export:generate'
  | 'class:read' | 'class:write' | 'class:readiness'
  | 'groupbuy:read' | 'groupbuy:create' | 'groupbuy:join' | 'groupbuy:manage'
  | 'order:read' | 'order:own_read' | 'order:adjust' | 'order:note_add' | 'order:timeline'
  | 'member:read' | 'member:browse'
  | 'audit:read';

const ROLE_PERMISSIONS: Record<Role, Permission[]> = {
  administrator: [
    'system:config', 'user:manage',
    'catalog:read', 'catalog:write',
    'inventory:read', 'inventory:adjust',
    'supplier:read', 'supplier:write',
    'po:read', 'po:write', 'po:receive',
    'report:dashboard', 'report:full', 'export:generate',
    'class:read', 'class:write', 'class:readiness',
    'groupbuy:read', 'groupbuy:create', 'groupbuy:manage',
    'order:read', 'order:adjust', 'order:note_add', 'order:timeline',
    'member:read', 'audit:read',
  ],
  operations_manager: [
    'catalog:read', 'catalog:write',
    'inventory:read', 'inventory:adjust',
    'supplier:read', 'po:read',
    'report:dashboard', 'report:full', 'export:generate',
    'class:read',
    'groupbuy:read', 'groupbuy:manage',
    'order:read', 'order:adjust', 'order:note_add', 'order:timeline',
    'member:read',
  ],
  procurement_specialist: [
    'catalog:read',
    'inventory:read', 'inventory:adjust',
    'supplier:read', 'supplier:write',
    'po:read', 'po:write', 'po:receive',
    'report:dashboard', 'export:generate',
  ],
  coach: [
    'catalog:read',
    'class:read', 'class:write', 'class:readiness',
    'report:dashboard', 'report:coach',
  ],
  member: [
    'catalog:read',
    'groupbuy:read', 'groupbuy:create', 'groupbuy:join',
    'order:own_read',
    'member:browse',
  ],
};

export function hasPermission(role: Role | undefined, permission: Permission): boolean {
  if (!role) return false;
  return ROLE_PERMISSIONS[role]?.includes(permission) ?? false;
}

export function hasRole(user: User | null, ...roles: Role[]): boolean {
  if (!user) return false;
  return roles.includes(user.role);
}
