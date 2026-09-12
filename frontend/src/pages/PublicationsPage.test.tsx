// Tests del PublicationsPage (spec publications req 8 — slice 3
// extiende con dual-mode, datetime-local, Cancelar y paginacion).
// Lista con estados loading/vacio/error, creacion multi-grupo con
// foto + botones, filtro por grupo que recarga con `?group_id=`, modo
// Programar, validacion de fecha pasada, Cancelar visible solo en
// scheduled, y Prev/Next funcional.
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Mock DateTimePicker → un input nativo simple para tests.
// DateTimePicker es button-based (no se puede escribir con user.type),
// asi que lo reemplazamos con un <input> que acepta "YYYY-MM-DDTHH:MM".
vi.mock('@mantine/dates', () => ({
  DateTimePicker: ({ value, onChange, label }: { value: unknown; onChange: (v: Date | null) => void; label: string }) => {
    const dateToValue = (v: unknown): string => {
      if (!v) return ''
      if (typeof v === 'string') return v
      if (v instanceof Date) {
        const pad = (n: number) => String(n).padStart(2, '0')
        return `${v.getFullYear()}-${pad(v.getMonth() + 1)}-${pad(v.getDate())}T${pad(v.getHours())}:${pad(v.getMinutes())}`
      }
      return ''
    }
    return (
      <label>
        {label}
        <input
          type="datetime-local"
          value={dateToValue(value)}
          onChange={(e) => onChange(e.target.value ? new Date(e.target.value) : null)}
        />
      </label>
    )
  },
}))
import PublicationsPage from './PublicationsPage'
import { errorJson, matchQuery, mockFetchRoutes, okJson, renderWithProviders } from '../test/helpers'

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
    scheduled_at: null,
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
    scheduled_at: null,
    created_at: '2026-09-05T09:00:00Z',
  },
  {
    id: 3,
    telegram_id: -100123,
    text: 'Programada para mañana',
    status: 'scheduled',
    message_id: null,
    error_message: null,
    actor_id: 1,
    photo_url: null,
    buttons: null,
    scheduled_at: '2027-01-01T10:00:00Z',
    created_at: '2026-09-07T09:00:00Z',
  },
]

function renderPage() {
  return renderWithProviders(<PublicationsPage />, ['/publications'])
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

  // --- slice 3: dual-mode + Cancelar + paginacion ---

  it('modo Programar muestra datetime-local y cambia el label del submit', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs),
    })

    renderPage()

    // Por default modo "Publicar ahora" -> sin datetime-local visible.
    expect(screen.queryByLabelText(/Fecha y hora/i)).not.toBeInTheDocument()
    // Boton "Publicar ahora" presente.
    expect(screen.getByRole('button', { name: 'Publicar ahora' })).toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(screen.getByLabelText('Programar'))

    // datetime-local ahora visible.
    expect(screen.getByLabelText(/Fecha y hora/i)).toBeInTheDocument()
    // Submit label cambia a "Programar".
    expect(screen.getByRole('button', { name: 'Programar' })).toBeInTheDocument()
  })

  it('validacion cliente: scheduled_at en el pasado bloquea POST', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      // GET con limit (paginacion slice 3).
      if (url.includes('/api/publications') && (init?.method ?? 'GET') === 'GET') {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [], error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('MU Online Comunidad'))
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'algo')
    // Cambiar a Programar.
    await user.click(screen.getByLabelText('Programar'))
    // Fecha en el pasado (2020).
    const input = screen.getByLabelText(/Fecha y hora/i)
    await user.type(input, '2020-01-01T10:00')
    await user.click(screen.getByRole('button', { name: 'Programar' }))

    expect(await screen.findByText(/la fecha debe ser futura/i)).toBeInTheDocument()
    const postCalls = fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST')
    expect(postCalls).toHaveLength(0)
  })

  it('programacion futura multi-grupo envia POST con scheduled_at', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications') && (init?.method ?? 'GET') === 'POST') {
        const scheduled = [
          {
            id: 99,
            telegram_id: -100123,
            text: 'programada',
            status: 'scheduled',
            message_id: null,
            error_message: null,
            actor_id: 1,
            photo_url: null,
            buttons: null,
            scheduled_at: '2027-06-15T14:00:00Z',
            created_at: '2026-09-07T12:00:00Z',
          },
        ]
        return Promise.resolve({
          ok: true,
          status: 201,
          json: () => Promise.resolve({ data: { publications: scheduled }, error: null }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const user = userEvent.setup()
    await waitFor(() => screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('MU Online Comunidad'))
    await user.click(screen.getByLabelText('Programadores'))
    await user.type(screen.getByPlaceholderText(/escribí el mensaje/i), 'programada')
    await user.click(screen.getByLabelText('Programar'))
    // Fecha futura lejana (anio 2027).
    await user.type(screen.getByLabelText(/Fecha y hora/i), '2027-06-15T14:00')
    await user.click(screen.getByRole('button', { name: 'Programar' }))

    expect(await screen.findByText(/Publicación programada/i)).toBeInTheDocument()
    const postCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST')
    expect(postCall).toBeDefined()
    const sentBody = JSON.parse(String((postCall![1] as RequestInit).body))
    expect(sentBody.scheduled_at).toBeTruthy()
    expect(typeof sentBody.scheduled_at).toBe('string')
    expect(sentBody.scheduled_at.length).toBeGreaterThan(10)
    expect(sentBody.group_ids).toEqual([-100123, -100456])
  })

  it('boton Cancelar visible solo en filas scheduled', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs),
    })

    renderPage()

    // Esperar render de las 3 filas.
    await screen.findByText(/Hola mundo/)
    await screen.findByText(/Segunda publicación/)
    await screen.findByText(/Programada para mañana/)

    const cancelButtons = screen.getAllByRole('button', { name: 'Cancelar' })
    expect(cancelButtons).toHaveLength(1)
  })

  it('Cancelar ejecuta DELETE y la fila desaparece tras refetch', async () => {
    const calls: string[] = []
    let currentList = [...pubs]
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      calls.push(`${method} ${url}`)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      // DELETE sobre /api/publications/3 -> 204 + quitar fila del store
      if (method === 'DELETE') {
        currentList = currentList.filter((p) => p.id !== 3)
        return Promise.resolve({ ok: true, status: 204, json: () => Promise.reject(new Error('204 sin body')) })
      }
      if (url.includes('/api/publications')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: currentList, error: null }) })
      }
      return Promise.reject(new Error('unexpected ' + url))
    })
    vi.stubGlobal('fetch', fetchMock)

    // Stubear window.confirm para evitar prompt real.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    renderPage()

    await screen.findByText(/Programada para mañana/)
    const cancelBtn = screen.getByRole('button', { name: 'Cancelar' })
    cancelBtn.click()

    // Tras DELETE + invalidate, refetch devuelve lista sin la fila.
    await waitFor(() => {
      expect(screen.queryByText(/Programada para mañana/)).not.toBeInTheDocument()
    })
    const delCalls = calls.filter((c) => c.startsWith('DELETE'))
    expect(delCalls.length).toBeGreaterThan(0)
    expect(delCalls[0]).toContain('/api/publications/3')
    confirmSpy.mockRestore()
  })

  it('Next deshabilitado cuando returned < limit (final real)', async () => {
    // Solo 2 publicaciones; limit default 50 -> Next disabled.
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs.slice(0, 2)),
    })

    renderPage()

    await screen.findByText(/Hola mundo/)
    const next = screen.getByRole('button', { name: /Siguiente/i })
    expect(next).toBeDisabled()
  })

  it('Prev deshabilitado cuando offset === 0', async () => {
    mockFetchRoutes({
      '/api/groups': () => okJson(groups),
      '/api/publications': () => okJson(pubs),
    })

    renderPage()

    await screen.findByText(/Hola mundo/)
    const prev = screen.getByRole('button', { name: /Anterior/i })
    expect(prev).toBeDisabled()
  })

  it('click en Next avanza offset y recarga la query', async () => {
    // Devolvemos >=50 items en la primera pagina para que Next se habilite.
    const firstPage = Array.from({ length: 50 }, (_, i) => ({
      ...pubs[0],
      id: 1000 + i,
      text: `pub ${i}`,
    }))
    const fetched: string[] = []
    const fetchMock = vi.fn((input: RequestInfo | URL, _init?: RequestInit) => {
      const url = String(input)
      fetched.push(url)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications')) {
        if (matchQuery(url, { offset: '0' })) {
          return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: firstPage, error: null }) })
        }
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [], error: null }) })
      }
      return Promise.reject(new Error('unexpected ' + url))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByText(/pub 0/)
    const next = screen.getByRole('button', { name: /Siguiente/i })
    expect(next).not.toBeDisabled()
    next.click()

    await waitFor(() => {
      expect(fetched.some((u) => matchQuery(u, { offset: '50', limit: '50' }))).toBe(true)
    })
  })

  it('GET lista envia ?limit=50 por default', async () => {
    const fetched: string[] = []
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      fetched.push(url)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
      }
      return Promise.reject(new Error('unexpected ' + url))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()
    await screen.findByText(/Hola mundo/)

    const pubsCall = fetched.find((u) => u.includes('/api/publications') && !u.includes('/groups'))
    expect(pubsCall).toBeDefined()
    expect(pubsCall).toContain('limit=50')
    expect(pubsCall).toContain('offset=0')
  })

  it('GET lista combina ?group_id= con ?limit=&offset=', async () => {
    const fetched: string[] = []
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      fetched.push(url)
      if (url.includes('/api/groups')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: groups, error: null }) })
      }
      if (url.includes('/api/publications')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: pubs, error: null }) })
      }
      return Promise.reject(new Error('unexpected ' + url))
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()
    await screen.findByText(/Hola mundo/)

    const user = userEvent.setup()
    await user.selectOptions(screen.getByLabelText(/Filtrar por grupo/i), '-100123')

    await waitFor(() => {
      const pubsCall = fetched.filter((u) => u.includes('/api/publications') && !u.includes('/groups')).pop()
      expect(pubsCall).toBeDefined()
      expect(pubsCall).toContain('group_id=-100123')
      expect(pubsCall).toContain('limit=50')
      expect(pubsCall).toContain('offset=0')
    })
  })
})
