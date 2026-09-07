// Tests del GroupRequestsPage (spec frontend-moderation req 6): lista
// de solicitudes con estados, aprobar/rechazar y concurrencia (otro
// admin ya decidio -> VALIDATION_ERROR legible).
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupRequestsPage from './GroupRequestsPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'

const requests = [
  {
    id: 11,
    group_id: 123,
    user_id: 201,
    first_name: 'Juan',
    username: 'juanito',
    status: 'pending',
    requested_at: '2026-09-05T15:20:00Z',
    decided_at: null,
    decided_by: null,
  },
  {
    id: 12,
    group_id: 123,
    user_id: 202,
    first_name: 'Ana',
    username: null,
    status: 'approved',
    requested_at: '2026-09-04T10:00:00Z',
    decided_at: '2026-09-04T11:30:00Z',
    decided_by: 1,
  },
]

function renderRequests() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/groups/123/requests']}>
        <Routes>
          <Route path="/groups/:id/requests" element={<GroupRequestsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('GroupRequestsPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('lista las solicitudes con su estado', async () => {
    mockFetchRoutes({
      '/api/groups/123/join-requests': () => okJson(requests),
    })

    renderRequests()

    expect(await screen.findByText('Juan')).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()
    expect(screen.getByText('Pendiente')).toBeInTheDocument()
    expect(screen.getByText('Aprobada')).toBeInTheDocument()
    // El boton de decidir solo aparece para solicitudes pendientes.
    expect(screen.getAllByRole('button', { name: 'Aprobar' })).toHaveLength(1)
  })

  it('muestra estado vacio', async () => {
    mockFetchRoutes({
      '/api/groups/123/join-requests': () => okJson([]),
    })

    renderRequests()

    expect(await screen.findByText(/Sin solicitudes de ingreso/i)).toBeInTheDocument()
  })

  it('muestra error de carga y permite reintentar', async () => {
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/join-requests': () => {
        if (spy.mock.calls.length === 0) {
          spy()
          return errorJson(403, 'PERMISSION_DENIED', 'El bot no tiene permisos suficientes en este grupo.')
        }
        return okJson(requests)
      },
    })

    renderRequests()

    expect(await screen.findByText(/el bot no tiene permisos suficientes/i)).toBeInTheDocument()
    screen.getByRole('button', { name: 'Reintentar' }).click()
    expect(await screen.findByText('Juan')).toBeInTheDocument()
  })

  it('aprueba una solicitud pendiente', async () => {
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/join-requests/11/approve': () => {
        spy()
        return okJson({ status: 'approved' })
      },
      '/api/groups/123/join-requests': () => okJson(requests),
    })

    renderRequests()

    const approveButton = await screen.findByRole('button', { name: 'Aprobar' })
    approveButton.click()

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(await screen.findByText(/decisión enviada/i)).toBeInTheDocument()
  })

  it('rechaza una solicitud pendiente', async () => {
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/join-requests/11/reject': () => {
        spy()
        return okJson({ status: 'rejected' })
      },
      '/api/groups/123/join-requests': () => okJson(requests),
    })

    renderRequests()

    const rejectButton = await screen.findByRole('button', { name: 'Rechazar' })
    rejectButton.click()

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(await screen.findByText(/decisión enviada/i)).toBeInTheDocument()
  })

  it('muestra mensaje legible si la solicitud ya fue decidida', async () => {
    mockFetchRoutes({
      '/api/groups/123/join-requests/11/approve': () =>
        errorJson(400, 'VALIDATION_ERROR', 'La solicitud de ingreso ya fue decidida.'),
      '/api/groups/123/join-requests': () => okJson(requests),
    })

    renderRequests()

    const approveButton = await screen.findByRole('button', { name: 'Aprobar' })
    approveButton.click()

    expect(await screen.findByText(/la solicitud de ingreso ya fue decidida/i)).toBeInTheDocument()
  })
})