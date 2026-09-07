// Tests de RequireAuth (spec frontend-routing): sin sesion redirige a
// /login, con sesion renderiza el contenido protegido.
// El test incluye una ruta /login real para que el <Navigate> del
// redirect aterrice en un destino (si no, router + Navigate bucean).
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RequireAuth from './RequireAuth'
import { AuthProvider } from '../lib/auth-context'

function renderProtected(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<div>pagina login</div>} />
          <Route
            element={
              <RequireAuth>
                <div>contenido protegido</div>
              </RequireAuth>
            }
          >
            <Route path="/groups" element={<div>contenido protegido</div>} />
          </Route>
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  beforeEach(() => {
    // Sin sesion al montar: /me responde 401.
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }),
    }))
  })

  it('redirige a /login sin sesion', async () => {
    renderProtected('/groups')

    await waitFor(() => expect(screen.getByText('pagina login')).toBeInTheDocument())
    expect(screen.queryByText('contenido protegido')).not.toBeInTheDocument()
  })

  it('muestra el contenido con sesion', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: { id: '1', username: 'admin' }, error: null }),
    }))

    renderProtected('/groups')

    expect(await screen.findByText('contenido protegido')).toBeInTheDocument()
  })
})