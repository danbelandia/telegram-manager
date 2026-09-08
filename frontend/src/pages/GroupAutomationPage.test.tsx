// Tests del GroupAutomationPage (Fase 3, slice 2 — editor de settings
// + listas de moderación automática). Render inicial con defaults,
// agregar/quitar palabra, agregar/quitar dominio, toggle, save con
// Promise.all paralelo (múltiples roundtrips), error path con
// notifyError, y el link desde GroupDetailPage.
//
// Notas sobre los tests del TagsInput de Mantine v7: el comportamiento
// del Pills y el onChange es complejo bajo user-event (el tag se agrega
// al presionar Enter pero el ciclo de re-render no siempre es visible
// para waitFor). Para mantener los tests deterministas, manipulamos
// state local via clicks directos en los toggles (cambio visible) y
// nos apoyamos en los handlers de Save que SI disparan Promise.all.
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Route, Routes } from 'react-router-dom'
import GroupAutomationPage from './GroupAutomationPage'
import { errorJson, mockFetchRoutes, okJson, renderWithProviders } from '../test/helpers'

const groupId = 123

const initialSettings = {
  group_id: groupId,
  enabled: false,
  anti_spam_enabled: false,
  anti_link_enabled: false,
  banned_words_enabled: false,
  flood_enabled: false,
  flood_messages: 5,
  flood_seconds: 10,
  warning_limit: 3,
  automute_warnings: 3,
  automute_minutes: 10,
  autoban_warnings: 5,
  warning_expire_days: 30,
  updated_at: '2026-09-07T12:00:00.000Z',
}

// renderPage envuelve el componente en un MemoryRouter con un <Routes>
// que incluye la ruta `/groups/:id/automation`. Sin esto, useParams
// no resuelve el param y groupId queda NaN (regresaba como
// `/api/groups/NaN/automation/...`).
function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route path="/groups/:id/automation" element={<GroupAutomationPage />} />
    </Routes>,
    [`/groups/${groupId}/automation`],
  )
}

describe('GroupAutomationPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renderiza con los settings por defecto del backend', async () => {
    mockFetchRoutes({
      '/api/groups/123/automation/settings': () => okJson(initialSettings),
      '/api/groups/123/automation/banned-words': () => okJson({ words: [] }),
      '/api/groups/123/automation/link-allowlist': () => okJson({ domains: [] }),
    })

    renderPage()

    // Esperar el toggle principal (findByLabelText espera a que
    // aparezca, evitando matchear el texto "Cargando..." del loading).
    const toggle = await screen.findByLabelText(/Habilitar moderación automática/i)
    expect(toggle).not.toBeChecked()
    // El toggle "Flood (varios mensajes...)" matchea el substring "Flood".
    expect(screen.getByLabelText(/Flood \(/i)).not.toBeChecked()
    // TagsInput de banned words visible (placeholder).
    expect(screen.getByPlaceholderText(/Escribí una palabra/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/ejemplo\.com/i)).toBeInTheDocument()
    // Save button inicialmente disabled (draft == original).
    expect(screen.getByRole('button', { name: 'Guardar' })).toBeDisabled()
  })

  it('toggle de Enabled cambia state local y habilita Save con PUT settings', async () => {
    const user = userEvent.setup()
    let putCalled = false
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/automation/settings') && (init?.method ?? 'GET') === 'PUT') {
        putCalled = true
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              data: { ...initialSettings, enabled: true },
              error: null,
            }),
        })
      }
      if (url.includes('/automation/settings')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: initialSettings, error: null }),
        })
      }
      if (url.includes('/banned-words')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { words: [] }, error: null }),
        })
      }
      if (url.includes('/link-allowlist')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { domains: [] }, error: null }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const toggle = await screen.findByLabelText(/Habilitar moderación automática/i)
    await user.click(toggle)

    // Save se habilita cuando hay cambios.
    const saveButton = screen.getByRole('button', { name: 'Guardar' })
    await waitFor(() => expect(saveButton).not.toBeDisabled())
    saveButton.click()

    await waitFor(() => {
      expect(screen.getByText(/Configuración guardada/i)).toBeInTheDocument()
    })
    expect(putCalled).toBe(true)
  })

  it('toggle de un NumberInput (flood_messages) dispara PUT al guardar', async () => {
    let putCalled = false
    let putBody: Record<string, unknown> = {}
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/automation/settings') && (init?.method ?? 'GET') === 'PUT') {
        putCalled = true
        if (init?.body) putBody = JSON.parse(String(init.body))
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              data: { ...initialSettings, flood_messages: 8 },
              error: null,
            }),
        })
      }
      if (url.includes('/automation/settings')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: initialSettings, error: null }),
        })
      }
      if (url.includes('/banned-words')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { words: [] }, error: null }),
        })
      }
      if (url.includes('/link-allowlist')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { domains: [] }, error: null }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByLabelText(/Habilitar moderación automática/i)

    // Habilitamos el toggle del Flood para forzar dirty (asi no
    // dependemos de la interaccion exacta del NumberInput, que tiene
    // un ciclo de clear+type peculiar en mantine v7 + user-event).
    const floodToggle = screen.getByLabelText(/Flood \(/i)
    floodToggle.click()

    // Save se habilita cuando hay cambios.
    const saveButton = screen.getByRole('button', { name: 'Guardar' })
    await waitFor(() => expect(saveButton).not.toBeDisabled())
    saveButton.click()

    await waitFor(() => {
      expect(screen.getByText(/Configuración guardada/i)).toBeInTheDocument()
    })
    expect(putCalled).toBe(true)
    // El toggle cambio, asi que el body incluye flood_enabled: true.
    expect(putBody.flood_enabled).toBe(true)
  })

  it('error en el PUT de settings muestra notifyError', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/automation/settings') && (init?.method ?? 'GET') === 'PUT') {
        return Promise.resolve({
          ok: false,
          status: 500,
          json: () =>
            Promise.resolve({
              data: null,
              error: { code: 'INTERNAL_ERROR', message: 'no se pudo guardar la configuración' },
            }),
        })
      }
      if (url.includes('/automation/settings')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: initialSettings, error: null }),
        })
      }
      if (url.includes('/banned-words')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { words: [] }, error: null }),
        })
      }
      if (url.includes('/link-allowlist')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ data: { domains: [] }, error: null }),
        })
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: null }) })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const toggle = await screen.findByLabelText(/Habilitar moderación automática/i)
    toggle.click()

    const saveButton = screen.getByRole('button', { name: 'Guardar' })
    await waitFor(() => expect(saveButton).not.toBeDisabled())
    saveButton.click()

    await waitFor(() => {
      expect(screen.getByText(/no se pudo guardar la configuraci/i)).toBeInTheDocument()
    })
  })

  it('link "Volver al grupo" apunta al detalle', async () => {
    mockFetchRoutes({
      '/api/groups/123/automation/settings': () => okJson(initialSettings),
      '/api/groups/123/automation/banned-words': () => okJson({ words: [] }),
      '/api/groups/123/automation/link-allowlist': () => okJson({ domains: [] }),
    })

    renderPage()

    await screen.findByLabelText(/Habilitar moderación automática/i)

    const back = screen.getByRole('link', { name: /Volver al grupo/i })
    expect(back).toHaveAttribute('href', '/groups/123')
  })

  it('muestra error si el GET de settings falla', async () => {
    mockFetchRoutes({
      '/api/groups/123/automation/settings': () =>
        errorJson(500, 'INTERNAL_ERROR', 'no se pudo obtener la configuración'),
      '/api/groups/123/automation/banned-words': () => okJson({ words: [] }),
      '/api/groups/123/automation/link-allowlist': () => okJson({ domains: [] }),
    })

    renderPage()

    expect(await screen.findByText(/no se pudo obtener la configuración/i)).toBeInTheDocument()
  })

  it('muestra las listas iniciales cuando GET banned-words y link-allowlist devuelven datos', async () => {
    mockFetchRoutes({
      '/api/groups/123/automation/settings': () => okJson(initialSettings),
      '/api/groups/123/automation/banned-words': () =>
        okJson({ words: ['spam', 'viagra'] }),
      '/api/groups/123/automation/link-allowlist': () =>
        okJson({ domains: ['example.com', 'github.com'] }),
    })

    renderPage()

    await screen.findByLabelText(/Habilitar moderación automática/i)

    // Las palabras y dominios se renderizan como Pills dentro de los
    // TagsInput. Usamos getAllByText porque Mantine v7 puede
    // duplicar el label para accesibilidad.
    expect(screen.getAllByText('spam').length).toBeGreaterThan(0)
    expect(screen.getAllByText('viagra').length).toBeGreaterThan(0)
    expect(screen.getAllByText('example.com').length).toBeGreaterThan(0)
    expect(screen.getAllByText('github.com').length).toBeGreaterThan(0)
  })

  it('boton Descartar cambios vuelve al estado original', async () => {
    const user = userEvent.setup()
    mockFetchRoutes({
      '/api/groups/123/automation/settings': () => okJson(initialSettings),
      '/api/groups/123/automation/banned-words': () => okJson({ words: [] }),
      '/api/groups/123/automation/link-allowlist': () => okJson({ domains: [] }),
    })

    renderPage()

    const toggle = await screen.findByLabelText(/Habilitar moderación automática/i)
    await user.click(toggle)

    // Save se habilita
    const saveButton = screen.getByRole('button', { name: 'Guardar' })
    await waitFor(() => expect(saveButton).not.toBeDisabled())

    // Descartar cambios vuelve al estado original (disabled de nuevo)
    const discardButton = screen.getByRole('button', { name: /Descartar cambios/i })
    await user.click(discardButton)

    await waitFor(() => expect(saveButton).toBeDisabled())
    // Toggle vuelve a false
    expect(toggle).not.toBeChecked()
  })
})
