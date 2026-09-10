import { render, screen, waitFor } from '@testing-library/react'
import { MantineProvider } from '@mantine/core'
import { afterEach, describe, expect, it, vi } from 'vitest'
import DegradedBanner from './DegradedBanner'
import { mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

function renderBanner() {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <DegradedBanner />
    </MantineProvider>,
  )
}

describe('DegradedBanner', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('no se muestra cuando el bot esta conectado', async () => {
    mockFetchRoutes({
      '/api/tenants/me/status': () => okJson({ bot_status: 'connected' }),
    })

    renderBanner()

    await waitFor(() => {
      expect(screen.queryByTestId('degraded-banner')).not.toBeInTheDocument()
    })
  })

  it('se muestra cuando el bot esta desconectado', async () => {
    mockFetchRoutes({
      '/api/tenants/me/status': () => okJson({ bot_status: 'disconnected' }),
    })

    renderBanner()

    expect(await screen.findByTestId('degraded-banner')).toBeInTheDocument()
    expect(screen.getByText(/bot esta desconectado/)).toBeInTheDocument()
  })

  it('se muestra cuando el estado es unknown', async () => {
    mockFetchRoutes({
      '/api/tenants/me/status': () => okJson({ bot_status: 'unknown' }),
    })

    renderBanner()

    expect(await screen.findByTestId('degraded-banner')).toBeInTheDocument()
    expect(screen.getByText(/Estado del bot desconocido/)).toBeInTheDocument()
  })
})
