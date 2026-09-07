import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './lib/auth-context'

function renderApp(initialEntries: string[] = ['/']) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <AuthProvider>
        <App />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('App', () => {
  beforeEach(() => {
    // Sin sesion al montar: /me responde 401.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }),
    }))
  })

  it('la ruta raiz redirige a /login sin sesion', async () => {
    renderApp(['/'])

    // RequireAuth redirige / -> /dashboard -> /login
    await waitFor(() => expect(screen.getByRole('heading', { name: 'Iniciar sesión' })).toBeInTheDocument())
  })

  it('renderiza el dashboard con sesion', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: { id: '1', username: 'admin' }, error: null }),
    }))

    renderApp(['/dashboard'])

    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument()
  })

  it('muestra 404 en rutas desconocidas', async () => {
    renderApp(['/no-existe'])

    expect(await screen.findByText('Página no encontrada')).toBeInTheDocument()
  })
})