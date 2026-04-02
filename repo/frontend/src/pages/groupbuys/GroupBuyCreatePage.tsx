import { useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Alert, Box, Button, Paper, TextField, Typography } from '@mui/material';
import { createGroupBuy } from '../../api/groupBuys';

export function GroupBuyCreatePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const itemId = searchParams.get('item_id') ?? '';

  const [form, setForm] = useState({
    item_id: itemId,
    location_id: '',
    title: '',
    description: '',
    min_quantity: '10',
    cutoff_at: '',
    price_per_unit: '',
    notes: '',
  });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const set = (field: string) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((prev) => ({ ...prev, [field]: e.target.value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.item_id || !form.location_id || !form.title || !form.cutoff_at || !form.price_per_unit) {
      setError('Please fill all required fields');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const cutoff = new Date(form.cutoff_at).toISOString();
      await createGroupBuy({
        item_id: form.item_id,
        location_id: form.location_id,
        title: form.title,
        description: form.description || undefined,
        min_quantity: parseInt(form.min_quantity, 10),
        cutoff_at: cutoff,
        price_per_unit: parseFloat(form.price_per_unit),
        notes: form.notes || undefined,
      });
      navigate('/group-buys');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Creation failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box maxWidth={600}>
      <Typography variant="h5" gutterBottom>Start a Group Buy</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Paper sx={{ p: 3 }} component="form" onSubmit={handleSubmit}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <TextField label="Item ID" required value={form.item_id} onChange={set('item_id')}
            helperText="UUID of the item for this group buy" />
          <TextField label="Location ID" required value={form.location_id} onChange={set('location_id')} />
          <TextField label="Title" required value={form.title} onChange={set('title')} />
          <TextField label="Description" multiline rows={2} value={form.description} onChange={set('description')} />
          <Box sx={{ display: 'flex', gap: 2 }}>
            <TextField label="Min Quantity" type="number" required value={form.min_quantity}
              onChange={set('min_quantity')} inputProps={{ min: 1 }} sx={{ flex: 1 }} />
            <TextField label="Price per Unit" type="number" required value={form.price_per_unit}
              onChange={set('price_per_unit')} inputProps={{ min: 0, step: '0.01' }} sx={{ flex: 1 }} />
          </Box>
          <TextField label="Cutoff Date/Time" type="datetime-local" required value={form.cutoff_at}
            onChange={set('cutoff_at')} InputLabelProps={{ shrink: true }} />
          <TextField label="Notes" multiline rows={2} value={form.notes} onChange={set('notes')} />
          <Box sx={{ display: 'flex', gap: 2, mt: 1 }}>
            <Button variant="outlined" onClick={() => navigate('/group-buys')}>Cancel</Button>
            <Button variant="contained" type="submit" disabled={loading}>
              {loading ? 'Creating...' : 'Create Group Buy'}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
