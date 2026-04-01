import { useEffect, useState, useCallback } from 'react';
import {
  Alert, Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  TablePagination, TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import { useAuthStore } from '../../store/authStore';
import { hasPermission } from '../../types/auth';
import type { ClassItem, CreateClassPayload } from '../../api/classes';
import { bookClass, cancelClass, cancelClassBooking, createClass, listClasses } from '../../api/classes';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'error'> = {
  scheduled: 'info',
  cancelled: 'error',
  completed: 'success',
};

export function ClassListPage() {
  const { user } = useAuthStore();
  const canWrite = hasPermission(user?.role, 'class:write');

  const [classes, setClasses] = useState<ClassItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [error, setError] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState<CreateClassPayload>({
    coach_id: '', location_id: '', name: '', scheduled_at: '', duration_minutes: 60, capacity: 20,
  });
  const [saving, setSaving] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      const res = await listClasses({ page: String(page + 1), per_page: String(perPage) });
      setClasses(res.data ?? []);
      setTotal(res.meta?.total ?? 0);
    } catch {
      setError('Failed to load classes');
    }
  }, [page, perPage]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleCreate = async () => {
    setSaving(true);
    try {
      await createClass(form);
      setCreateOpen(false);
      setForm({ coach_id: '', location_id: '', name: '', scheduled_at: '', duration_minutes: 60, capacity: 20 });
      fetchData();
    } catch {
      setError('Failed to create class');
    } finally {
      setSaving(false);
    }
  };

  const handleBook = async (id: string) => {
    try {
      await bookClass(id);
      fetchData();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to book class';
      setError(msg);
    }
  };

  const handleCancelBooking = async (id: string) => {
    try {
      await cancelClassBooking(id);
      fetchData();
    } catch {
      setError('Failed to cancel booking');
    }
  };

  const handleCancelClass = async (id: string) => {
    if (!confirm('Cancel this class?')) return;
    try {
      await cancelClass(id);
      fetchData();
    } catch {
      setError('Failed to cancel class');
    }
  };

  return (
    <Box>
      <Toolbar disableGutters>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Classes</Typography>
        {canWrite && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
            New Class
          </Button>
        )}
      </Toolbar>

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Scheduled</TableCell>
              <TableCell align="right">Seats</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {classes.map((cl) => (
              <TableRow key={cl.id} hover>
                <TableCell>{cl.name}</TableCell>
                <TableCell>{new Date(cl.scheduled_at).toLocaleString()}</TableCell>
                <TableCell align="right">{cl.booked_seats} / {cl.capacity}</TableCell>
                <TableCell>
                  <Chip label={cl.status} size="small" color={statusColor[cl.status] ?? 'default'} />
                </TableCell>
                <TableCell>
                  {cl.status === 'scheduled' && user?.role === 'member' && (
                    <>
                      <Button size="small" onClick={() => handleBook(cl.id)}>Book</Button>
                      <Button size="small" color="warning" onClick={() => handleCancelBooking(cl.id)}>Cancel Booking</Button>
                    </>
                  )}
                  {cl.status === 'scheduled' && canWrite && (
                    <Button size="small" color="error" onClick={() => handleCancelClass(cl.id)}>Cancel Class</Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {classes.length === 0 && (
              <TableRow><TableCell colSpan={5} align="center">No classes</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)}
        onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />

      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>New Class</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 2 }}>
          <TextField label="Name" required value={form.name}
            onChange={(e) => setForm(f => ({ ...f, name: e.target.value }))} />
          <TextField label="Coach ID" required value={form.coach_id}
            onChange={(e) => setForm(f => ({ ...f, coach_id: e.target.value }))} />
          <TextField label="Location ID" required value={form.location_id}
            onChange={(e) => setForm(f => ({ ...f, location_id: e.target.value }))} />
          <TextField label="Scheduled At (RFC3339)" required value={form.scheduled_at}
            placeholder="2026-05-01T10:00:00Z"
            onChange={(e) => setForm(f => ({ ...f, scheduled_at: e.target.value }))} />
          <TextField label="Duration (minutes)" type="number" value={form.duration_minutes}
            onChange={(e) => setForm(f => ({ ...f, duration_minutes: +e.target.value }))} />
          <TextField label="Capacity" type="number" value={form.capacity}
            onChange={(e) => setForm(f => ({ ...f, capacity: +e.target.value }))} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreate} disabled={saving}>Create</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
