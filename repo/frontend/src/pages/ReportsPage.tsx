import { useEffect, useState, useCallback } from 'react';
import {
  Alert, Box, Button, Chip, CircularProgress, Paper, Table, TableBody, TableCell,
  TableContainer, TableHead, TableRow, Toolbar, Typography,
} from '@mui/material';
import DownloadIcon from '@mui/icons-material/Download';
import type { ExportJob } from '../api/reports';
import { createExport, downloadExport, listExports } from '../api/reports';

const statusColor: Record<string, 'default' | 'info' | 'success' | 'error'> = {
  queued: 'default', processing: 'info', completed: 'success', failed: 'error',
};

export function ReportsPage() {
  const [exports, setExports] = useState<ExportJob[]>([]);
  const [creating, setCreating] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string>('');
  const [error, setError] = useState('');

  const fetchExports = useCallback(async () => {
    const res = await listExports({ per_page: '50' });
    setExports(res.data ?? []);
  }, []);

  useEffect(() => { fetchExports(); }, [fetchExports]);

  // Poll for processing jobs
  useEffect(() => {
    const hasProcessing = exports.some((e) => e.status === 'queued' || e.status === 'processing');
    if (!hasProcessing) return;
    const interval = setInterval(fetchExports, 3000);
    return () => clearInterval(interval);
  }, [exports, fetchExports]);

  const handleCreate = async (reportType: string, format: string) => {
    setCreating(true);
    setError('');
    try {
      await createExport(reportType, format);
      fetchExports();
    } catch {
      setError('Could not queue export job. Please try again.');
    }
    finally { setCreating(false); }
  };

  const handleDownload = async (id: string) => {
    setError('');
    setDownloadingId(id);
    try {
      const { blob, filename } = await downloadExport(id);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);
    } catch {
      setError('Download failed. Verify your session and try again.');
    } finally {
      setDownloadingId('');
    }
  };

  return (
    <Box>
      <Toolbar disableGutters>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Reports & Exports</Typography>
      </Toolbar>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Box sx={{ display: 'flex', gap: 1, mb: 3, flexWrap: 'wrap' }}>
        {['inventory', 'orders', 'member-growth', 'group-buys'].map((type) => (
          <Box key={type} sx={{ display: 'flex', gap: 0.5 }}>
            <Button size="small" variant="outlined" disabled={creating}
              onClick={() => handleCreate(type, 'csv')} startIcon={<DownloadIcon />}>
              {type} CSV
            </Button>
            <Button size="small" variant="outlined" disabled={creating}
              onClick={() => handleCreate(type, 'pdf')} startIcon={<DownloadIcon />}>
              PDF
            </Button>
          </Box>
        ))}
      </Box>

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>Report</TableCell>
              <TableCell>Format</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Created</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {exports.map((e) => (
              <TableRow key={e.id}>
                <TableCell>{e.report_type}</TableCell>
                <TableCell>{e.format.toUpperCase()}</TableCell>
                <TableCell>
                  <Chip label={e.status} size="small" color={statusColor[e.status] ?? 'default'} />
                </TableCell>
                <TableCell>{new Date(e.created_at).toLocaleString()}</TableCell>
                <TableCell align="right">
                  {e.status === 'completed' && (
                    <Button
                      size="small"
                      onClick={() => { void handleDownload(e.id); }}
                      disabled={downloadingId === e.id}
                      startIcon={downloadingId === e.id ? <CircularProgress size={14} /> : <DownloadIcon fontSize="small" />}
                    >
                      {downloadingId === e.id ? 'Downloading...' : 'Download'}
                    </Button>
                  )}
                  {e.error_msg && (
                    <Typography variant="caption" color="error">{e.error_msg}</Typography>
                  )}
                </TableCell>
              </TableRow>
            ))}
            {exports.length === 0 && (
              <TableRow><TableCell colSpan={5} align="center">No exports yet</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
