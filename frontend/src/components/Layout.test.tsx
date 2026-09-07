// Smoke test del Layout autenticado (frontend-refresh slice 1 — design D6).
// Verifica que el AppShell de Mantine renderiza header + navbar (los 3
// NavLinks a rutas globales) + outlet. Mockea useAuth para inyectar un
// usuario sin depender del backend.
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Layout from './Layout'
import { mantineTheme } from '../theme'

// Mock del hook de auth: Layout consume `useAuth` para el username y
// para `logout` (se ejecuta async en el wrapper). El mock evita la red.
vi.mock('../lib/auth-context', () => ({
  useAuth: () => ({
    loading: false,
    user: { id: '1', username: 'admin' },
    login: vi.fn(),
    logout: vi.fn(async () => {}),
  }),
}))

function renderLayout(initialEntries: string[] = ['/dashboard']) {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route element={<Layout />}>
            <Route
              path="/dashboard"
              element={<div data-testid="outlet-content">Dashboard outlet</div>}
            />
          </Route>
        </Routes>
      </MemoryRouter>
    </MantineProvider>,
  )
}

describe('Layout', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('smoke: renderiza header, navbar con 3 NavLinks, toggle y outlet', async () => {
    renderLayout()

    // Header: brand + username + toggle + boton logout.
    expect(screen.getByText('Telegram Manager')).toBeInTheDocument()
    expect(screen.getByText('admin')).toBeInTheDocument()
    expect(screen.getByTestId('color-scheme-toggle')).toBeInTheDocument()
    expect(screen.getByTestId('logout-button')).toBeInTheDocument()

    // Navbar: las 3 rutas globales.
    expect(screen.getByTestId('nav-dashboard')).toBeInTheDocument()
    expect(screen.getByTestId('nav-groups')).toBeInTheDocument()
    expect(screen.getByTestId('nav-publications')).toBeInTheDocument()

    // Outlet: el contenido de la ruta hija.
    expect(screen.getByTestId('outlet-content')).toBeInTheDocument()
  })
})