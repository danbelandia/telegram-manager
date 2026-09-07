// Tests del PublicationsPage (spec publications req 8): lista con estados
// loading/vacio/error, creacion multi-grupo con foto + botones, y
// filtro por grupo que recarga con `?group_id=`. Se usa
// `mockFetchRoutes` (dispatch por substring de URL) salvo donde se
// distingue metodo GET/POST sobre el mismo path.
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import PublicationsPage from './PublicationsPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'

const groups = [
  {
    id: '1',
    telegram_id: -100123,
    title: 'MU Online Comunidad',
    username: null,
    type: 'supergroup',
    member_count: 4821,
    bot_status: 'administrator',
    bot_permissions: { can_manage_chat: true },
  },
  {
    id: '2',
    telegram_id: -100456,
    title: 'Programadores',
    username: null,
    type: 'supergroup',
    member_count: 1203,
    bot_status: 'administrator',
    bot_permissions: { can_manage_chat: true },
  },
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
    photo_url: 'https://example.com/x.jpg',
    buttons: [[{ text: 'Ir', url: 'https://example.com' }]],
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
    photo_url: null,
    buttons: null,
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

  it('lista las publicaciones con estado, foto y botones', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs),
    })

    renderPage()

    expect(await screen.findByText(/Hola mundo/)).toBeInTheDocument()
    expect(screen.getByText(/Segunda publicación/)).toBeInTheDocument()
    expect(screen.getByText('Enviada')).toBeInTheDocument()
    expect(screen.getByText('Fallida')).toBeInTheDocument()
    // La imagen de la primera publicacion aparece en la tabla.
    expect(screen.getByAltText(/foto publicación/i)).toHaveAttribute(
      'src',
      'https://example.com/x.jpg',
    )
    // El chip del boton "Ir" aparece.
    expect(screen.getByText('Ir')).toBeInTheDocument()
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

  it('crea una publicacion multi-grupo con foto y botones', async () => {
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
        return Promise.resolve({
          ok: true,
          status: 201,
          json: () => Promise.resolve({ data: { publications: [created] }, error: null }),
        })
      }
      // Distinguimos GET con query de filtro del GET sin filtro.
      if (url.includes('/api/publications') && url.includes('group_id=')) {
        const gid = new URL(url, 'http://x').searchParams.get('group_id')
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              data: list.filter((p) => String(p.telegram_id) === gid),
              error: null,
            }),
        })
      }
      return Promise.resolve({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ data: list, error: null }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    // Espera a que las opciones de grupo carguen.
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    await waitFor(() => screen.getByLabelText('Programadores'))

    // Selecciona 2 grupos (multi-select via checkboxes).
    await user.click(screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('Programadores'))

    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'Nueva publicación')

    // Foto
    await user.type(screen.getByPlaceholderText(/ejemplo\.com\/imagen/i), 'https://example.com/x.jpg')

    // Agregar fila de botones + un boton
    await user.click(screen.getByRole('button', { name: /\+ Agregar fila de botones/i }))
    const textInputs = screen.getAllByPlaceholderText(/Texto del botón/i)
    const urlInputs = screen.getAllByPlaceholderText(/https:\/\/\.\.\./i)
    await user.type(textInputs[0], 'Ir')
    await user.type(urlInputs[0], 'https://example.com')

    await user.click(screen.getByRole('button', { name: 'Publicar ahora' }))

    expect(await screen.findByText(/Publicación enviada/)).toBeInTheDocument()

    // Verificar que el POST se hizo con el body correcto.
    const postCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST')
    expect(postCall).toBeDefined()
    const sentBody = JSON.parse(String((postCall![1] as RequestInit).body))
    expect(sentBody.text).toBe('Nueva publicación')
    expect(sentBody.photo_url).toBe('https://example.com/x.jpg')
    expect(sentBody.buttons).toEqual([[{ text: 'Ir', url: 'https://example.com' }]])
    expect(sentBody.group_ids).toEqual([-100123, -100456])

    // La query se invalida y re-fetchea.
    expect(await screen.findByTitle('Nueva publicación')).toBeInTheDocument()
  })

  it('filtra por grupo y recarga con ?group_id=', async () => {
    const calls: string[] = []
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      calls.push(url)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      // GET sin filtro -> 2 pubs (1 en cada grupo)
      if (url.includes('/api/publications') && url.includes('group_id=')) {
        const gid = new URL(url, 'http://x').searchParams.get('group_id')
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({
            data: pubs.filter((p) => String(p.telegram_id) === gid),
            error: null,
          }),
        })
      }
      if (url.includes('/api/publications')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
      }
      return Promise.reject(new Error('unexpected ' + url))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    // Esperar a que carguen ambas publicaciones.
    expect(await screen.findByText(/Hola mundo/)).toBeInTheDocument()
    expect(screen.getByText(/Segunda publicación/)).toBeInTheDocument()

    // Cambiar el filtro al primer grupo.
    const user = userEvent.setup()
    const filterSelect = screen.getByLabelText(/Filtrar por grupo/i)
    await user.selectOptions(filterSelect, '-100123')

    // Debe volver a quedar solo la primera publicacion (grupo -100123).
    await waitFor(() => {
      expect(screen.queryByText(/Segunda publicación/)).not.toBeInTheDocument()
    })
    expect(screen.getByText(/Hola mundo/)).toBeInTheDocument()

    // Confirmar que la nueva request llevo el query param.
    expect(calls.some((c) => c.includes('group_id=-100123'))).toBe(true)
  })

  it('no envia POST si la URL de la foto no es http(s)', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [], error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('MU Online Comunidad'))
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'algo')
    await user.type(screen.getByPlaceholderText(/ejemplo\.com\/imagen/i), 'javascript:alert(1)')
    await user.click(screen.getByRole('button', { name: 'Publicar ahora' }))

    expect(await screen.findByText(/la URL debe empezar con http o https/)).toBeInTheDocument()
    // Ninguna llamada POST a /api/publications.
    const postCalls = fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST')
    expect(postCalls).toHaveLength(0)
  })

  it('no envia POST si no hay grupos seleccionados', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [], error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    // NO seleccionamos ningun grupo; escribimos texto y enviamos.
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'algo')
    // El boton Publicar esta deshabilitado -> click no envia nada.
    const submitBtn = screen.getByRole('button', { name: 'Publicar ahora' })
    expect(submitBtn).toBeDisabled()

    const postCalls = fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST')
    expect(postCalls).toHaveLength(0)
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
          json: () => Promise.resolve({
            data: null,
            error: { code: 'PERMISSION_DENIED', message: 'el bot no tiene permisos suficientes en este grupo' },
          }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await waitFor(() => expect(screen.getByText(/Hola mundo/)).toBeInTheDocument())

    const user = userEvent.setup()
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('MU Online Comunidad'))
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'Hola')
    await user.click(screen.getByRole('button', { name: 'Publicar ahora' }))

    expect(
      await screen.findByText(/el bot no tiene permisos suficientes en este grupo/),
    ).toBeInTheDocument()
  })
})
