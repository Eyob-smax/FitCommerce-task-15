import { Navigate, useLocation } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { useAuthStore } from '../../store/authStore';
import type { Role } from '../../types/auth';

interface Props {
  children: React.ReactNode;
  /** If provided, only users with one of these roles may access the route */
  roles?: Role[];
}

export function ProtectedRoute({ children, roles }: Props) {
  const { isAuthenticated, isLoading, user } = useAuthStore();
  const location = useLocation();

  if (isLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" minHeight="100vh">
        <CircularProgress />
      </Box>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }

  if (roles && user && !roles.includes(user.role)) {
    return <Navigate to="/forbidden" replace />;
  }

  return <>{children}</>;
}

/** Inline guard that hides UI sections without a full redirect */
export function RoleGuard({ children, roles }: Props) {
  const { user } = useAuthStore();
  if (!user || (roles && !roles.includes(user.role))) return null;
  return <>{children}</>;
}
