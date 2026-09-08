// Tests de LoginPage (spec frontend-auth): validacion Zod bloquea el
// envio vacio, submit llama a login, error legible se muestra.
// mockFetchRoutes (by URL) evita desalinear mocks con el refresh
// automatico del api-client.
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import LoginPage from './LoginPage'
import { AuthProvider } from '../lib/auth-context'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

// state de entrada para /login: permite simular redirect post-login
const ROUTE_WITH_STATE: { pathname: string; state: { from?: string } } = {
  pathname: '/login',
  state: { from: '/groups/123' },
}

function renderLogin(initialEntries: Array<string | { pathname: string; state: unknown }> = ['/login']) {
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <MemoryRouter initialEntries={initialEntries}>
        <AuthProvider>
          <LoginPage />
        </AuthProvider>
      </MemoryRouter>
    </MantineProvider>,
  )
}

describe('LoginPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('bloquea el envio con campos vacios (validacion cliente)', async () => {
    mockFetchRoutes({})

    renderLogin()
    fireEvent.click(await screen.findByRole('button', { name: 'Ingresar' }))

    expect(await screen.findByText('El usuario es obligatorio')).toBeInTheDocument()
    expect(await screen.findByText('La contraseña es obligatoria')).toBeInTheDocument()
  })

  it('muestra el mensaje de error del backend con credenciales invalidas', async () => {
    mockFetchRoutes({
      '/api/auth/login': () => errorJson(401, 'INVALID_CREDENTIALS', 'credenciales invalidas'),
    })

    renderLogin()

    fireEvent.change(await screen.findByLabelText('Usuario'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'mal' } })
    fireEvent.click(screen.getByRole('button', { name: 'Ingresar' }))

    expect(await screen.findByText('credenciales invalidas')).toBeInTheDocument()
  })

  it('loguea y navega a la ruta original (redirect post-login)', async () => {
    let meCalls = 0
    mockFetchRoutes({
      '/api/auth/login': () => okJson({ access_token: 'token-nuevo' }),
      // al montar: sin sesion (401). tras login: identidad.
      '/api/auth/me': () => {
        meCalls += 1
        if (meCalls === 1) return errorJson(401, 'UNAUTHORIZED', 'no autenticado')
        return okJson({ id: '1', username: 'admin' })
      },
    })

    renderLogin([ROUTE_WITH_STATE])

    fireEvent.change(await screen.findByLabelText('Usuario'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'secret' } })
    fireEvent.click(screen.getByRole('button', { name: 'Ingresar' }))

    // Tras login exitoso ya no esta el boton de login (se navego fuera).
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Ingresar' })).not.toBeInTheDocument())
  })

  it('pre-rellena el usuario y avisa con ?username=&created=1 (degradacion del signup)', async () => {
    mockFetchRoutes({
      '/api/auth/me': () => errorJson(401, 'UNAUTHORIZED', 'no autenticado'),
      '/api/auth/refresh': () => errorJson(401, 'UNAUTHORIZED', 'refresh invalido'),
    })

    renderLogin(['/login?username=juan&created=1'])

    expect(await screen.findByDisplayValue('juan')).toBeInTheDocument()
    expect(await screen.findByText('Cuenta creada, iniciá sesión')).toBeInTheDocument()
  })
})