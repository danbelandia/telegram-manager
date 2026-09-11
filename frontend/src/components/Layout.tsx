// Layout autenticado del panel (frontend-refresh slice 1 — design D6).
// Reemplaza el `<div className="layout">` ad-hoc por AppShell de Mantine:
// header 60px con brand + ColorSchemeToggle + boton Logout, navbar 260px
// con los NavLinks a rutas globales + icono Tabler, y `<Outlet/>` en
// AppShell.Main para las rutas hijas (spec frontend-routing).
// El wrapping `<RequireAuth><Layout/></RequireAuth>` se conserva en App.tsx
// — LoginPage queda FUERA del AppShell.
// Layout autenticado del panel (frontend-refresh slice 1 — design D6).
// Reemplaza el `<div className="layout">` ad-hoc por AppShell de Mantine:
// header 60px con brand + ColorSchemeToggle + boton Logout, navbar 260px
// con los NavLinks a rutas globales + icono Tabler, y `<Outlet/>` en
// AppShell.Main para las rutas hijas (spec frontend-routing).
// El wrapping `<RequireAuth><Layout/></RequireAuth>` se conserva en App.tsx
// — LoginPage queda FUERA del AppShell.
import {
  AppShell,
  Burger,
  Group,
  NavLink,
  Stack,
  Text,
  Title,
  Button,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import {
  IconLayoutDashboard,
  IconUsersGroup,
  IconSend,
  IconLogout,
  IconSettings,
  IconShield,
} from '@tabler/icons-react'
import { Link, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth-context'
import { notifySuccess } from '../lib/notifications'
import ColorSchemeToggle from './ColorSchemeToggle'
import DegradedBanner from './DegradedBanner'

type NavItem = {
  to: string
  label: string
  icon: React.ComponentType<{ size?: number }>
}

const BASE_NAV_ITEMS: ReadonlyArray<NavItem> = [
  { to: '/dashboard', label: 'Dashboard', icon: IconLayoutDashboard },
  { to: '/groups', label: 'Grupos', icon: IconUsersGroup },
  { to: '/publications', label: 'Publicaciones', icon: IconSend },
  { to: '/tenant', label: 'Configuracion', icon: IconSettings },
]

export default function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const [mobileOpened, { toggle: toggleMobile }] = useDisclosure(false)

  const navItems: NavItem[] = [
    ...BASE_NAV_ITEMS,
    ...(user?.isSuperAdmin
      ? [{ to: '/admin/tenants', label: 'Admin Panel', icon: IconShield } as NavItem]
      : []),
  ]

  const handleLogout = async () => {
    await logout()
    notifySuccess('Sesión cerrada')
    navigate('/login', { replace: true })
  }

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{
        width: 260,
        breakpoint: 'sm',
        collapsed: { mobile: !mobileOpened },
      }}
      padding="md"
    >
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger
              opened={mobileOpened}
              onClick={toggleMobile}
              hiddenFrom="sm"
              size="sm"
            />
            <Title order={4} fw={700}>
              Telegram Manager
            </Title>
          </Group>
          <Group gap="sm" wrap="nowrap">
            {user ? (
              <>
                <Text size="sm" c="dimmed" visibleFrom="sm">
                  {user.username}
                </Text>
                {/* Badge del tenant (REQ badge): slug conocido o
                    fallback "Tenant #id"; nunca vacio. */}
                <Text size="sm" fw={600} data-testid="tenant-badge">
                  {user.tenantSlug ?? (user.tenantId !== null ? `Tenant #${user.tenantId}` : 'Tenant')}
                </Text>
              </>
            ) : null}
            <ColorSchemeToggle />
            <Button
              variant="default"
              leftSection={<IconLogout size={16} />}
              onClick={handleLogout}
              data-testid="logout-button"
            >
              Salir
            </Button>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="md">
        <Stack gap="xs">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              component={Link}
              to={item.to}
              label={item.label}
              leftSection={<item.icon size={18} />}
              data-testid={`nav-${item.to.replace('/', '') || 'root'}`}
            />
          ))}
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main>
        <DegradedBanner />
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}