import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box, Button, Chip, IconButton, Paper, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, TablePagination, TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import type { Supplier } from '../../api/suppliers';
import { listSuppliers } from '../../api/suppliers';

export function SupplierListPage() {
  const navigate = useNavigate();
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [search, setSearch] = useState('');

  const fetchData = useCallback(async () => {
    const params: Record<string, string> = { page: String(page + 1), per_page: String(perPage) };
    if (search) params.search = search;
    const res = await listSuppliers(params);
    setSuppliers(res.data ?? []);
    setTotal(res.meta?.total ?? 0);
  }, [page, perPage, search]);

  useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <Box>
      <Toolbar disableGutters sx={{ gap: 2 }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Suppliers</Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => navigate('/suppliers/new')}>
          New Supplier
        </Button>
      </Toolbar>

      <TextField size="small" label="Search" value={search}
        onChange={(e) => { setSearch(e.target.value); setPage(0); }} sx={{ mb: 2, minWidth: 250 }} />

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Contact</TableCell>
              <TableCell>Email</TableCell>
              <TableCell>Phone</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {suppliers.map((s) => (
              <TableRow key={s.id} hover>
                <TableCell>{s.name}</TableCell>
                <TableCell>{s.contact_name ?? '—'}</TableCell>
                <TableCell>{s.email ?? '—'}</TableCell>
                <TableCell>{s.phone ?? '—'}</TableCell>
                <TableCell>
                  <Chip label={s.is_active ? 'Active' : 'Inactive'} size="small"
                    color={s.is_active ? 'success' : 'default'} />
                </TableCell>
                <TableCell align="right">
                  <IconButton size="small" onClick={() => navigate(`/suppliers/${s.id}/edit`)}>
                    <EditIcon fontSize="small" />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
            {suppliers.length === 0 && (
              <TableRow><TableCell colSpan={6} align="center">No suppliers found</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />
    </Box>
  );
}
