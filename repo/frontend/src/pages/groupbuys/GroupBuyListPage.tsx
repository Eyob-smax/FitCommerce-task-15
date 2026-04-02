import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box, Button, Chip, LinearProgress, MenuItem, Paper, Table, TableBody,
  TableCell, TableContainer, TableHead, TableRow, TablePagination,
  TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../types/auth';
import type { GroupBuy } from '../../api/groupBuys';
import { listGroupBuys } from '../../api/groupBuys';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  draft: 'default', published: 'info', active: 'warning',
  succeeded: 'success', failed: 'error', cancelled: 'default', fulfilled: 'success',
};

export function GroupBuyListPage() {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const canCreate = hasRole(user, 'administrator', 'operations_manager', 'member');

  const [gbs, setGbs] = useState<GroupBuy[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [status, setStatus] = useState('');

  const fetchData = useCallback(async () => {
    const params: Record<string, string> = { page: String(page + 1), per_page: String(perPage) };
    if (status) params.status = status;
    const res = await listGroupBuys(params);
    setGbs(res.data ?? []);
    setTotal(res.meta?.total ?? 0);
  }, [page, perPage, status]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const isPast = (cutoff: string) => new Date(cutoff) < new Date();

  return (
    <Box>
      <Toolbar disableGutters sx={{ gap: 2, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Group Buys</Typography>
        {canCreate && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => navigate('/group-buys/new')}>
            Start Group Buy
          </Button>
        )}
      </Toolbar>

      <TextField size="small" select label="Status" value={status}
        onChange={(e) => { setStatus(e.target.value); setPage(0); }} sx={{ mb: 2, minWidth: 160 }}>
        <MenuItem value="">All</MenuItem>
        <MenuItem value="published">Published</MenuItem>
        <MenuItem value="active">Active</MenuItem>
        <MenuItem value="succeeded">Succeeded</MenuItem>
        <MenuItem value="failed">Failed</MenuItem>
        <MenuItem value="cancelled">Cancelled</MenuItem>
      </TextField>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Title</TableCell>
              <TableCell>Price</TableCell>
              <TableCell>Progress</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Cutoff</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {gbs.map((gb) => (
              <TableRow key={gb.id} hover sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/group-buys/${gb.id}`)}>
                <TableCell>{gb.title}</TableCell>
                <TableCell>${gb.price_per_unit.toFixed(2)}</TableCell>
                <TableCell sx={{ minWidth: 160 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <LinearProgress variant="determinate" value={gb.progress}
                      sx={{ flexGrow: 1, height: 8, borderRadius: 4 }}
                      color={gb.progress >= 100 ? 'success' : 'primary'} />
                    <Typography variant="caption" noWrap>
                      {gb.current_quantity}/{gb.min_quantity}
                    </Typography>
                  </Box>
                </TableCell>
                <TableCell>
                  <Chip label={gb.status} size="small" color={statusColor[gb.status] ?? 'default'} />
                </TableCell>
                <TableCell sx={{ color: isPast(gb.cutoff_at) ? 'error.main' : 'text.primary' }}>
                  {new Date(gb.cutoff_at).toLocaleString()}
                </TableCell>
              </TableRow>
            ))}
            {gbs.length === 0 && (
              <TableRow><TableCell colSpan={5} align="center">No group buys found</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />
    </Box>
  );
}
