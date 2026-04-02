import { useEffect, useState } from 'react';
import { Chip } from '@mui/material';
import CloudDoneIcon from '@mui/icons-material/CloudDone';
import CloudOffIcon from '@mui/icons-material/CloudOff';
import SyncIcon from '@mui/icons-material/Sync';
import { syncManager } from '../sync/syncManager';
import type { SyncStatus } from '../sync/types';

const statusConfig: Record<SyncStatus, { label: string; color: 'success' | 'default' | 'warning' | 'error'; icon: React.ReactElement }> = {
  online: { label: 'Online', color: 'success', icon: <CloudDoneIcon fontSize="small" /> },
  offline: { label: 'Offline', color: 'default', icon: <CloudOffIcon fontSize="small" /> },
  syncing: { label: 'Syncing...', color: 'warning', icon: <SyncIcon fontSize="small" /> },
  error: { label: 'Sync Error', color: 'error', icon: <CloudOffIcon fontSize="small" /> },
};

export function SyncStatusIndicator() {
  const [status, setStatus] = useState<SyncStatus>(syncManager.getStatus());

  useEffect(() => {
    const unsubscribe = syncManager.subscribe(setStatus);
    return unsubscribe;
  }, []);

  const config = statusConfig[status];

  return (
    <Chip
      icon={config.icon}
      label={config.label}
      size="small"
      color={config.color}
      variant="outlined"
      sx={{ fontSize: '0.75rem' }}
    />
  );
}
