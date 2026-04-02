import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box, Chip, MenuItem, Paper, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, TablePagination,
  TextField, Toolbar, Typography,
} from '@mui/material';
import type { Order } from '../../api/orders';
import { listOrders } from '../../api/orders';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  pending: 'default', confirmed: 'info', processing: 'warning',
  fulfilled: 'success', cancelled: 'error', refunded: 'default',
};

export function OrderListPage() {
  const navigate = useNavigate();
  const [orders, setOrders] = useState<Order[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [status, setStatus] = useState('');

  const fetchData = useCallback(async () => {
    const params: Record<string, string> = { page: String(page + 1), per_page: String(perPage) };
    if (status) params.status = status;
    const res = await listOrders(params);
    setOrders(res.data ?? []);
    setTotal(res.meta?.total ?? 0);
  }, [page, perPage, status]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <Box>
      <Toolbar disableGutters>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Orders</Typography>
      </Toolbar>

      <TextField size="small" select label="Status" value={status}
        onChange={(e) => { setStatus(e.target.value); setPage(0); }} sx={{ mb: 2, minWidth: 160 }}>
        <MenuItem value="">All</MenuItem>
        <MenuItem value="pending">Pending</MenuItem>
        <MenuItem value="confirmed">Confirmed</MenuItem>
        <MenuItem value="processing">Processing</MenuItem>
        <MenuItem value="fulfilled">Fulfilled</MenuItem>
        <MenuItem value="cancelled">Cancelled</MenuItem>
        <MenuItem value="refunded">Refunded</MenuItem>
      </TextField>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Order ID</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Total</TableCell>
              <TableCell>Created</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {orders.map((o) => (
              <TableRow key={o.id} hover sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/orders/${o.id}`)}>
                <TableCell sx={{ fontFamily: 'monospace' }}>{o.id.slice(0, 8)}...</TableCell>
                <TableCell>
                  <Chip label={o.status} size="small" color={statusColor[o.status] ?? 'default'} />
                </TableCell>
                <TableCell align="right">${o.total_amount.toFixed(2)}</TableCell>
                <TableCell>{new Date(o.created_at).toLocaleString()}</TableCell>
              </TableRow>
            ))}
            {orders.length === 0 && (
              <TableRow><TableCell colSpan={4} align="center">No orders</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />
    </Box>
  );
}
