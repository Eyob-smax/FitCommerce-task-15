import { useEffect, useState } from 'react';
import {
  Alert, Box, Button, Card, CardContent, CircularProgress,
  MenuItem, Paper, TextField, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import TrendingUpIcon from '@mui/icons-material/TrendingUp';
import TrendingDownIcon from '@mui/icons-material/TrendingDown';
import PeopleIcon from '@mui/icons-material/People';
import FitnessCenterIcon from '@mui/icons-material/FitnessCenter';
import EventIcon from '@mui/icons-material/Event';
import BarChartIcon from '@mui/icons-material/BarChart';
import DownloadIcon from '@mui/icons-material/Download';
import { useAuthStore } from '../store/authStore';
import { hasPermission } from '../types/auth';
import type { DashboardKPI } from '../api/reports';
import { getDashboard, createExport } from '../api/reports';

export function DashboardPage() {
  const { user } = useAuthStore();
  const canExport = hasPermission(user?.role, 'export:generate');

  const [kpi, setKpi] = useState<DashboardKPI | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [granularity, setGranularity] = useState<string>('monthly');
  const [locationId, setLocationId] = useState('');
  const [coachId, setCoachId] = useState('');
  const [itemCategory, setItemCategory] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');
  const [exporting, setExporting] = useState(false);

  const buildFilters = (): Record<string, string> => {
    const params: Record<string, string> = { granularity };
    if (locationId) params.location_id = locationId;
    if (coachId) params.coach_id = coachId;
    if (itemCategory) params.item_category = itemCategory;
    if (startDate) params.start_date = startDate;
    if (endDate) params.end_date = endDate;
    return params;
  };

  const fetchKPI = async () => {
    setLoading(true);
    setError('');
    try {
      setKpi(await getDashboard(buildFilters()));
    } catch {
      setError('Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchKPI(); }, [granularity, locationId, coachId, itemCategory, startDate, endDate]);

  const handleExport = async (format: 'csv' | 'pdf') => {
    setExporting(true);
    try {
      await createExport('dashboard', format, buildFilters());
      alert(`${format.toUpperCase()} export queued. Check exports page.`);
    } catch {
      alert('Export failed');
    } finally {
      setExporting(false);
    }
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" minHeight={300}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return <Alert severity="error">{error}</Alert>;
  }

  if (!kpi) {
    return <Typography color="text.secondary">No data available</Typography>;
  }

  const kpiCards = [
    { label: 'Member Growth', value: kpi.member_growth, icon: <TrendingUpIcon />, color: '#4caf50' },
    { label: 'Member Churn', value: kpi.member_churn, icon: <TrendingDownIcon />, color: '#f44336' },
    { label: 'Renewal Rate', value: `${kpi.renewal_rate.toFixed(1)}%`, icon: <PeopleIcon />, color: '#2196f3' },
    { label: 'Engagement Events', value: kpi.engagement, icon: <EventIcon />, color: '#ff9800' },
    { label: 'Class Fill Rate', value: `${kpi.class_fill_rate.toFixed(1)}%`, icon: <FitnessCenterIcon />, color: '#9c27b0' },
    { label: 'Coach Productivity', value: kpi.coach_productivity, icon: <BarChartIcon />, color: '#00bcd4' },
  ];

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3, flexWrap: 'wrap' }}>
        <Typography variant="h5" sx={{ flexGrow: 1 }}>Dashboard</Typography>
        <ToggleButtonGroup value={granularity} exclusive
          onChange={(_, v) => { if (v) setGranularity(v); }} size="small">
          <ToggleButton value="daily">Daily</ToggleButton>
          <ToggleButton value="weekly">Weekly</ToggleButton>
          <ToggleButton value="monthly">Monthly</ToggleButton>
        </ToggleButtonGroup>
        <TextField size="small" label="Location ID" value={locationId}
          onChange={(e) => setLocationId(e.target.value)} sx={{ width: 200 }} />
        <TextField size="small" label="Coach ID" value={coachId}
          onChange={(e) => setCoachId(e.target.value)} sx={{ width: 200 }} />
        <TextField size="small" label="Item Category" value={itemCategory}
          onChange={(e) => setItemCategory(e.target.value)} sx={{ width: 180 }} />
        <TextField size="small" type="date" label="Start Date" value={startDate}
          onChange={(e) => setStartDate(e.target.value)} InputLabelProps={{ shrink: true }} sx={{ width: 170 }} />
        <TextField size="small" type="date" label="End Date" value={endDate}
          onChange={(e) => setEndDate(e.target.value)} InputLabelProps={{ shrink: true }} sx={{ width: 170 }} />
        {canExport && (
          <>
            <Button size="small" variant="outlined" startIcon={<DownloadIcon />}
              onClick={() => handleExport('csv')} disabled={exporting}>CSV</Button>
            <Button size="small" variant="outlined" startIcon={<DownloadIcon />}
              onClick={() => handleExport('pdf')} disabled={exporting}>PDF</Button>
          </>
        )}
      </Box>

      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Period: {kpi.start_date} to {kpi.end_date}
      </Typography>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' }, gap: 2, mb: 3 }}>
        {kpiCards.map((card) => (
          <Card key={card.label}>
            <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Box sx={{ color: card.color, fontSize: 40 }}>{card.icon}</Box>
              <Box>
                <Typography variant="h4" fontWeight={700}>{card.value}</Typography>
                <Typography variant="body2" color="text.secondary">{card.label}</Typography>
              </Box>
            </CardContent>
          </Card>
        ))}
      </Box>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h6" gutterBottom>KPI Definitions</Typography>
        <Typography variant="body2"><strong>Member Growth:</strong> Count of new members created in the period.</Typography>
        <Typography variant="body2"><strong>Member Churn:</strong> Count of members who became inactive/cancelled/expired in the period.</Typography>
        <Typography variant="body2"><strong>Renewal Rate:</strong> Percentage of active members whose membership extends past the period end.</Typography>
        <Typography variant="body2"><strong>Engagement:</strong> Total engagement events (attendance, orders, bookings, group-buy joins) in the period.</Typography>
        <Typography variant="body2"><strong>Class Fill Rate:</strong> Average (booked_seats / capacity * 100) across classes in the period.</Typography>
        <Typography variant="body2"><strong>Coach Productivity:</strong> Count of completed classes in the period.</Typography>
      </Paper>
    </Box>
  );
}
