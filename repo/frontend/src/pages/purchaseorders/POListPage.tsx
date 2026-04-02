import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box, Button, Chip, MenuItem, Paper, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, TablePagination, TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import type { PurchaseOrder } from '../../api/purchaseOrders';
import { listPurchaseOrders } from '../../api/purchaseOrders';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  draft: 'default',
  issued: 'info',
  partially_received: 'warning',
  received: 'success',
  cancelled: 'error',
  closed: 'default',
};

export function POListPage() {
  const navigate = useNavigate();
  const [orders, setOrders] = useState<PurchaseOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [status, setStatus] = useState('');

  const fetchData = useCallback(async () => {
    const params: Record<string, string> = { page: String(page + 1), per_page: String(perPage) };
    if (status) params.status = status;
    const res = await listPurchaseOrders(params);
    setOrders(res.data ?? []);
    setTotal(res.meta?.total ?? 0);
  }, [page, perPage, status]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <Box>
      <Toolbar disableGutters sx={{ gap: 2, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Purchase Orders</Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => navigate('/purchase-orders/new')}>
          New PO
        </Button>
      </Toolbar>

      <TextField size="small" select label="Status" value={status}
        onChange={(e) => { setStatus(e.target.value); setPage(0); }} sx={{ mb: 2, minWidth: 180 }}>
        <MenuItem value="">All</MenuItem>
        <MenuItem value="draft">Draft</MenuItem>
        <MenuItem value="issued">Issued</MenuItem>
        <MenuItem value="partially_received">Partially Received</MenuItem>
        <MenuItem value="received">Received</MenuItem>
        <MenuItem value="cancelled">Cancelled</MenuItem>
      </TextField>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>PO ID</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Expected</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Version</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {orders.map((po) => (
              <TableRow key={po.id} hover sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/purchase-orders/${po.id}`)}>
                <TableCell sx={{ fontFamily: 'monospace' }}>{po.id.slice(0, 8)}...</TableCell>
                <TableCell>
                  <Chip label={po.status.replace(/_/g, ' ')} size="small"
                    color={statusColor[po.status] ?? 'default'} />
                </TableCell>
                <TableCell>{po.expected_at ?? '—'}</TableCell>
                <TableCell>{new Date(po.created_at).toLocaleDateString()}</TableCell>
                <TableCell>{po.version}</TableCell>
              </TableRow>
            ))}
            {orders.length === 0 && (
              <TableRow><TableCell colSpan={5} align="center">No purchase orders</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />
    </Box>
  );
}
