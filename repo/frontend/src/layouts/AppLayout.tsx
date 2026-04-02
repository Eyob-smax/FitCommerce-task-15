import { useState } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  AppBar,
  Box,
  Chip,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
} from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import LogoutIcon from '@mui/icons-material/Logout';
import DashboardIcon from '@mui/icons-material/Dashboard';
import Inventory2Icon from '@mui/icons-material/Inventory2';
import WarehouseIcon from '@mui/icons-material/Warehouse';
import LocalShippingIcon from '@mui/icons-material/LocalShipping';
import ReceiptIcon from '@mui/icons-material/Receipt';
import GroupsIcon from '@mui/icons-material/Groups';
import ShoppingCartIcon from '@mui/icons-material/ShoppingCart';
import BarChartIcon from '@mui/icons-material/BarChart';
import FitnessCenterIcon from '@mui/icons-material/FitnessCenter';
import SettingsIcon from '@mui/icons-material/Settings';

import { useAuthStore } from '../store/authStore';
import { NAV_ITEMS } from '../router/routes';
import { SyncStatusIndicator } from '../components/SyncStatusIndicator';
import type { Role } from '../types/auth';

const DRAWER_WIDTH = 260;

const ICON_MAP: Record<string, React.ReactElement> = {
  Dashboard: <DashboardIcon />,
  Inventory2: <Inventory2Icon />,
  Warehouse: <WarehouseIcon />,
  LocalShipping: <LocalShippingIcon />,
  Receipt: <ReceiptIcon />,
  Groups: <GroupsIcon />,
  ShoppingCart: <ShoppingCartIcon />,
  BarChart: <BarChartIcon />,
  FitnessCenter: <FitnessCenterIcon />,
  Settings: <SettingsIcon />,
};

function roleLabel(role: Role): string {
  return role.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user, logout } = useAuthStore();
  const [mobileOpen, setMobileOpen] = useState(false);

  const visibleItems = NAV_ITEMS.filter(
    (item) => user && item.roles.includes(user.role),
  );

  const handleLogout = async () => {
    await logout();
    navigate('/login', { replace: true });
  };

  const drawer = (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Brand */}
      <Toolbar>
        <Typography variant="h6" noWrap fontWeight={700}>
          FitCommerce
        </Typography>
      </Toolbar>
      <Divider />

      {/* Navigation */}
      <List sx={{ flex: 1, px: 1 }}>
        {visibleItems.map((item) => {
          const active = location.pathname.startsWith(item.path);
          return (
            <ListItemButton
              key={item.path}
              selected={active}
              onClick={() => {
                navigate(item.path);
                setMobileOpen(false);
              }}
              sx={{ borderRadius: 1, mb: 0.5 }}
            >
              <ListItemIcon sx={{ minWidth: 40 }}>
                {ICON_MAP[item.icon] ?? <DashboardIcon />}
              </ListItemIcon>
              <ListItemText primary={item.label} />
            </ListItemButton>
          );
        })}
      </List>

      <Divider />

      {/* User info + Sync + Logout */}
      <Box sx={{ p: 2 }}>
        <Box sx={{ mb: 1 }}><SyncStatusIndicator /></Box>
        {user && (
          <>
            <Typography variant="body2" fontWeight={600} noWrap>
              {user.first_name} {user.last_name}
            </Typography>
            <Chip
              label={roleLabel(user.role)}
              size="small"
              color="primary"
              variant="outlined"
              sx={{ mt: 0.5 }}
            />
          </>
        )}
        <ListItemButton
          onClick={handleLogout}
          sx={{ borderRadius: 1, mt: 1, mx: -1 }}
        >
          <ListItemIcon sx={{ minWidth: 40 }}>
            <LogoutIcon />
          </ListItemIcon>
          <ListItemText primary="Logout" />
        </ListItemButton>
      </Box>
    </Box>
  );

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      {/* App bar — mobile only */}
      <AppBar
        position="fixed"
        sx={{
          display: { md: 'none' },
          zIndex: (t) => t.zIndex.drawer + 1,
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            edge="start"
            onClick={() => setMobileOpen(!mobileOpen)}
            sx={{ mr: 2 }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap>
            FitCommerce
          </Typography>
        </Toolbar>
      </AppBar>

      {/* Mobile drawer */}
      <Drawer
        variant="temporary"
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        ModalProps={{ keepMounted: true }}
        sx={{
          display: { xs: 'block', md: 'none' },
          '& .MuiDrawer-paper': { width: DRAWER_WIDTH },
        }}
      >
        {drawer}
      </Drawer>

      {/* Permanent drawer — desktop */}
      <Drawer
        variant="permanent"
        sx={{
          display: { xs: 'none', md: 'block' },
          '& .MuiDrawer-paper': {
            width: DRAWER_WIDTH,
            boxSizing: 'border-box',
          },
        }}
        open
      >
        {drawer}
      </Drawer>

      {/* Main content */}
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          width: { md: `calc(100% - ${DRAWER_WIDTH}px)` },
          mt: { xs: 8, md: 0 },
          p: 3,
        }}
      >
        <Outlet />
      </Box>
    </Box>
  );
}
