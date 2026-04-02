import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Alert, Box, Button, Chip, Divider, LinearProgress, Paper,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  TextField, Typography,
} from '@mui/material';
import { useAuthStore } from '../../store/authStore';
import { hasRole } from '../../types/auth';
import type { GroupBuy, Participant } from '../../api/groupBuys';
import {
  getGroupBuy, joinGroupBuy, leaveGroupBuy, cancelGroupBuy,
  publishGroupBuy, getParticipants,
} from '../../api/groupBuys';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'warning' | 'error'> = {
  draft: 'default', published: 'info', active: 'warning',
  succeeded: 'success', failed: 'error', cancelled: 'default', fulfilled: 'success',
};

const outcomeMessages: Record<string, { text: string; severity: 'success' | 'error' | 'info' }> = {
  succeeded: { text: 'This group buy succeeded! The minimum quantity was reached.', severity: 'success' },
  failed: { text: 'This group buy did not reach the minimum quantity by the cutoff time.', severity: 'error' },
  cancelled: { text: 'This group buy was cancelled by an administrator.', severity: 'info' },
  fulfilled: { text: 'This group buy has been fulfilled. Orders are being processed.', severity: 'success' },
};

export function GroupBuyDetailPage() {
  const { id } = useParams();
  const { user } = useAuthStore();
  const isMember = hasRole(user, 'member');
  const isStaff = hasRole(user, 'administrator', 'operations_manager');

  const [gb, setGB] = useState<GroupBuy | null>(null);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [error, setError] = useState('');
  const [joinQty, setJoinQty] = useState('1');
  const [actionLoading, setActionLoading] = useState(false);

  const load = async () => {
    if (!id) return;
    try {
      const data = await getGroupBuy(id);
      setGB(data);
      const parts = await getParticipants(id);
      setParticipants(parts);
    } catch {
      setError('Group buy not found');
    }
  };

  useEffect(() => { load(); }, [id]);

  // Auto-refresh every 10s for live progress
  useEffect(() => {
    if (!gb || gb.status === 'succeeded' || gb.status === 'failed' || gb.status === 'cancelled') return;
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, [gb?.status]);

  const handleJoin = async () => {
    if (!id) return;
    setError('');
    setActionLoading(true);
    try {
      await joinGroupBuy(id, parseInt(joinQty, 10) || 1);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Join failed');
    } finally {
      setActionLoading(false);
    }
  };

  const handleLeave = async () => {
    if (!id) return;
    setError('');
    setActionLoading(true);
    try {
      await leaveGroupBuy(id);
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Leave failed');
    } finally {
      setActionLoading(false);
    }
  };

  const handlePublish = async () => {
    if (!id) return;
    try { setGB(await publishGroupBuy(id)); } catch { setError('Publish failed'); }
  };

  const handleCancel = async () => {
    if (!id || !confirm('Cancel this group buy?')) return;
    try { await cancelGroupBuy(id); load(); } catch { setError('Cancel failed'); }
  };

  if (error && !gb) return <Alert severity="error">{error}</Alert>;
  if (!gb) return <Typography>Loading...</Typography>;

  const isPast = new Date(gb.cutoff_at) < new Date();
  const canJoin = (gb.status === 'published' || gb.status === 'active') && !isPast && isMember;
  const outcome = outcomeMessages[gb.status];

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>{gb.title}</Typography>
        <Chip label={gb.status} color={statusColor[gb.status] ?? 'default'} />
        {isStaff && gb.status === 'draft' && (
          <Button variant="contained" size="small" onClick={handlePublish}>Publish</Button>
        )}
        {isStaff && (gb.status === 'draft' || gb.status === 'published' || gb.status === 'active') && (
          <Button variant="outlined" size="small" color="error" onClick={handleCancel}>Cancel</Button>
        )}
      </Box>

      {/* Outcome message */}
      {outcome && <Alert severity={outcome.severity} sx={{ mb: 2 }}>{outcome.text}</Alert>}
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {/* Progress */}
      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="h6" gutterBottom>Progress</Typography>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
          <LinearProgress variant="determinate" value={gb.progress}
            sx={{ flexGrow: 1, height: 12, borderRadius: 6 }}
            color={gb.progress >= 100 ? 'success' : 'primary'} />
          <Typography variant="h6">
            {gb.current_quantity} / {gb.min_quantity}
          </Typography>
        </Box>
        <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
          <Typography><strong>Price per unit:</strong> ${gb.price_per_unit.toFixed(2)}</Typography>
          <Typography><strong>Cutoff:</strong> {new Date(gb.cutoff_at).toLocaleString()}</Typography>
          <Typography><strong>Min quantity:</strong> {gb.min_quantity}</Typography>
          <Typography sx={{ color: isPast ? 'error.main' : 'success.main' }}>
            <strong>{isPast ? 'Cutoff passed' : 'Open for joining'}</strong>
          </Typography>
        </Box>
        {gb.description && <Typography sx={{ mt: 2 }}>{gb.description}</Typography>}
      </Paper>

      {/* Join / Leave */}
      {canJoin && (
        <Paper sx={{ p: 3, mb: 3 }}>
          <Typography variant="h6" gutterBottom>Join this Group Buy</Typography>
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <TextField label="Quantity" type="number" size="small" value={joinQty}
              onChange={(e) => setJoinQty(e.target.value)} inputProps={{ min: 1 }} sx={{ width: 100 }} />
            <Button variant="contained" onClick={handleJoin} disabled={actionLoading}>
              {actionLoading ? 'Joining...' : 'Join'}
            </Button>
            <Button variant="outlined" onClick={handleLeave} disabled={actionLoading}>Leave</Button>
          </Box>
        </Paper>
      )}

      {/* Participants */}
      <Divider sx={{ mb: 2 }} />
      <Typography variant="h6" gutterBottom>
        Participants ({participants.filter((p) => p.status === 'committed').length})
      </Typography>
      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Member</TableCell>
              <TableCell align="right">Quantity</TableCell>
              <TableCell>Joined</TableCell>
              <TableCell>Status</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {participants.map((p) => (
              <TableRow key={p.id}>
                <TableCell sx={{ fontFamily: 'monospace' }}>{p.member_id.slice(0, 8)}...</TableCell>
                <TableCell align="right">{p.quantity}</TableCell>
                <TableCell>{new Date(p.joined_at).toLocaleString()}</TableCell>
                <TableCell>
                  <Chip label={p.status} size="small"
                    color={p.status === 'committed' ? 'success' : 'default'} />
                </TableCell>
              </TableRow>
            ))}
            {participants.length === 0 && (
              <TableRow><TableCell colSpan={4} align="center">No participants yet</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
