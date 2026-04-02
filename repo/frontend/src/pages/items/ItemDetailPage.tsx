import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Box, Button, Chip, Divider, IconButton, Paper, Table, TableBody,
  TableCell, TableContainer, TableHead, TableRow, TextField, Typography, Alert,
} from '@mui/material';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../types/auth';
import type { Item, AvailabilityWindow } from '../../api/items';
import {
  getItem, publishItem, unpublishItem,
  listAvailabilityWindows, addAvailabilityWindow, deleteAvailabilityWindow,
} from '../../api/items';
import { listGroupBuys } from '../../api/groupBuys';
import { getStock } from '../../api/inventory';

export function ItemDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const canEdit = hasRole(user, 'administrator', 'operations_manager');
  const isMember = hasRole(user, 'member');

  const [item, setItem] = useState<Item | null>(null);
  const [windows, setWindows] = useState<AvailabilityWindow[]>([]);
  const [joinGroupBuyId, setJoinGroupBuyId] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [availableQty, setAvailableQty] = useState<number | null>(null);
  const [startsAt, setStartsAt] = useState('');
  const [endsAt, setEndsAt] = useState('');
  const [windowError, setWindowError] = useState('');

  useEffect(() => {
    if (!id) return;
    getItem(id).then(setItem).catch(() => setError('Item not found'));
    listAvailabilityWindows(id).then(setWindows);

    if (isMember) {
      (async () => {
        try {
          const active = await listGroupBuys({ item_id: id, status: 'active', per_page: '1' });
          const published = await listGroupBuys({ item_id: id, status: 'published', per_page: '1' });
          const candidate = active.data?.[0] ?? published.data?.[0] ?? null;
          setJoinGroupBuyId(candidate?.id ?? null);
        } catch {
          setJoinGroupBuyId(null);
        }
      })();
    }

    if (canEdit) {
      getStock(id)
        .then((stock) => setAvailableQty(stock.available))
        .catch(() => setAvailableQty(null));
    } else {
      setAvailableQty(null);
    }
  }, [id, isMember, canEdit]);

  const handlePublish = async () => {
    if (!id) return;
    await publishItem(id);
    const refreshed = await getItem(id);
    setItem(refreshed);
  };

  const handleUnpublish = async () => {
    if (!id) return;
    await unpublishItem(id);
    const refreshed = await getItem(id);
    setItem(refreshed);
  };

  const handleAddWindow = async () => {
    if (!id) return;
    setWindowError('');
    if (!startsAt || !endsAt) { setWindowError('Both dates required'); return; }
    const s = new Date(startsAt).toISOString();
    const e = new Date(endsAt).toISOString();
    if (e <= s) { setWindowError('End must be after start'); return; }
    try {
      const w = await addAvailabilityWindow(id, s, e);
      setWindows((prev) => [...prev, w]);
      setStartsAt('');
      setEndsAt('');
    } catch {
      setWindowError('Failed to add window');
    }
  };

  const handleDeleteWindow = async (wid: string) => {
    if (!id) return;
    await deleteAvailabilityWindow(id, wid);
    setWindows((prev) => prev.filter((w) => w.id !== wid));
  };

  if (error) return <Alert severity="error">{error}</Alert>;
  if (!item) return <Typography>Loading...</Typography>;

  const statusColor: Record<string, 'default' | 'success' | 'warning'> = {
    draft: 'warning', published: 'success', unpublished: 'default',
  };

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>{item.name}</Typography>
        <Chip label={item.status} color={statusColor[item.status] ?? 'default'} />
        {canEdit && (
          <>
            {(item.status === 'draft' || item.status === 'unpublished') && (
              <Button variant="contained" size="small" onClick={handlePublish}>Publish</Button>
            )}
            {item.status === 'published' && (
              <Button variant="outlined" size="small" onClick={handleUnpublish}>Unpublish</Button>
            )}
            <Button variant="outlined" startIcon={<EditIcon />} size="small"
              onClick={() => navigate(`/items/${id}/edit`)}>Edit</Button>
          </>
        )}
        {isMember && (
          <>
            <Button variant="contained" size="small" onClick={() => navigate(`/group-buys/new?item_id=${id}`)}>
              Start Group Buy
            </Button>
            {joinGroupBuyId && (
              <Button variant="outlined" size="small" onClick={() => navigate(`/group-buys/${joinGroupBuyId}`)}>
                Join Active Group Buy
              </Button>
            )}
          </>
        )}
      </Box>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
          <Typography><strong>SKU:</strong> {item.sku ?? '—'}</Typography>
          <Typography><strong>Category:</strong> {item.category}</Typography>
          <Typography><strong>Brand:</strong> {item.brand ?? '—'}</Typography>
          <Typography><strong>Condition:</strong> {item.condition}</Typography>
          <Typography><strong>Price:</strong> ${item.price.toFixed(2)}</Typography>
          <Typography><strong>Deposit:</strong> ${item.deposit_amount.toFixed(2)}</Typography>
          <Typography><strong>Billing:</strong> {item.billing_model}</Typography>
          {canEdit && <Typography><strong>Available Quantity:</strong> {availableQty ?? '—'}</Typography>}
          <Typography><strong>Version:</strong> {item.version}</Typography>
        </Box>
        {item.description && (
          <Typography sx={{ mt: 2 }}><strong>Description:</strong> {item.description}</Typography>
        )}
        {item.images && item.images.length > 0 && (
          <Typography sx={{ mt: 1 }}><strong>Images:</strong> {item.images.join(', ')}</Typography>
        )}
      </Paper>

      {/* Availability Windows */}
      <Typography variant="h6" gutterBottom>Availability Windows</Typography>
      <TableContainer component={Paper} sx={{ mb: 2 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Start</TableCell>
              <TableCell>End</TableCell>
              {canEdit && <TableCell align="right">Actions</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {windows.map((w) => (
              <TableRow key={w.id}>
                <TableCell>{new Date(w.starts_at).toLocaleString()}</TableCell>
                <TableCell>{new Date(w.ends_at).toLocaleString()}</TableCell>
                {canEdit && (
                  <TableCell align="right">
                    <IconButton size="small" color="error" onClick={() => handleDeleteWindow(w.id)}>
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {windows.length === 0 && (
              <TableRow><TableCell colSpan={canEdit ? 3 : 2} align="center">No windows</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {canEdit && (
        <>
          <Divider sx={{ mb: 2 }} />
          {windowError && <Alert severity="error" sx={{ mb: 1 }}>{windowError}</Alert>}
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <TextField type="datetime-local" label="Start" value={startsAt}
              onChange={(e) => setStartsAt(e.target.value)} InputLabelProps={{ shrink: true }} size="small" />
            <TextField type="datetime-local" label="End" value={endsAt}
              onChange={(e) => setEndsAt(e.target.value)} InputLabelProps={{ shrink: true }} size="small" />
            <Button variant="contained" size="small" onClick={handleAddWindow}>Add Window</Button>
          </Box>
        </>
      )}
    </Box>
  );
}
