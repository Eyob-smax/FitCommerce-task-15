import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Alert, Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, MenuItem, Paper, Table, TableBody, TableCell, TableContainer,
  TableHead, TableRow, TablePagination, TextField, Toolbar, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import PublishIcon from '@mui/icons-material/Publish';
import UnpublishedIcon from '@mui/icons-material/Unpublished';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../types/auth';
import type { Item } from '../../api/items';
import { listItems, deleteItem, publishItem, unpublishItem, batchUpdateItems } from '../../api/items';
import { listInventory } from '../../api/inventory';

const statusColor: Record<string, 'default' | 'success' | 'warning'> = {
  draft: 'warning',
  published: 'success',
  unpublished: 'default',
};

export function ItemListPage() {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const canEdit = hasRole(user, 'administrator', 'operations_manager');

  const [items, setItems] = useState<Item[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [perPage, setPerPage] = useState(25);
  const [category, setCategory] = useState('');
  const [status, setStatus] = useState('');
  const [search, setSearch] = useState('');
  const [availableByItem, setAvailableByItem] = useState<Record<string, number>>({});

  // Batch
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [batchOpen, setBatchOpen] = useState(false);
  const [batchPrice, setBatchPrice] = useState('');
  const [batchStart, setBatchStart] = useState('');
  const [batchEnd, setBatchEnd] = useState('');
  const [batchError, setBatchError] = useState('');

  const fetchItems = useCallback(async () => {
    const params: Record<string, string> = {
      page: String(page + 1),
      per_page: String(perPage),
    };
    if (category) params.category = category;
    if (status) params.status = status;
    if (search) params.search = search;
    const res = await listItems(params);
    setItems(res.data ?? []);
    setTotal(res.meta?.total ?? 0);

    if (!canEdit) {
      setAvailableByItem({});
      return;
    }

    const inv = await listInventory({ page: '1', per_page: '500' });
    const next: Record<string, number> = {};
    for (const record of inv.data ?? []) {
      next[record.item_id] = record.available;
    }
    setAvailableByItem(next);
  }, [page, perPage, category, status, search, canEdit]);

  useEffect(() => { fetchItems(); }, [fetchItems]);

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this item?')) return;
    await deleteItem(id);
    fetchItems();
  };

  const handlePublish = async (id: string) => {
    await publishItem(id);
    fetchItems();
  };

  const handleUnpublish = async (id: string) => {
    await unpublishItem(id);
    fetchItems();
  };

  const handleBatch = async () => {
    if (selected.size === 0) return;
    setBatchError('');

    const hasPrice = !!batchPrice;
    const hasWindow = !!batchStart || !!batchEnd;
    if (!hasPrice && !hasWindow) {
      setBatchError('Provide a new price or an availability window.');
      return;
    }
    if ((batchStart && !batchEnd) || (!batchStart && batchEnd)) {
      setBatchError('Availability start and end must both be provided.');
      return;
    }

    const payload: { item_ids: string[]; price?: number; availability_windows?: Array<{ starts_at: string; ends_at: string }> } = {
      item_ids: Array.from(selected),
    };
    if (hasPrice) payload.price = Number(batchPrice);
    if (batchStart && batchEnd) {
      const startsAt = new Date(batchStart).toISOString();
      const endsAt = new Date(batchEnd).toISOString();
      if (endsAt <= startsAt) {
        setBatchError('Availability end must be after start.');
        return;
      }
      payload.availability_windows = [{ starts_at: startsAt, ends_at: endsAt }];
    }

    await batchUpdateItems(payload);
    setBatchOpen(false);
    setBatchPrice('');
    setBatchStart('');
    setBatchEnd('');
    setBatchError('');
    setSelected(new Set());
    fetchItems();
  };

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  return (
    <Box>
      <Toolbar disableGutters sx={{ gap: 2, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Catalog</Typography>
        {canEdit && selected.size > 0 && (
          <Button variant="outlined" onClick={() => setBatchOpen(true)}>
            Batch Edit ({selected.size})
          </Button>
        )}
        {canEdit && (
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => navigate('/items/new')}>
            New Item
          </Button>
        )}
      </Toolbar>

      <Box sx={{ display: 'flex', gap: 2, mb: 2, flexWrap: 'wrap' }}>
        <TextField size="small" label="Search" value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(0); }} sx={{ minWidth: 200 }} />
        <TextField size="small" label="Category" value={category}
          onChange={(e) => { setCategory(e.target.value); setPage(0); }} sx={{ minWidth: 150 }} />
        {canEdit && (
          <TextField size="small" select label="Status" value={status}
            onChange={(e) => { setStatus(e.target.value); setPage(0); }} sx={{ minWidth: 140 }}>
            <MenuItem value="">All</MenuItem>
            <MenuItem value="draft">Draft</MenuItem>
            <MenuItem value="published">Published</MenuItem>
            <MenuItem value="unpublished">Unpublished</MenuItem>
          </TextField>
        )}
      </Box>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              {canEdit && <TableCell padding="checkbox" />}
              <TableCell>Name</TableCell>
              <TableCell>SKU</TableCell>
              <TableCell>Category</TableCell>
              <TableCell>Price</TableCell>
              <TableCell align="right">Available</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Condition</TableCell>
              {canEdit && <TableCell align="right">Actions</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.id} hover sx={{ cursor: 'pointer' }}
                onClick={() => navigate(`/items/${item.id}`)}>
                {canEdit && (
                  <TableCell padding="checkbox" onClick={(e) => e.stopPropagation()}>
                    <input type="checkbox" checked={selected.has(item.id)}
                      onChange={() => toggleSelect(item.id)} />
                  </TableCell>
                )}
                <TableCell>{item.name}</TableCell>
                <TableCell>{item.sku ?? '—'}</TableCell>
                <TableCell>{item.category}</TableCell>
                <TableCell>${item.price.toFixed(2)}</TableCell>
                <TableCell align="right">{availableByItem[item.id] ?? '—'}</TableCell>
                <TableCell><Chip label={item.status} size="small" color={statusColor[item.status] ?? 'default'} /></TableCell>
                <TableCell>{item.condition}</TableCell>
                {canEdit && (
                  <TableCell align="right" onClick={(e) => e.stopPropagation()}>
                    <IconButton size="small" onClick={() => navigate(`/items/${item.id}/edit`)}><EditIcon fontSize="small" /></IconButton>
                    {item.status === 'draft' || item.status === 'unpublished' ? (
                      <IconButton size="small" onClick={() => handlePublish(item.id)}><PublishIcon fontSize="small" /></IconButton>
                    ) : item.status === 'published' ? (
                      <IconButton size="small" onClick={() => handleUnpublish(item.id)}><UnpublishedIcon fontSize="small" /></IconButton>
                    ) : null}
                    <IconButton size="small" color="error" onClick={() => handleDelete(item.id)}><DeleteIcon fontSize="small" /></IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {items.length === 0 && (
              <TableRow><TableCell colSpan={canEdit ? 9 : 7} align="center">No items found</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination component="div" count={total} page={page} rowsPerPage={perPage}
        onPageChange={(_, p) => setPage(p)} onRowsPerPageChange={(e) => { setPerPage(+e.target.value); setPage(0); }} />

      {/* Batch edit dialog */}
      <Dialog open={batchOpen} onClose={() => setBatchOpen(false)}>
        <DialogTitle>Batch Update Price / Availability</DialogTitle>
        <DialogContent>
          {batchError && <Alert severity="error" sx={{ mt: 1 }}>{batchError}</Alert>}
          <TextField fullWidth label="New Price" type="number" value={batchPrice}
            onChange={(e) => setBatchPrice(e.target.value)} sx={{ mt: 1 }}
            inputProps={{ min: 0, step: '0.01' }} />
          <TextField fullWidth type="datetime-local" label="Availability Start" value={batchStart}
            onChange={(e) => setBatchStart(e.target.value)} sx={{ mt: 2 }} InputLabelProps={{ shrink: true }} />
          <TextField fullWidth type="datetime-local" label="Availability End" value={batchEnd}
            onChange={(e) => setBatchEnd(e.target.value)} sx={{ mt: 2 }} InputLabelProps={{ shrink: true }} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBatchOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleBatch} disabled={!batchPrice && !(batchStart && batchEnd)}>Apply</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
