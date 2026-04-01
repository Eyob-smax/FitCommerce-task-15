import { useEffect } from 'react';
import { RouterProvider } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { router } from './router';
import { useAuthStore } from './store/authStore';
import { syncManager } from './sync/syncManager';

export default function App() {
  const { restoreSession, isLoading, isAuthenticated } = useAuthStore();

  useEffect(() => {
    restoreSession();
  }, [restoreSession]);

  // Start sync manager when authenticated
  useEffect(() => {
    if (isAuthenticated) {
      syncManager.start();
      return () => syncManager.stop();
    }
  }, [isAuthenticated]);

  if (isLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" minHeight="100vh">
        <CircularProgress />
      </Box>
    );
  }

  return <RouterProvider router={router} />;
}
