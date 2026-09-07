// Tests del DashboardPage (spec frontend-dashboard): lista grupos con
// estados loading/vacio/error + reintento. Se usa mockFetchRoutes (por
// URL) para que los mocks no se desalineen con el refresh del api-client.
// El wrapper provee MantineProvider + Notifications porque el componente
// migrado usa Card / Stack / SimpleGrid / Skeleton de Mantine v7
// (frontend-refresh slice 1).
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import DashboardPage from './DashboardPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

function renderDashboard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <DashboardPage />
        </MemoryRouter>
      </QueryClientProvider>
    </MantineProvider>,
  )
}

const groups = [
  {
    id: '1',
    telegram_id: -100123456789,
    title: 'MU Online Comunidad',
    username: 'muonline',
    type: 'supergroup',
    member_count: 4821,
    bot_status: 'administrator',
    bot_permissions: { can_delete_messages: true, can_restrict_members: true },
  },
]

describe('DashboardPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('muestra los grupos administrables', async () => {
    mockFetchRoutes({ '/api/groups': () => okJson(groups) })

    renderDashboard()

    expect(await screen.findByText('MU Online Comunidad')).toBeInTheDocument()
    expect(screen.getByText('@muonline')).toBeInTheDocument()
    expect(screen.getByText('ID: -100123456789')).toBeInTheDocument()
    expect(screen.getByText('Miembros: 4.821')).toBeInTheDocument()
    // El detalle se resuelve por telegram_id, no por el id de BD (bug
    // corregido: antes el link apuntaba a /groups/1 -> "grupo no encontrado").
    expect(screen.getByRole('link', { name: 'Administrar' })).toHaveAttribute(
      'href',
      '/groups/-100123456789',
    )
    // Los permisos se muestran legibles, no como claves tecnicas can_*.
    expect(screen.getByText('Eliminar mensajes, Restringir miembros')).toBeInTheDocument()
  })

  it('muestra estado vacio cuando no hay grupos', async () => {
    mockFetchRoutes({ '/api/groups': () => okJson([]) })

    renderDashboard()

    expect(
      await screen.findByText(/Todavía no hay grupos/i),
    ).toBeInTheDocument()
  })

  it('muestra error y permite reintentar', async () => {
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups': () => {
        spy()
        return errorJson(502, 'TELEGRAM_ERROR', 'error de prueba')
      },
    })

    renderDashboard()

    expect(await screen.findByText('error de prueba')).toBeInTheDocument()
    const retry = screen.getByRole('button', { name: 'Reintentar' })
    spy.mockClear()
    retry.click()

    // El botón dispara refetch -> se vuelve a llamar a /api/groups.
    await vi.waitFor(() => expect(spy).toHaveBeenCalled())
  })

  it('smoke: renderiza una Card por grupo con data-testid (slice 1 foundation)', async () => {
    mockFetchRoutes({
      '/api/groups': () =>
        okJson([
          {
            id: 'a',
            telegram_id: -1001,
            title: 'Grupo A',
            username: null,
            type: 'supergroup',
            member_count: 100,
            bot_status: 'administrator',
            bot_permissions: {},
          },
          {
            id: 'b',
            telegram_id: -1002,
            title: 'Grupo B',
            username: null,
            type: 'supergroup',
            member_count: 200,
            bot_status: 'member',
            bot_permissions: {},
          },
        ]),
    })

    renderDashboard()

    const cards = await screen.findAllByTestId('group-card')
    expect(cards).toHaveLength(2)
    expect(cards[0]).toHaveTextContent('Grupo A')
    expect(cards[1]).toHaveTextContent('Grupo B')
  })
})