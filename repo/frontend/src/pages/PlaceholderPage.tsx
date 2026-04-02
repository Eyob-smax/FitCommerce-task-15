import { Box, Typography, Chip } from '@mui/material';
import BuildIcon from '@mui/icons-material/Build';

interface Props {
  title: string;
}

export function PlaceholderPage({ title }: Props) {
  return (
    <Box
      display="flex"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      minHeight="60vh"
      gap={2}
    >
      <BuildIcon sx={{ fontSize: 48, color: 'primary.main', opacity: 0.5 }} />
      <Typography variant="h5" fontWeight={600}>
        {title}
      </Typography>
      <Chip label="Coming in next prompt" color="primary" variant="outlined" size="small" />
    </Box>
  );
}
