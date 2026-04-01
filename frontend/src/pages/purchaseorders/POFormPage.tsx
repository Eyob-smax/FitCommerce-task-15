import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Alert, Box, Button, IconButton, Paper, TextField, Typography,
} from '@mui/material';
import DeleteIcon from '@mui/icons-material/Delete';
import AddIcon from '@mui/icons-material/Add';
import { createPurchaseOrder } from '../../api/purchaseOrders';
import type { CreatePOLinePayload } from '../../api/purchaseOrders';
import type { Supplier } from '../../api/suppliers';
import { listSuppliers } from '../../api/suppliers';

interface LineState {
  item_id: string;
  quantity: string;
  unit_cost: string;
}

export function POFormPage() {
  const navigate = useNavigate();
  const [supplierID, setSupplierID] = useState('');
  const [locationID, setLocationID] = useState('');
  const [notes, setNotes] = useState('');
  const [expectedAt, setExpectedAt] = useState('');
  const [lines, setLines] = useState<LineState[]>([{ item_id: '', quantity: '1', unit_cost: '' }]);
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    listSuppliers({ per_page: '100' }).then((res) => setSuppliers(res.data ?? []));
  }, []);

  const addLine = () => setLines((p) => [...p, { item_id: '', quantity: '1', unit_cost: '' }]);
  const removeLine = (i: number) => setLines((p) => p.filter((_, idx) => idx !== i));
  const updateLine = (i: number, field: keyof LineState, value: string) =>
    setLines((p) => p.map((l, idx) => (idx === i ? { ...l, [field]: value } : l)));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!supplierID || !locationID || lines.length === 0) {
      setError('Supplier, location, and at least one line are required');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const linePayloads: CreatePOLinePayload[] = lines.map((l) => ({
        item_id: l.item_id,
        quantity: parseInt(l.quantity, 10),
        unit_cost: parseFloat(l.unit_cost),
      }));
      await createPurchaseOrder({
        supplier_id: supplierID, location_id: locationID,
        notes: notes || undefined,
        expected_at: expectedAt || undefined,
        lines: linePayloads,
      });
      navigate('/purchase-orders');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create PO');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box maxWidth={800}>
      <Typography variant="h5" gutterBottom>New Purchase Order</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Paper sx={{ p: 3 }} component="form" onSubmit={handleSubmit}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <TextField select label="Supplier" required value={supplierID}
            onChange={(e) => setSupplierID(e.target.value)}
            SelectProps={{ native: true }}>
            <option value="">Select supplier</option>
            {suppliers.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </TextField>
          <TextField label="Location ID" required value={locationID}
            onChange={(e) => setLocationID(e.target.value)}
            helperText="UUID of the receiving location" />
          <TextField label="Expected Arrival" type="date" value={expectedAt}
            onChange={(e) => setExpectedAt(e.target.value)} InputLabelProps={{ shrink: true }} />
          <TextField label="Notes" multiline rows={2} value={notes}
            onChange={(e) => setNotes(e.target.value)} />

          <Typography variant="subtitle1" sx={{ mt: 1 }}>Line Items</Typography>
          {lines.map((line, i) => (
            <Box key={i} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              <TextField label="Item ID" size="small" required value={line.item_id}
                onChange={(e) => updateLine(i, 'item_id', e.target.value)} sx={{ flex: 2 }} />
              <TextField label="Qty" size="small" type="number" required value={line.quantity}
                onChange={(e) => updateLine(i, 'quantity', e.target.value)} sx={{ width: 80 }}
                inputProps={{ min: 1 }} />
              <TextField label="Unit Cost" size="small" type="number" required value={line.unit_cost}
                onChange={(e) => updateLine(i, 'unit_cost', e.target.value)} sx={{ width: 120 }}
                inputProps={{ min: 0, step: '0.01' }} />
              <IconButton size="small" color="error" onClick={() => removeLine(i)}
                disabled={lines.length === 1}>
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Box>
          ))}
          <Button startIcon={<AddIcon />} onClick={addLine} size="small" sx={{ alignSelf: 'flex-start' }}>
            Add Line
          </Button>

          <Box sx={{ display: 'flex', gap: 2, mt: 2 }}>
            <Button variant="outlined" onClick={() => navigate('/purchase-orders')}>Cancel</Button>
            <Button variant="contained" type="submit" disabled={loading}>
              {loading ? 'Creating...' : 'Create PO'}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
