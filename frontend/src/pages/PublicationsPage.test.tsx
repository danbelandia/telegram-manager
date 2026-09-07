// Tests del PublicationsPage (spec publications req 8): lista con estados
// loading/vacio/error, y creacion que invalida la lista (la nueva
// publicacion aparece) o muestra un error legible. Se usa mockFetchRoutes
// (dispatch por substring de URL) salvo donde se distingue metodo GET/POST
// sobre el mismo path.
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import PublicationsPage from './PublicationsPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'

const groups = [
  { id: '1', telegram_id: -100123, title: 'MU Online Comunidad', username: null, type: 'supergroup', member_count: 4821, bot_status: 'administrator', bot_permissions: { can_manage_chat: true } },
  { id: '2', telegram_id: -100456, title: 'Programadores', username: null, type: 'supergroup', member_count: 1203, bot_status: 'administrator', bot_permissions: { can_manage_chat: true } },
]

const pubs = [
  {
    id: 1,
    telegram_id: -100123,
    text: 'Hola mundo',
    status: 'sent',
    message_id: 77,
    error_message: null,
    actor_id: 1,
    created_at: '2026-09-06T10:00:00Z',
  },
  {
    id: 2,
    telegram_id: -100456,
    text: 'Segunda publicación',
    status: 'failed',
    message_id: null,
    error_message: 'Telegram rechazo la publicacion',
    actor_id: 1,
    created_at: '2026-09-05T09:00:00Z',
  },
]

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/publications']}>{<PublicationsPage />}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('PublicationsPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('lista las publicaciones con estado y fecha', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs),
    })

    renderPage()

    expect(await screen.findByText(/Hola mundo/)).toBeInTheDocument()
    expect(screen.getByText(/Segunda publicación/)).toBeInTheDocument()
    expect(screen.getByText('Enviada')).toBeInTheDocument()
    expect(screen.getByText('Fallida')).toBeInTheDocument()
  })

  it('muestra estado vacio', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson([]),
    })

    renderPage()

    expect(await screen.findByText(/No hay publicaciones/)).toBeInTheDocument()
  })

  it('muestra error de carga y permite reintentar', async () => {
    let failed = true
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () =>
        failed ? errorJson(500, 'INTERNAL_ERROR', 'no se pudieron listar las publicaciones') : okJson(pubs),
    })

    renderPage()

    expect(await screen.findByText(/no se pudieron listar las publicaciones/)).toBeInTheDocument()
    failed = false
    screen.getByRole('button', { name: 'Reintentar' }).click()
    expect(await screen.findByText(/Hola mundo/)).toBeInTheDocument()
  })

  it('crea una publicacion exitosa que aparece en la lista', async () => {
    // GET y POST comparten path /api/publications: distinguimos por metodo
    // con un mock propio en lugar de mockFetchRoutes.
    const created = { ...pubs[0], id: 3, text: 'Nueva publicación' }
    let list = [] as typeof pubs
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications') && (init?.method ?? 'GET') === 'POST') {
        list = [created, ...list]
        return Promise.resolve({ ok: true, status: 201, json: () => Promise.resolve({ data: created, error: null }) })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: list, error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    // Espera a que las opciones de grupo carguen (query async de useGroups).
    await waitFor(() => screen.getByRole('option', { name: /MU Online Comunidad/ }))
    await user.selectOptions(screen.getByLabelText(/Grupo/), '-100123')
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'Nueva publicación')
    await user.click(screen.getByRole('button', { name: 'Publicar ahora' }))

    expect(await screen.findByText(/Publicación enviada/)).toBeInTheDocument()
    // La query de lista se invalida y re-fetchea: la nueva aparece en la
    // tabla (identificada por el atributo title de la celda).
    expect(await screen.findByTitle('Nueva publicación')).toBeInTheDocument()
  })

  it('muestra mensaje legible si el bot no tiene permisos', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () =>
        errorJson(403, 'PERMISSION_DENIED', 'el bot no tiene permisos suficientes en este grupo'),
    })

    // La lista falla (PERMISSION_DENIED) y no podemos crear; el error
    // legible debe verse en el historial.
    renderPage()

    expect(
      await screen.findByText(/el bot no tiene permisos suficientes en este grupo/),
    ).toBeInTheDocument()
  })

  it('muestra mensaje legible al intentar publicar sin permisos', async () => {
    // GET de lista OK; POST responde PERMISSION_DENIED -> el error
    // aparece en el formulario.
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications') && (init?.method ?? 'GET') === 'POST') {
        return Promise.resolve({
          ok: false,
          status: 403,
          json: () => Promise.resolve({ data: null, error: { code: 'PERMISSION_DENIED', message: 'el bot no tiene permisos suficientes en este grupo' } }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await waitFor(() => expect(screen.getByText(/Hola mundo/)).toBeInTheDocument())

    const user = userEvent.setup()
    await waitFor(() => screen.getByRole('option', { name: /MU Online Comunidad/ }))
    await user.selectOptions(screen.getByLabelText(/Grupo/), '-100123')
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'Hola')
    await user.click(screen.getByRole('button', { name: 'Publicar ahora' }))

    expect(
      await screen.findByText(/el bot no tiene permisos suficientes en este grupo/),
    ).toBeInTheDocument()
  })
})
