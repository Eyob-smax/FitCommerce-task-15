import { useEffect, useState, useCallback } from 'react';
import {
  Alert, Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  MenuItem, Paper, Table, TableBody, TableCell, TableContainer, TableHead,
  TableRow, TablePagination, TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import type { UserItem, CreateUserPayload } from '../../api/users';
import { createUser, deactivateUser, listUsers, updateUser } from '../../api/users';

const ROLES = ['administrator', 'operations_manager', 'procurement_specialist', 'coach', 'member'];

const roleColor: Record<string, 'default' | 'primary' | 'secondary' | 'warning' | 'info'> = {
  administrator: 'primary',
  operations_manager: 'secondary',
  procurement_specialist: 'warning',
  coach: 'info',
  member: 'default',
};

export function AdminUsersPage() {
  const [users, setUsers] = useState<UserItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [editUser, setEditUser] = useState<UserItem | null>(null);
  const [form, setForm] = useState<CreateUserPayload>({
    email: '', password: '', first_name: '', last_name: '', role: 'member',
  });
  const [editForm, setEditForm] = useState({ first_name: '', last_name: '', role: '', is_active: true });
  const [saving, setSaving] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const res = await listUsers({ page: String(page + 1), per_page: String(perPage) });
      setUsers(res.data ?? []);
      setTotal(res.meta?.total ?? 0);
    } catch {
      setError('Failed to load users');
    }
  }, [page, perPage]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleCreate = async () => {
    setSaving(true);
    try {
      await createUser(form);
      setCreateOpen(false);
      setForm({ email: '', password: '', first_name: '', last_name: '', role: 'member' });
      fetchData();
    } catch {
      setError('Failed to create user');
    } finally {
      setSaving(false);
    }
  };

  const openEdit = (u: UserItem) => {
    setEditUser(u);
    setEditForm({ first_name: u.first_name, last_name: u.last_name, role: u.role, is_active: u.is_active });
  };

  const handleUpdate = async () => {
    if (!editUser) return;
    setSaving(true);
    try {
      await updateUser(editUser.id, editForm);
      setEditUser(null);
      fetchData();
    } catch {
      setError('Failed to update user');
    } finally {
      setSaving(false);
    }
  };

  const handleDeactivate = async (id: string) => {
    if (!confirm('Deactivate this user?')) return;
    try {
      await deactivateUser(id);
      fetchData();
    } catch {
      setError('Failed to deactivate user');
    }
  };

  return (
    <Box>
      <Toolbar disableGutters>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>User Management</Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
          New User
        </Button>
      </Toolbar>

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Role</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id} hover>
                <TableCell>{u.first_name} {u.last_name}</TableCell>
                <TableCell>{u.email}</TableCell>
                <TableCell>
                  <Chip label={u.role.replace('_', ' ')} size="small" color={roleColor[u.role] ?? 'default'} />
                </TableCell>
                <TableCell>
                  <Chip label={u.is_active ? 'Active' : 'Inactive'} size="small"
                    color={u.is_active ? 'success' : 'default'} />
                </TableCell>
                <TableCell>{new Date(u.created_at).toLocaleDateString()}</TableCell>
                <TableCell>
                  <Button size="small" onClick={() => openEdit(u)}>Edit</Button>
                  {u.is_active && (
                    <Button size="small" color="error" onClick={() => handleDeactivate(u.id)}>Deactivate</Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {users.length === 0 && (
              <TableRow><TableCell colSpan={6} align="center">No users</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)}
        onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />

      {/* Create user dialog */}
      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>New User</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 2 }}>
          <TextField label="Email" required type="email" value={form.email}
            onChange={(e) => setForm(f => ({ ...f, email: e.target.value }))} />
          <TextField label="Password" required type="password" value={form.password}
            onChange={(e) => setForm(f => ({ ...f, password: e.target.value }))} />
          <TextField label="First Name" required value={form.first_name}
            onChange={(e) => setForm(f => ({ ...f, first_name: e.target.value }))} />
          <TextField label="Last Name" required value={form.last_name}
            onChange={(e) => setForm(f => ({ ...f, last_name: e.target.value }))} />
          <TextField select label="Role" value={form.role}
            onChange={(e) => setForm(f => ({ ...f, role: e.target.value }))}>
            {ROLES.map((r) => <MenuItem key={r} value={r}>{r.replace(/_/g, ' ')}</MenuItem>)}
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreate} disabled={saving}>Create</Button>
        </DialogActions>
      </Dialog>

      {/* Edit user dialog */}
      <Dialog open={!!editUser} onClose={() => setEditUser(null)} maxWidth="sm" fullWidth>
        <DialogTitle>Edit User</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 2 }}>
          <TextField label="First Name" value={editForm.first_name}
            onChange={(e) => setEditForm(f => ({ ...f, first_name: e.target.value }))} />
          <TextField label="Last Name" value={editForm.last_name}
            onChange={(e) => setEditForm(f => ({ ...f, last_name: e.target.value }))} />
          <TextField select label="Role" value={editForm.role}
            onChange={(e) => setEditForm(f => ({ ...f, role: e.target.value }))}>
            {ROLES.map((r) => <MenuItem key={r} value={r}>{r.replace(/_/g, ' ')}</MenuItem>)}
          </TextField>
          <TextField select label="Status" value={editForm.is_active ? 'active' : 'inactive'}
            onChange={(e) => setEditForm(f => ({ ...f, is_active: e.target.value === 'active' }))}>
            <MenuItem value="active">Active</MenuItem>
            <MenuItem value="inactive">Inactive</MenuItem>
          </TextField>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditUser(null)}>Cancel</Button>
          <Button variant="contained" onClick={handleUpdate} disabled={saving}>Save</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
