import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import TenantSettingsPage from './TenantSettingsPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

function renderPage() {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <MemoryRouter>
        <TenantSettingsPage />
      </MemoryRouter>
    </MantineProvider>,
  )
}

const tenantData = {
  slug: 'my-tenant',
  bot_username: 'TestBot',
  bot_status: 'connected',
  created_at: '2026-01-15T00:00:00Z',
}

describe('TenantSettingsPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('muestra datos del tenant en modo readonly', async () => {
    mockFetchRoutes({
      '/api/tenants/me': () => okJson(tenantData),
    })

    renderPage()

    expect(await screen.findByText('my-tenant')).toBeInTheDocument()
    expect(screen.getByText('TestBot')).toBeInTheDocument()
    expect(screen.getByText('Conectado')).toBeInTheDocument()
  })

  it('muestra formulario de rotacion', async () => {
    mockFetchRoutes({
      '/api/tenants/me': () => okJson(tenantData),
    })

    renderPage()

    await screen.findByText('my-tenant')
    expect(screen.getByLabelText('Password actual')).toBeInTheDocument()
    expect(screen.getByLabelText('Nuevo Bot Token')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rotar Token' })).toBeInTheDocument()
  })

  it('muestra error cuando falla la carga', async () => {
    mockFetchRoutes({
      '/api/tenants/me': () => errorJson(500, 'INTERNAL_ERROR', 'Error del servidor'),
    })

    renderPage()

    expect(await screen.findByText('Error del servidor')).toBeInTheDocument()
  })

  it('envia formulario y llama a la API correctamente', async () => {
    const user = userEvent.setup()
    const fetchCalls: string[] = []
    const origFetch = window.fetch
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      fetchCalls.push(`${init?.method ?? 'GET'} ${url}`)
      if (url.includes('/api/tenants/me/bot-token')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: { status: 'rotated' }, error: null }) })
      }
      if (url.includes('/api/tenants/me')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: tenantData, error: null }) })
      }
      return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }) })
    }))

    renderPage()

    await screen.findByText('my-tenant')

    await user.type(screen.getByLabelText('Password actual'), 'mypassword')
    await user.type(screen.getByLabelText('Nuevo Bot Token'), '123456:ABC')
    await user.click(screen.getByRole('button', { name: 'Rotar Token' }))

    // Verify the PUT was dispatched
    await waitFor(() => {
      expect(fetchCalls.some(c => c.includes('PUT /api/tenants/me/bot-token'))).toBe(true)
    })

    window.fetch = origFetch
  })

  it('no envia cuando la password esta vacia (validacion Zod)', async () => {
    const user = userEvent.setup()
    const fetchCalls: string[] = []
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      fetchCalls.push(`${init?.method ?? 'GET'} ${url}`)
      if (url.includes('/api/tenants/me')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: tenantData, error: null }) })
      }
      return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }) })
    }))

    renderPage()

    await screen.findByText('my-tenant')

    // Only fill bot_token, leave password empty
    await user.type(screen.getByLabelText('Nuevo Bot Token'), '123456:ABC')
    await user.click(screen.getByRole('button', { name: 'Rotar Token' }))

    // Zod validation should prevent the PUT call
    await vi.waitFor(() => {
      expect(fetchCalls.some(c => c.includes('PUT'))).toBe(false)
    })
  })

  it('maneja error 401 sin crashear', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/tenants/me/bot-token')) {
        return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'Password incorrecta' } }) })
      }
      if (url.includes('/api/tenants/me')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: tenantData, error: null }) })
      }
      return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }) })
    }))

    renderPage()

    await screen.findByText('my-tenant')

    await user.type(screen.getByLabelText('Password actual'), 'wrong')
    await user.type(screen.getByLabelText('Nuevo Bot Token'), '123456:ABC')
    await user.click(screen.getByRole('button', { name: 'Rotar Token' }))

    // After error, form should still be visible (no crash)
    await waitFor(() => {
      expect(screen.getByText('Rotar Bot Token')).toBeInTheDocument()
    })
  })

  it('maneja error 502 sin crashear', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/tenants/me/bot-token')) {
        return Promise.resolve({ ok: false, status: 502, json: () => Promise.resolve({ data: null, error: { code: 'TELEGRAM_ERROR', message: 'Telegram rechazo el bot token' } }) })
      }
      if (url.includes('/api/tenants/me')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: tenantData, error: null }) })
      }
      return Promise.resolve({ ok: false, status: 401, json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no autenticado' } }) })
    }))

    renderPage()

    await screen.findByText('my-tenant')

    await user.type(screen.getByLabelText('Password actual'), 'correct')
    await user.type(screen.getByLabelText('Nuevo Bot Token'), 'invalid')
    await user.click(screen.getByRole('button', { name: 'Rotar Token' }))

    // After error, form should still be visible (no crash)
    await waitFor(() => {
      expect(screen.getByText('Rotar Bot Token')).toBeInTheDocument()
    })
  })
})
