import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Box, Button, Paper, Switch, FormControlLabel, TextField, Typography, Alert } from '@mui/material';
import { createSupplier, getSupplier, updateSupplier } from '../../api/suppliers';

interface FormState {
  name: string;
  contact_name: string;
  email: string;
  phone: string;
  address: string;
  is_active: boolean;
}

const defaults: FormState = {
  name: '', contact_name: '', email: '', phone: '', address: '', is_active: true,
};

export function SupplierFormPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const isEdit = !!id;

  const [form, setForm] = useState<FormState>(defaults);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!id) return;
    getSupplier(id).then((s) =>
      setForm({
        name: s.name,
        contact_name: s.contact_name ?? '',
        email: s.email ?? '',
        phone: s.phone ?? '',
        address: s.address ?? '',
        is_active: s.is_active,
      }),
    );
  }, [id]);

  const set = (field: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((prev) => ({ ...prev, [field]: e.target.value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    setLoading(true);
    setError('');
    try {
      if (isEdit) {
        await updateSupplier(id!, {
          name: form.name, contact_name: form.contact_name || undefined,
          email: form.email || undefined, phone: form.phone || undefined,
          address: form.address || undefined, is_active: form.is_active,
        });
      } else {
        await createSupplier({
          name: form.name, contact_name: form.contact_name || undefined,
          email: form.email || undefined, phone: form.phone || undefined,
          address: form.address || undefined, is_active: form.is_active,
        });
      }
      navigate('/suppliers');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box maxWidth={600}>
      <Typography variant="h5" gutterBottom>{isEdit ? 'Edit Supplier' : 'New Supplier'}</Typography>
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
      <Paper sx={{ p: 3 }} component="form" onSubmit={handleSubmit}>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <TextField label="Name" required value={form.name} onChange={set('name')} />
          <TextField label="Contact Name" value={form.contact_name} onChange={set('contact_name')} />
          <TextField label="Email" value={form.email} onChange={set('email')} />
          <TextField label="Phone" value={form.phone} onChange={set('phone')} />
          <TextField label="Address" multiline rows={2} value={form.address} onChange={set('address')} />
          <FormControlLabel control={
            <Switch checked={form.is_active} onChange={(e) => setForm((p) => ({ ...p, is_active: e.target.checked }))} />
          } label="Active" />
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button variant="outlined" onClick={() => navigate('/suppliers')}>Cancel</Button>
            <Button variant="contained" type="submit" disabled={loading}>
              {loading ? 'Saving...' : isEdit ? 'Update' : 'Create'}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
