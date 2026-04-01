import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Alert, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogTitle, Divider, MenuItem, Paper, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, TextField, Typography,
} from '@mui/material';
import TimelineIcon from '@mui/icons-material/Timeline';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../types/auth';
import type { Order, TimelineEvent, OrderNote } from '../../api/orders';
import {
  getOrder, cancelOrder, adjustOrder, splitOrder,
  changeOrderStatus, getTimeline, addNote, listNotes,
} from '../../api/orders';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  pending: 'default', confirmed: 'info', processing: 'warning',
  fulfilled: 'success', cancelled: 'error', refunded: 'default',
};

const eventTypeIcon: Record<string, string> = {
  creation: 'Created', status_change: 'Status Change', adjustment: 'Adjusted',
  split: 'Split', cancellation: 'Cancelled', note: 'Note Added', refund: 'Refunded',
};

export function OrderDetailPage() {
  const { id } = useParams();
  const { user } = useAuthStore();
  const isStaff = hasRole(user, 'administrator', 'operations_manager');

  const [order, setOrder] = useState<Order | null>(null);
  const [timeline, setTimeline] = useState<TimelineEvent[]>([]);
  const [notes, setNotes] = useState<OrderNote[]>([]);
  const [error, setError] = useState('');
  const [noteText, setNoteText] = useState('');

  // Dialogs
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [adjustLineId, setAdjustLineId] = useState('');
  const [adjustQty, setAdjustQty] = useState('');
  const [adjustReason, setAdjustReason] = useState('');

  const [splitOpen, setSplitOpen] = useState(false);
  const [splitLineId, setSplitLineId] = useState('');
  const [splitQty, setSplitQty] = useState('');
  const [splitReason, setSplitReason] = useState('');

  const [statusOpen, setStatusOpen] = useState(false);
  const [newStatus, setNewStatus] = useState('');
  const [statusReason, setStatusReason] = useState('');

  const load = async () => {
    if (!id) return;
    try {
      setOrder(await getOrder(id));
      setTimeline(await getTimeline(id));
      setNotes(await listNotes(id));
    } catch { setError('Order not found'); }
  };

  useEffect(() => { load(); }, [id]);

  const handleCancel = async () => {
    if (!id || !confirm('Cancel this order?')) return;
    try { await cancelOrder(id, 'Cancelled by staff'); load(); } catch { setError('Cancel failed'); }
  };

  const handleAdjust = async () => {
    if (!id) return;
    try {
      await adjustOrder(id, adjustLineId, parseInt(adjustQty), adjustReason);
      setAdjustOpen(false);
      load();
    } catch (e) { setError(e instanceof Error ? e.message : 'Adjust failed'); }
  };

  const handleSplit = async () => {
    if (!id) return;
    try {
      await splitOrder(id, [{ line_id: splitLineId, quantity: parseInt(splitQty) }], splitReason);
      setSplitOpen(false);
      load();
    } catch (e) { setError(e instanceof Error ? e.message : 'Split failed'); }
  };

  const handleStatusChange = async () => {
    if (!id) return;
    try {
      await changeOrderStatus(id, newStatus, statusReason);
      setStatusOpen(false);
      load();
    } catch (e) { setError(e instanceof Error ? e.message : 'Status change failed'); }
  };

  const handleAddNote = async () => {
    if (!id || !noteText.trim()) return;
    try {
      await addNote(id, noteText);
      setNoteText('');
      load();
    } catch { setError('Failed to add note'); }
  };

  if (error && !order) return <Alert severity="error">{error}</Alert>;
  if (!order) return <Typography>Loading...</Typography>;

  const isTerminal = ['cancelled', 'refunded', 'fulfilled'].includes(order.status);

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>
          Order {order.id.slice(0, 8)}...
        </Typography>
        <Chip label={order.status} color={statusColor[order.status] ?? 'default'} />
        {isStaff && !isTerminal && (
          <>
            <Button size="small" variant="outlined" onClick={() => setStatusOpen(true)}>Change Status</Button>
            <Button size="small" variant="outlined" color="error" onClick={handleCancel}>Cancel</Button>
          </>
        )}
      </Box>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {/* Order details */}
      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
          <Typography><strong>Total:</strong> ${order.total_amount.toFixed(2)}</Typography>
          <Typography><strong>Deposit:</strong> ${order.deposit_amount.toFixed(2)}</Typography>
          <Typography><strong>Member:</strong> {order.member_id.slice(0, 8)}...</Typography>
          <Typography><strong>Version:</strong> {order.version}</Typography>
        </Box>
      </Paper>

      {/* Line items */}
      <Typography variant="h6" gutterBottom>Line Items</Typography>
      <TableContainer component={Paper} sx={{ mb: 3 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Item</TableCell>
              <TableCell align="right">Qty</TableCell>
              <TableCell align="right">Unit Price</TableCell>
              <TableCell align="right">Subtotal</TableCell>
              {isStaff && !isTerminal && <TableCell align="right">Actions</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {(order.lines ?? []).map((l) => (
              <TableRow key={l.id}>
                <TableCell sx={{ fontFamily: 'monospace' }}>{l.item_id.slice(0, 8)}...</TableCell>
                <TableCell align="right">{l.quantity}</TableCell>
                <TableCell align="right">${l.unit_price.toFixed(2)}</TableCell>
                <TableCell align="right">${(l.quantity * l.unit_price).toFixed(2)}</TableCell>
                {isStaff && !isTerminal && (
                  <TableCell align="right">
                    <Button size="small" onClick={() => { setAdjustLineId(l.id); setAdjustQty(String(l.quantity)); setAdjustOpen(true); }}>
                      Adjust
                    </Button>
                    <Button size="small" onClick={() => { setSplitLineId(l.id); setSplitQty('1'); setSplitOpen(true); }}>
                      Split
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Timeline */}
      <Divider sx={{ mb: 2 }} />
      <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <TimelineIcon /> Operation Timeline
      </Typography>
      <Paper sx={{ p: 2, mb: 3 }}>
        {timeline.map((e) => (
          <Box key={e.id} sx={{ mb: 2, pl: 2, borderLeft: 3, borderColor: 'primary.main' }}>
            <Typography variant="subtitle2" color="text.secondary">
              {new Date(e.occurred_at).toLocaleString()} — {eventTypeIcon[e.event_type] ?? e.event_type}
            </Typography>
            <Typography>{e.description}</Typography>
          </Box>
        ))}
        {timeline.length === 0 && <Typography color="text.secondary">No timeline events</Typography>}
      </Paper>

      {/* Notes */}
      <Divider sx={{ mb: 2 }} />
      <Typography variant="h6" gutterBottom>Notes</Typography>
      <Paper sx={{ p: 2, mb: 2 }}>
        {notes.map((n) => (
          <Box key={n.id} sx={{ mb: 1 }}>
            <Typography variant="caption" color="text.secondary">
              {new Date(n.created_at).toLocaleString()} — {n.author_id.slice(0, 8)}
            </Typography>
            <Typography>{n.content}</Typography>
          </Box>
        ))}
        {notes.length === 0 && <Typography color="text.secondary">No notes</Typography>}
      </Paper>
      <Box sx={{ display: 'flex', gap: 1 }}>
        <TextField size="small" fullWidth label="Add a note" value={noteText}
          onChange={(e) => setNoteText(e.target.value)} />
        <Button variant="contained" onClick={handleAddNote} disabled={!noteText.trim()}>Add</Button>
      </Box>

      {/* Adjust Dialog */}
      <Dialog open={adjustOpen} onClose={() => setAdjustOpen(false)}>
        <DialogTitle>Adjust Line Quantity</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          <TextField label="New Quantity" type="number" value={adjustQty}
            onChange={(e) => setAdjustQty(e.target.value)} inputProps={{ min: 1 }} />
          <TextField label="Reason" required value={adjustReason}
            onChange={(e) => setAdjustReason(e.target.value)} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAdjustOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleAdjust}>Apply</Button>
        </DialogActions>
      </Dialog>

      {/* Split Dialog */}
      <Dialog open={splitOpen} onClose={() => setSplitOpen(false)}>
        <DialogTitle>Split Line to New Order</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          <TextField label="Quantity to Split" type="number" value={splitQty}
            onChange={(e) => setSplitQty(e.target.value)} inputProps={{ min: 1 }} />
          <TextField label="Reason" required value={splitReason}
            onChange={(e) => setSplitReason(e.target.value)} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSplitOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSplit}>Split</Button>
        </DialogActions>
      </Dialog>

      {/* Status Change Dialog */}
      <Dialog open={statusOpen} onClose={() => setStatusOpen(false)}>
        <DialogTitle>Change Order Status</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          <TextField select label="New Status" value={newStatus}
            onChange={(e) => setNewStatus(e.target.value)}>
            <MenuItem value="confirmed">Confirmed</MenuItem>
            <MenuItem value="processing">Processing</MenuItem>
            <MenuItem value="fulfilled">Fulfilled</MenuItem>
            <MenuItem value="refunded">Refunded</MenuItem>
          </TextField>
          <TextField label="Reason" required value={statusReason}
            onChange={(e) => setStatusReason(e.target.value)} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setStatusOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleStatusChange}>Apply</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
