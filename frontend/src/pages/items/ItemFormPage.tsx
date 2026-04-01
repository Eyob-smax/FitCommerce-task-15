import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  Box, Button, MenuItem, Paper, TextField, Typography, Alert,
} from '@mui/material';
import { createItem, getItem, updateItem } from '../../api/items';
import type { CreateItemPayload, UpdateItemPayload } from '../../api/items';

const CONDITIONS = ['new', 'open-box', 'used'] as const;
const BILLING_MODELS = ['one-time', 'monthly-rental'] as const;

interface FormState {
  name: string;
  sku: string;
  category: string;
  brand: string;
  condition: string;
  description: string;
  images: string;
  deposit_amount: string;
  billing_model: string;
  price: string;
  location_id: string;
}

const defaults: FormState = {
  name: '', sku: '', category: '', brand: '', condition: 'new',
  description: '', images: '', deposit_amount: '50.00', billing_model: 'one-time',
  price: '', location_id: '',
};

export function ItemFormPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [form, setForm] = useState<FormState>(defaults);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!id) return;
    getItem(id).then((item) => {
      setForm({
        name: item.name,
        sku: item.sku ?? '',
        category: item.category,
        brand: item.brand ?? '',
        condition: item.condition,
        description: item.description ?? '',
        images: (item.images ?? []).join(', '),
        deposit_amount: String(item.deposit_amount),
        billing_model: item.billing_model,
        price: String(item.price),
        location_id: item.location_id ?? '',
      });
    });
  }, [id]);

  const set = (field: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, [field]: e.target.value }));
    setErrors((prev) => { const next = { ...prev }; delete next[field]; return next; });
  };

  const validate = (): boolean => {
    const errs: Record<string, string> = {};
    if (!form.name.trim()) errs.name = 'Required';
    if (!form.category.trim()) errs.category = 'Required';
    const price = parseFloat(form.price);
    if (isNaN(price) || price < 0) errs.price = 'Must be >= 0';
    const deposit = parseFloat(form.deposit_amount);
    if (isNaN(deposit) || deposit < 0) errs.deposit_amount = 'Must be >= 0';
    setErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setLoading(true);
    setServerError('');

    const images = form.images.split(',').map((s) => s.trim()).filter(Boolean);

    try {
      if (isEdit) {
        const payload: UpdateItemPayload = {
          name: form.name, sku: form.sku || undefined, category: form.category,
          brand: form.brand || undefined, condition: form.condition,
          description: form.description || undefined, images,
          deposit_amount: parseFloat(form.deposit_amount),
          billing_model: form.billing_model, price: parseFloat(form.price),
        };
        await updateItem(id!, payload);
      } else {
        const payload: CreateItemPayload = {
          name: form.name, sku: form.sku || undefined, category: form.category,
          brand: form.brand || undefined, condition: form.condition,
          description: form.description || undefined, images,
          deposit_amount: parseFloat(form.deposit_amount),
          billing_model: form.billing_model, price: parseFloat(form.price),
          location_id: form.location_id || undefined,
        };
        await createItem(payload);
      }
      navigate('/items');
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box maxWidth={700}>
      <Typography variant="h5" gutterBottom>{isEdit ? 'Edit Item' : 'New Item'}</Typography>
      {serverError && <Alert severity="error" sx={{ mb: 2 }}>{serverError}</Alert>}

      <Paper sx={{ p: 3 }} component="form" onSubmit={handleSubmit}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <TextField label="Name" required value={form.name} onChange={set('name')}
            error={!!errors.name} helperText={errors.name} />
          <TextField label="SKU" value={form.sku} onChange={set('sku')} />
          <TextField label="Category" required value={form.category} onChange={set('category')}
            error={!!errors.category} helperText={errors.category} />
          <TextField label="Brand" value={form.brand} onChange={set('brand')} />
          <TextField select label="Condition" value={form.condition} onChange={set('condition')}>
            {CONDITIONS.map((c) => <MenuItem key={c} value={c}>{c}</MenuItem>)}
          </TextField>
          <TextField label="Description" multiline rows={3} value={form.description} onChange={set('description')} />
          <TextField label="Image URLs (comma separated)" value={form.images} onChange={set('images')} />
          <Box sx={{ display: 'flex', gap: 2 }}>
            <TextField label="Price" required type="number" value={form.price} onChange={set('price')}
              error={!!errors.price} helperText={errors.price}
              inputProps={{ min: 0, step: '0.01' }} sx={{ flex: 1 }} />
            <TextField label="Deposit" type="number" value={form.deposit_amount} onChange={set('deposit_amount')}
              error={!!errors.deposit_amount} helperText={errors.deposit_amount}
              inputProps={{ min: 0, step: '0.01' }} sx={{ flex: 1 }} />
          </Box>
          <TextField select label="Billing Model" value={form.billing_model} onChange={set('billing_model')}>
            {BILLING_MODELS.map((b) => <MenuItem key={b} value={b}>{b}</MenuItem>)}
          </TextField>

          <Box sx={{ display: 'flex', gap: 2, mt: 1 }}>
            <Button variant="outlined" onClick={() => navigate('/items')}>Cancel</Button>
            <Button variant="contained" type="submit" disabled={loading}>
              {loading ? 'Saving...' : isEdit ? 'Update' : 'Create'}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
