// Tests del GroupLogsPage (spec frontend-moderation req 7): lista de
// entradas de auditoria ordenada (la API ya devuelve desc por
// created_at), con accion/status/detalle y fecha legible.
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupLogsPage from './GroupLogsPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'

const logs = [
  {
    id: 1,
    actor_id: 42,
    group_id: 123,
    action: 'BAN_USER',
    target_user_id: 99,
    metadata: {},
    status: 'SUCCESS',
    error_message: null,
    created_at: '2026-09-05T15:20:00Z',
  },
  {
    id: 2,
    actor_id: 42,
    group_id: 123,
    action: 'LOCK_GROUP',
    target_user_id: null,
    metadata: {},
    status: 'PERMISSION_DENIED',
    error_message: 'El bot no tiene permisos suficientes en este grupo.',
    created_at: '2026-09-04T10:00:00Z',
  },
]

function renderLogs() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/groups/123/logs']}>
        <Routes>
          <Route path="/groups/:id/logs" element={<GroupLogsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('GroupLogsPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('lista las entradas con accion y status', async () => {
    mockFetchRoutes({
      '/api/groups/123/logs': () => okJson(logs),
    })

    renderLogs()

    expect(await screen.findByText('BAN_USER')).toBeInTheDocument()
    expect(screen.getByText('LOCK_GROUP')).toBeInTheDocument()
    expect(screen.getAllByText('SUCCESS')).toHaveLength(1)
    expect(screen.getByText('PERMISSION_DENIED')).toBeInTheDocument()
    // Detalle: target en BAN_USER y error en LOCK_GROUP.
    expect(screen.getByText(/usuario 99/)).toBeInTheDocument()
    expect(screen.getByText(/el bot no tiene permisos suficientes/i)).toBeInTheDocument()
  })

  it('muestra estado vacio', async () => {
    mockFetchRoutes({
      '/api/groups/123/logs': () => okJson([]),
    })

    renderLogs()

    expect(await screen.findByText(/Sin acciones registradas/i)).toBeInTheDocument()
  })

  it('muestra error de carga y permite reintentar', async () => {
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/logs': () => {
        if (spy.mock.calls.length === 0) {
          spy()
          return errorJson(403, 'PERMISSION_DENIED', 'El bot no tiene permisos suficientes en este grupo.')
        }
        return okJson(logs)
      },
    })

    renderLogs()

    expect(await screen.findByText(/el bot no tiene permisos suficientes/i)).toBeInTheDocument()
    screen.getByRole('button', { name: 'Reintentar' }).click()
    expect(await screen.findByText('BAN_USER')).toBeInTheDocument()
  })
})