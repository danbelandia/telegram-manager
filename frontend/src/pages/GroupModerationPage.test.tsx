// Tests del GroupModerationPage (Fase 3, slice 3 — Warnings Dashboard).
// Cubre render inicial con stats + warnings, period selector 7d dispara
// nueva query, empty state, truncation alert, refresh button invalida
// 2 queries, reset modal abre/cancela/confirma, display name fallback
// a "user {id}", y error del backend muestra notifyError.
//
// Mocking: mockFetchRoutes del test/helpers.tsx dispatcha por substring
// de URL; como las queries son GET /warnings y GET /stats?period=24h
// (substring "warnings" y "stats"), las atrapamos asi. Los resets
// (POST /warnings/{user_id}/reset) se mockean por substring "reset".
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupModerationPage from './GroupModerationPage'
import { mockFetchRoutes, okJson, renderWithProviders } from '../test/helpers'

const groupId = 123

const sampleWarnings = {
  warnings: [
    {
      user_id: 1,
      display_name: 'Ana',
      username: 'ana_p',
      warning_count: 3,
      last_warning_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
      last_action_at: null,
      expires_at: null,
    },
    {
      user_id: 2,
      display_name: 'user 2',
      username: null,
      warning_count: 1,
      last_warning_at: null,
      last_action_at: null,
      expires_at: null,
    },
  ],
  truncated: false,
}

const sampleStats = {
  rule_triggered: 12,
  automute: 3,
  autoban: 1,
  period: '24h',
}

function renderPage(initial: string[] = [`/groups/${groupId}/moderation`]) {
  return renderWithProviders(
    <Routes>
      <Route path="/groups/:id/moderation" element={<GroupModerationPage />} />
    </Routes>,
    initial,
  )
}

describe('GroupModerationPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renderiza stats y tabla con los datos del backend', async () => {
    mockFetchRoutes({
      '/automation/warnings': () => okJson(sampleWarnings),
      '/automation/stats': () => okJson(sampleStats),
    })

    renderPage()

    // Esperar a que el query de stats resuelva usando el testId del
    // card (que ya esta en el DOM durante loading, pero el dato se
    // monta dentro del card una vez resuelta la query). Usamos
    // waitFor para esperar el contenido "12" en el card especifico.
    const ruleCard = await screen.findByTestId('stats-card-rule-triggered')
    await waitFor(() => expect(ruleCard).toHaveTextContent('12'))
    expect(screen.getByTestId('stats-card-automute')).toHaveTextContent('3')
    expect(screen.getByTestId('stats-card-autoban')).toHaveTextContent('1')
    // Labels presentes (3 cards).
    expect(screen.getByText('Reglas disparadas')).toBeInTheDocument()
    expect(screen.getByText('Auto-mute')).toBeInTheDocument()
    expect(screen.getByText('Auto-ban')).toBeInTheDocument()

    // Tabla con 2 filas.
    expect(await screen.findByTestId('warning-row-1')).toBeInTheDocument()
    expect(screen.getByTestId('warning-row-2')).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()
    expect(screen.getByText('user 2')).toBeInTheDocument()
    // Botones Reset presentes.
    expect(screen.getByTestId('warning-reset-1')).toBeInTheDocument()
    expect(screen.getByTestId('warning-reset-2')).toBeInTheDocument()
  })

  it('cambia el period selector a 7d dispara nueva query', async () => {
    const user = userEvent.setup()
    let stats24hCalls = 0
    let stats7dCalls = 0
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/automation/stats?period=24h')) {
        stats24hCalls++
        return okJson({ ...sampleStats, period: '24h' })
      }
      if (url.includes('/automation/stats?period=7d')) {
        stats7dCalls++
        return okJson({ ...sampleStats, rule_triggered: 100, period: '7d' })
      }
      if (url.includes('/automation/warnings')) {
        return okJson(sampleWarnings)
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no auth' } }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByTestId('stats-card-rule-triggered')
    expect(stats24hCalls).toBe(1)

    // Cambiar el Select.
    const select = screen.getByTestId('stats-period-select')
    await user.click(select)
    // Mantine v7 Select: las opciones son botones con role="option".
    const option7d = await screen.findByRole('option', { name: /Últimos 7 días/i })
    await user.click(option7d)

    await waitFor(() => expect(stats7dCalls).toBe(1))
    expect(stats24hCalls).toBe(1)
  })

  it('muestra empty state cuando no hay advertencias activas', async () => {
    mockFetchRoutes({
      '/automation/warnings': () => okJson({ warnings: [], truncated: false }),
      '/automation/stats': () => okJson(sampleStats),
    })

    renderPage()

    expect(await screen.findByTestId('warnings-empty-state')).toHaveTextContent(
      'No hay advertencias activas',
    )
  })

  it('muestra Alert de truncation cuando hay mas de 100 activas', async () => {
    const bigList = {
      warnings: Array.from({ length: 100 }, (_, i) => ({
        user_id: i + 1,
        display_name: `user ${i + 1}`,
        username: null,
        warning_count: 1,
        last_warning_at: null,
        last_action_at: null,
        expires_at: null,
      })),
      truncated: true,
    }
    mockFetchRoutes({
      '/automation/warnings': () => okJson(bigList),
      '/automation/stats': () => okJson(sampleStats),
    })

    renderPage()

    expect(await screen.findByTestId('warnings-truncated-alert')).toHaveTextContent(
      'primeras 100 advertencias activas',
    )
  })

  it('boton Refrescar invalida ambas queries', async () => {
    let warningsCalls = 0
    let statsCalls = 0
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/automation/warnings')) {
        warningsCalls++
        return okJson(sampleWarnings)
      }
      if (url.includes('/automation/stats')) {
        statsCalls++
        return okJson(sampleStats)
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no auth' } }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    await screen.findByTestId('warning-row-1')
    expect(warningsCalls).toBe(1)
    expect(statsCalls).toBe(1)

    // Refrescar → ambas queries se vuelven a llamar.
    const refreshBtn = screen.getByTestId('stats-refresh-button')
    refreshBtn.click()

    await waitFor(() => {
      expect(warningsCalls).toBeGreaterThanOrEqual(2)
      expect(statsCalls).toBeGreaterThanOrEqual(2)
    })
  })

  it('abre el Modal de confirm al click en Reset y cancela sin disparar POST', async () => {
    const user = userEvent.setup()
    let resetCalls = 0
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/warnings/1/reset')) {
        resetCalls++
        return okJson({ user_id: 1, warning_count: 0, reset: true })
      }
      if (url.includes('/automation/warnings')) {
        return okJson(sampleWarnings)
      }
      if (url.includes('/automation/stats')) {
        return okJson(sampleStats)
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no auth' } }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const resetBtn = await screen.findByTestId('warning-reset-1')
    resetBtn.click()

    // Modal abierto: data-testid aparece + body del Modal visible (la
    // presencia de los botones Cancelar/Resetear confirma que el
    // body esta montado con resetTarget != null). El texto del body
    // ("Resetear las advertencias de Ana", "No desmutea") se fragmenta
    // por los <strong> wrappers; no es estable de matchear via
    // getByText — los buttons son anclas mas confiables. Mantine v7
    // Modal tiene open animation (~150ms); usamos findByTestId async
    // para esperar el render del contenido. Nota: el Modal usa Portal
    // que monta los buttons fuera del data-testid root, asi que
    // usamos screen (no within) para las busquedas.
    expect(await screen.findByTestId('reset-warning-modal')).toBeInTheDocument()
    expect(await screen.findByTestId('reset-warning-cancel')).toBeInTheDocument()
    expect(await screen.findByTestId('reset-warning-confirm')).toBeInTheDocument()

    // Cancelar → cierra sin POST. Mantine v7 Modal tiene exit animation
    // (transitionProps default); usamos findBy para esperar a que
    // el modal desaparezca del DOM.
    await user.click(await screen.findByTestId('reset-warning-cancel'))
    // El modal cerrado: el body (texto "no desmutea") ya no esta.
    await waitFor(() => {
      expect(screen.queryByText(/no desmutea/i)).not.toBeInTheDocument()
    })
    expect(resetCalls).toBe(0)
  })

  it('confirma el reset y dispara POST + invalida cache', async () => {
    let resetCalls = 0
    let warningsCalls = 0
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/warnings/1/reset')) {
        resetCalls++
        return okJson({ user_id: 1, warning_count: 0, reset: true })
      }
      if (url.includes('/automation/warnings')) {
        warningsCalls++
        return okJson(sampleWarnings)
      }
      if (url.includes('/automation/stats')) {
        return okJson(sampleStats)
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no auth' } }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const resetBtn = await screen.findByTestId('warning-reset-1')
    resetBtn.click()
    const confirmBtn = await screen.findByTestId('reset-warning-confirm')
    confirmBtn.click()

    await waitFor(() => expect(resetCalls).toBe(1))
    // Cache invalidada → warnings refetch.
    await waitFor(() => expect(warningsCalls).toBeGreaterThanOrEqual(2))
  })

  it('muestra notifyError cuando el reset falla', async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/warnings/1/reset')) {
        return Promise.resolve({
          ok: false,
          status: 500,
          json: () =>
            Promise.resolve({
              data: null,
              error: { code: 'INTERNAL_ERROR', message: 'db explosion' },
            }),
        })
      }
      if (url.includes('/automation/warnings')) {
        return okJson(sampleWarnings)
      }
      if (url.includes('/automation/stats')) {
        return okJson(sampleStats)
      }
      return Promise.resolve({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ data: null, error: { code: 'UNAUTHORIZED', message: 'no auth' } }),
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage()

    const resetBtn = await screen.findByTestId('warning-reset-1')
    resetBtn.click()
    const confirmBtn = await screen.findByTestId('reset-warning-confirm')
    confirmBtn.click()

    // Mantine notifications.show se invoca; verificamos que aparece
    // el titulo "No se pudo resetear".
    expect(await screen.findByText(/No se pudo resetear/i)).toBeInTheDocument()
  })

  it('renderiza el fallback display_name=user {id} cuando username es null y display_name=user {id}', async () => {
    // (el backend computa "user {id}" como fallback; el frontend solo
    // lo renderea).
    mockFetchRoutes({
      '/automation/warnings': () =>
        okJson({
          warnings: [
            {
              user_id: 99,
              display_name: 'user 99',
              username: null,
              warning_count: 2,
              last_warning_at: null,
              last_action_at: null,
              expires_at: null,
            },
          ],
          truncated: false,
        }),
      '/automation/stats': () => okJson(sampleStats),
    })

    renderPage()

    expect(await screen.findByText('user 99')).toBeInTheDocument()
    expect(screen.getByTestId('warning-reset-99')).toBeInTheDocument()
  })
})