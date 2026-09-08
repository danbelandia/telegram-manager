// Tests de SignupPage (spec frontend-auth: REQ signup-happy,
// signup-degraded, signup-409, signup-errors, hygiene). mockFetchRoutes
// (by URL) evita desalinear mocks con el refresh automatico del
// api-client (patron LoginPage.test.tsx). El token solo aparece como
// literal sintetico SIGNUP_BOT_TOKEN_EXAMPLE (REQ hygiene).
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useSearchParams } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Mock } from 'vitest'
import SignupPage from './SignupPage'
import { AuthProvider } from '../lib/auth-context'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

const FAKE_TOKEN = 'SIGNUP_BOT_TOKEN_EXAMPLE'

function fillValidForm() {
  fireEvent.change(screen.getByLabelText('Identificador del espacio'), { target: { value: 'acme' } })
  fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'juan' } })
  fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'secreto123' } })
  fireEvent.change(screen.getByLabelText('Token del bot'), { target: { value: FAKE_TOKEN } })
}

/** Sonda que expone los query params de /login para el test de degradacion. */
function LoginProbe() {
  const [params] = useSearchParams()
  return (
    <div>
      pagina login:{params.get('username') ?? ''}:{params.get('created') ?? ''}
    </div>
  )
}

function renderSignup() {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <MemoryRouter initialEntries={['/signup']}>
        <AuthProvider>
          <Routes>
            <Route path="/signup" element={<SignupPage />} />
            <Route path="/dashboard" element={<div>pagina dashboard</div>} />
            <Route path="/login" element={<LoginProbe />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>
    </MantineProvider>,
  )
}

function signupCalled(): boolean {
  const calls = (globalThis.fetch as Mock).mock.calls
  return calls.some((args) => String(args[0]).includes('/api/auth/signup'))
}

describe('SignupPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('la validacion cliente bloquea sin llamar a la API (password corto)', async () => {
    mockFetchRoutes({})
    renderSignup()

    fireEvent.change(await screen.findByLabelText('Identificador del espacio'), { target: { value: 'acme' } })
    fireEvent.change(screen.getByLabelText('Usuario'), { target: { value: 'juan' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'abc12' } })
    fireEvent.change(screen.getByLabelText('Token del bot'), { target: { value: FAKE_TOKEN } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('Mínimo 8 caracteres')).toBeInTheDocument()
    expect(signupCalled()).toBe(false)
  })

  it('signup 201 con auto-login va a /dashboard', async () => {
    let meCalls = 0
    mockFetchRoutes({
      '/api/auth/signup': () => okJson({ tenant: { id: 7, slug: 'acme' }, admin: { id: '1', username: 'juan' } }),
      '/api/auth/login': () => okJson({ access_token: 'jwt-nuevo' }),
      // al montar: sin sesion (401). tras login: identidad con tenant.
      '/api/auth/me': () => {
        meCalls += 1
        if (meCalls === 1) return errorJson(401, 'UNAUTHORIZED', 'no autenticado')
        return okJson({ id: '1', username: 'juan', tenant_id: 7, tenant_slug: 'acme' })
      },
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('pagina dashboard')).toBeInTheDocument()
    // Higiene: el campo del token quedo vacio tras el submit.
    expect(screen.queryByDisplayValue(FAKE_TOKEN)).not.toBeInTheDocument()
  })

  it('auto-login fallido degrada a /login?username=&created=1', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () => okJson({ tenant: { id: 7, slug: 'acme' }, admin: { id: '1', username: 'juan' } }),
      '/api/auth/login': () => errorJson(401, 'INVALID_CREDENTIALS', 'credenciales invalidas'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('pagina login:juan:1')).toBeInTheDocument()
    expect(screen.queryByDisplayValue(FAKE_TOKEN)).not.toBeInTheDocument()
  })

  it('409 que menciona slug muestra el error junto al campo slug', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () => errorJson(409, 'CONFLICT', 'el slug ya esta registrado'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('el slug ya esta registrado')).toBeInTheDocument()
    expect(screen.queryByDisplayValue(FAKE_TOKEN)).not.toBeInTheDocument()
  })

  it('409 que menciona username muestra el error junto al campo username', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () => errorJson(409, 'CONFLICT', 'el username ya esta en uso'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('el username ya esta en uso')).toBeInTheDocument()
  })

  it('409 generico muestra el error como mensaje general', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () => errorJson(409, 'CONFLICT', 'conflicto al crear el tenant'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    // findByText (no role=alert: las notificaciones globales de
    // Mantine tambien usan alert y colisionan entre tests).
    expect(await screen.findByText('conflicto al crear el tenant')).toBeInTheDocument()
  })

  it('400 de validacion va junto a su campo', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () =>
        errorJson(400, 'VALIDATION_ERROR', 'slug, username, password (minimo 8 caracteres) y bot_token son requeridos'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    // El mensaje nombra bot_token: cae junto al campo del token.
    await waitFor(() =>
      expect(screen.getByText('slug, username, password (minimo 8 caracteres) y bot_token son requeridos')).toBeInTheDocument(),
    )
  })

  it('502 de Telegram guia a @BotFather con reintento', async () => {
    mockFetchRoutes({
      '/api/auth/signup': () => errorJson(502, 'TELEGRAM_ERROR', 'Telegram rechazo el bot token'),
    })
    renderSignup()
    await screen.findByLabelText('Identificador del espacio')

    fillValidForm()
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText(/@BotFather/)).toBeInTheDocument()
  })
})
