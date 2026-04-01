import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Alert, Box, Button, Chip, Divider, Paper, Table, TableBody,
  TableCell, TableContainer, TableHead, TableRow, TextField, Typography,
} from '@mui/material';
import type { PurchaseOrder } from '../../api/purchaseOrders';
import {
  getPurchaseOrder, issuePurchaseOrder, cancelPurchaseOrder, receivePurchaseOrder,
} from '../../api/purchaseOrders';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  draft: 'default', issued: 'info', partially_received: 'warning',
  received: 'success', cancelled: 'error', closed: 'default',
};

export function PODetailPage() {
  const { id } = useParams();
  const [po, setPO] = useState<PurchaseOrder | null>(null);
  const [error, setError] = useState('');
  const [receiveNotes, setReceiveNotes] = useState('');
  const [receiveQtys, setReceiveQtys] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!id) return;
    getPurchaseOrder(id).then(setPO).catch(() => setError('PO not found'));
  }, [id]);

  const reload = async () => {
    if (!id) return;
    setPO(await getPurchaseOrder(id));
  };

  const handleIssue = async () => {
    if (!id) return;
    setError('');
    try { setPO(await issuePurchaseOrder(id)); } catch { setError('Issue failed'); }
  };

  const handleCancel = async () => {
    if (!id || !confirm('Cancel this PO?')) return;
    setError('');
    try { await cancelPurchaseOrder(id); reload(); } catch { setError('Cancel failed'); }
  };

  const handleReceive = async () => {
    if (!id || !po?.lines) return;
    setError('');
    const lines = po.lines
      .filter((l) => {
        const q = parseInt(receiveQtys[l.id] ?? '0', 10);
        return q > 0;
      })
      .map((l) => ({
        po_line_item_id: l.id,
        quantity_received: parseInt(receiveQtys[l.id] ?? '0', 10),
      }));
    if (lines.length === 0) { setError('Enter quantities to receive'); return; }
    try {
      await receivePurchaseOrder(id, { notes: receiveNotes || undefined, lines });
      setReceiveQtys({});
      setReceiveNotes('');
      reload();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Receive failed');
    }
  };

  if (error && !po) return <Alert severity="error">{error}</Alert>;
  if (!po) return <Typography>Loading...</Typography>;

  const canIssue = po.status === 'draft';
  const canCancel = po.status === 'draft' || po.status === 'issued';
  const canReceive = po.status === 'issued' || po.status === 'partially_received';

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>
          PO {po.id.slice(0, 8)}...
        </Typography>
        <Chip label={po.status.replace(/_/g, ' ')} color={statusColor[po.status] ?? 'default'} />
        {canIssue && <Button variant="contained" size="small" onClick={handleIssue}>Issue</Button>}
        {canCancel && <Button variant="outlined" size="small" color="error" onClick={handleCancel}>Cancel</Button>}
      </Box>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Paper sx={{ p: 3, mb: 3 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
          <Typography><strong>Supplier:</strong> {po.supplier_id.slice(0, 8)}...</Typography>
          <Typography><strong>Location:</strong> {po.location_id.slice(0, 8)}...</Typography>
          <Typography><strong>Expected:</strong> {po.expected_at ?? '—'}</Typography>
          <Typography><strong>Issued:</strong> {po.issued_at ? new Date(po.issued_at).toLocaleString() : '—'}</Typography>
          <Typography><strong>Version:</strong> {po.version}</Typography>
        </Box>
        {po.notes && <Typography sx={{ mt: 1 }}><strong>Notes:</strong> {po.notes}</Typography>}
      </Paper>

      {/* Lines */}
      <Typography variant="h6" gutterBottom>Line Items</Typography>
      <TableContainer component={Paper} sx={{ mb: 3 }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Item</TableCell>
              <TableCell align="right">Qty Ordered</TableCell>
              <TableCell align="right">Unit Cost</TableCell>
              <TableCell align="right">Received</TableCell>
              <TableCell align="right">Remaining</TableCell>
              {canReceive && <TableCell align="right">Receive Now</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {(po.lines ?? []).map((l) => (
              <TableRow key={l.id}>
                <TableCell sx={{ fontFamily: 'monospace' }}>{l.item_id.slice(0, 8)}...</TableCell>
                <TableCell align="right">{l.quantity}</TableCell>
                <TableCell align="right">${l.unit_cost.toFixed(2)}</TableCell>
                <TableCell align="right">{l.received_quantity}</TableCell>
                <TableCell align="right">{l.quantity - l.received_quantity}</TableCell>
                {canReceive && (
                  <TableCell align="right">
                    <TextField type="number" size="small" sx={{ width: 80 }}
                      value={receiveQtys[l.id] ?? ''}
                      onChange={(e) => setReceiveQtys((p) => ({ ...p, [l.id]: e.target.value }))}
                      inputProps={{ min: 0, max: l.quantity - l.received_quantity }} />
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {canReceive && (
        <>
          <Divider sx={{ mb: 2 }} />
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <TextField size="small" label="Receipt Notes" value={receiveNotes}
              onChange={(e) => setReceiveNotes(e.target.value)} sx={{ flexGrow: 1 }} />
            <Button variant="contained" onClick={handleReceive}>Record Receipt</Button>
          </Box>
        </>
      )}
    </Box>
  );
}
