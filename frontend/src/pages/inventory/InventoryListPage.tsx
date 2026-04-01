import { useEffect, useState, useCallback } from 'react';
import {
  Box, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  MenuItem, Paper, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, TablePagination, TextField, Toolbar, Typography, Alert,
} from '@mui/material';
import { useAuthStore } from '../../store/authStore';
import { hasPermission } from '../../types/auth';
import type { StockRecord } from '../../api/inventory';
import { listInventory, adjustStock } from '../../api/inventory';

const REASON_CODES = [
  'damaged', 'found', 'correction', 'return', 'theft', 'audit', 'expired', 'other',
] as const;

export function InventoryListPage() {
  const { user } = useAuthStore();
  const canAdjust = hasPermission(user?.role, 'inventory:adjust');

  const [records, setRecords] = useState<StockRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);

  // Adjust dialog
  const [adjustTarget, setAdjustTarget] = useState<StockRecord | null>(null);
  const [qty, setQty] = useState('');
  const [reason, setReason] = useState<string>('correction');
  const [notes, setNotes] = useState('');
  const [adjustError, setAdjustError] = useState('');

  const fetchData = useCallback(async () => {
    const res = await listInventory({ page: String(page + 1), per_page: String(perPage) });
    setRecords(res.data ?? []);
    setTotal(res.meta?.total ?? 0);
  }, [page, perPage]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleAdjust = async () => {
    setAdjustError('');
    const change = parseInt(qty, 10);
    if (isNaN(change) || change === 0) { setAdjustError('Quantity must be non-zero'); return; }
    if (!adjustTarget) return;
    try {
      await adjustStock(adjustTarget.item_id, { quantity_change: change, reason_code: reason, notes: notes || undefined });
      setAdjustTarget(null);
      setQty('');
      setNotes('');
      fetchData();
    } catch (err) {
      setAdjustError(err instanceof Error ? err.message : 'Adjustment failed');
    }
  };

  return (
    <Box>
      <Toolbar disableGutters>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Inventory</Typography>
      </Toolbar>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Item</TableCell>
              <TableCell align="right">On Hand</TableCell>
              <TableCell align="right">Reserved</TableCell>
              <TableCell align="right">Allocated</TableCell>
              <TableCell align="right">Available</TableCell>
              <TableCell align="right">In Rental</TableCell>
              <TableCell align="right">Damaged</TableCell>
              {canAdjust && <TableCell align="right">Actions</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {records.map((r) => (
              <TableRow key={r.id}>
                <TableCell>{r.item_name}</TableCell>
                <TableCell align="right">{r.on_hand}</TableCell>
                <TableCell align="right">{r.reserved}</TableCell>
                <TableCell align="right">{r.allocated}</TableCell>
                <TableCell align="right">{r.available}</TableCell>
                <TableCell align="right">{r.in_rental}</TableCell>
                <TableCell align="right">{r.damaged}</TableCell>
                {canAdjust && (
                  <TableCell align="right">
                    <Button size="small" onClick={() => setAdjustTarget(r)}>Adjust</Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {records.length === 0 && (
              <TableRow><TableCell colSpan={canAdjust ? 8 : 7} align="center">No inventory records</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />

      {/* Adjust dialog */}
      <Dialog open={!!adjustTarget} onClose={() => setAdjustTarget(null)} maxWidth="sm" fullWidth>
        <DialogTitle>Adjust Stock — {adjustTarget?.item_name}</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          {adjustError && <Alert severity="error">{adjustError}</Alert>}
          <Typography variant="body2">Current on-hand: <strong>{adjustTarget?.on_hand}</strong></Typography>
          <TextField label="Quantity Change" type="number" value={qty} onChange={(e) => setQty(e.target.value)}
            helperText="Positive to add, negative to subtract" fullWidth />
          <TextField select label="Reason Code" value={reason} onChange={(e) => setReason(e.target.value)} fullWidth>
            {REASON_CODES.map((r) => <MenuItem key={r} value={r}>{r}</MenuItem>)}
          </TextField>
          <TextField label="Notes (optional)" value={notes} onChange={(e) => setNotes(e.target.value)} multiline rows={2} fullWidth />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAdjustTarget(null)}>Cancel</Button>
          <Button variant="contained" onClick={handleAdjust}>Apply</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
