import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import { AuthProvider } from './lib/auth-context'

function renderApp(initialEntries: string[] = ['/']) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={initialEntries}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
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
    vi.stubGlobal('fetch', vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/auth/me')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { id: '1', username: 'admin' }, error: null }),
        })
      }
      // /api/groups: lista vacia -> Dashboard muestra estado vacio
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: [], error: null }),
      })
    }))

    renderApp(['/dashboard'])

    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument()
  })

  it('muestra 404 en rutas desconocidas', async () => {
    renderApp(['/no-existe'])

    expect(await screen.findByText('Página no encontrada')).toBeInTheDocument()
  })
})