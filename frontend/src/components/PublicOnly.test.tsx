// Tests de PublicOnly (spec frontend-routing): sin sesion renderiza el
// contenido publico; con sesion redirige a /groups. mockFetchRoutes
// (by URL) evita desalinear mocks con el refresh automatico del
// api-client (patron LoginPage.test.tsx).
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { describe, expect, it, vi, afterEach } from 'vitest'
import PublicOnly from './PublicOnly'
import { AuthProvider } from '../lib/auth-context'
import { createTestQueryClient, errorJson, mockFetchRoutes, okJson } from '../test/helpers'

function renderPublic(initialEntry: string) {
  // QueryClientProvider: AuthProvider usa useQueryClient() para limpiar
  // la cache al cerrar sesion (slice 4 spec).
  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <AuthProvider>
          <Routes>
            <Route path="/groups" element={<div>pagina grupos</div>} />
            <Route
              element={
                <PublicOnly>
                  <div>contenido publico</div>
                </PublicOnly>
              }
            >
              <Route path="/signup" element={<div>contenido publico</div>} />
            </Route>
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PublicOnly', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('muestra el contenido publico sin sesion', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => errorJson(401, 'UNAUTHORIZED', 'no autenticado'),
      '/api/auth/refresh': () => errorJson(401, 'UNAUTHORIZED', 'refresh invalido'),
    })

    renderPublic('/signup')

    expect(await screen.findByText('contenido publico')).toBeInTheDocument()
  })

  it('redirige a /groups con sesion', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => okJson({ id: '1', username: 'admin', tenant_id: 7 }),
    })

    renderPublic('/signup')

    await waitFor(() => expect(screen.getByText('pagina grupos')).toBeInTheDocument())
    expect(screen.queryByText('contenido publico')).not.toBeInTheDocument()
  })
})
