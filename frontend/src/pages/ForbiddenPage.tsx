import { useNavigate } from 'react-router-dom';
import { Box, Button, Typography } from '@mui/material';
import LockIcon from '@mui/icons-material/Lock';

export function ForbiddenPage() {
  const navigate = useNavigate();
  return (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      minHeight="100vh"
      gap={2}
    >
      <LockIcon sx={{ fontSize: 64, color: 'error.main' }} />
      <Typography variant="h4" fontWeight={700}>
        Access Denied
      </Typography>
      <Typography variant="body1" color="text.secondary">
        You don't have permission to view this page.
      </Typography>
      <Button variant="contained" onClick={() => navigate('/dashboard')}>
        Back to Dashboard
      </Button>
    </Box>
  );
}
