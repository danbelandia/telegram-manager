// Tests del GroupUsersPage (spec frontend-moderation req 2-3): lista de
// administradores con estados, lookup puntual y acciones con
// confirmacion (confirm=true ejecuta, confirm=false no llama a fetch).
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupUsersPage from './GroupUsersPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'

const admins = [
  {
    user_id: 42,
    first_name: 'Juan',
    username: 'juanito',
    status: 'administrator',
    can_restrict_members: true,
    can_delete_messages: true,
    can_pin_messages: false,
    can_invite_users: false,
  },
  {
    user_id: 7,
    first_name: 'Ana',
    username: null,
    status: 'creator',
    can_restrict_members: true,
    can_delete_messages: true,
    can_pin_messages: true,
    can_invite_users: true,
  },
]

function renderUsers() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/groups/123/users']}>
        <Routes>
          <Route path="/groups/:id/users" element={<GroupUsersPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('GroupUsersPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('lista los administradores del grupo', async () => {
    mockFetchRoutes({
      '/api/groups/123/users?userId=': async () => {
        throw new Error('no debe llamarse')
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    expect(await screen.findByText('Juan')).toBeInTheDocument()
    expect(screen.getByText('@juanito')).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()
    expect(screen.getByText('creator')).toBeInTheDocument()
  })

  it('muestra estado vacio con la nota de limitacion de la Bot API', async () => {
    mockFetchRoutes({ '/api/groups/123/users': () => okJson([]) })

    renderUsers()

    expect(await screen.findByText(/Sin administradores visibles/i)).toBeInTheDocument()
  })

  it('muestra error de carga y permite reintentar', async () => {
    let calls = 0
    mockFetchRoutes({
      '/api/groups/123/users': () => {
        calls += 1
        if (calls === 1) {
          return errorJson(403, 'PERMISSION_DENIED', 'el bot no tiene permisos suficientes en este grupo')
        }
        return okJson(admins)
      },
    })

    renderUsers()

    expect(await screen.findByText(/el bot no tiene permisos suficientes/i)).toBeInTheDocument()
    screen.getByRole('button', { name: 'Reintentar' }).click()
    expect(await screen.findByText('Juan')).toBeInTheDocument()
  })

  it('banea un usuario cuando el admin confirma', async () => {
    const confirmSpy = vi.fn(() => true)
    window.confirm = confirmSpy
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/users/42/ban': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const banButton = (await screen.findByText('Juan')).closest('.group-card')!.querySelector('button')!
    banButton.click()

    expect(confirmSpy).toHaveBeenCalled()
    await waitFor(() => expect(spy).toHaveBeenCalled())
  })

  it('no llama a la API si el admin cancela la confirmacion', async () => {
    window.confirm = vi.fn(() => false)
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/users/42/ban': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const banButton = (await screen.findByText('Juan')).closest('.group-card')!.querySelector('button')!
    banButton.click()

    expect(spy).not.toHaveBeenCalled()
  })

  it('hace lookup puntual por userId', async () => {
    mockFetchRoutes({
      '/api/groups/123/users?userId=99': () =>
        okJson({
          user_id: 99,
          first_name: 'Pedro',
          username: 'pedrin',
          status: 'member',
          can_restrict_members: null,
          can_delete_messages: null,
          can_pin_messages: null,
          can_invite_users: null,
        }),
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const input = screen.getByPlaceholderText(/ID de Telegram del usuario/i)
    await userEvent.type(input, '99')
    screen.getByRole('button', { name: 'Buscar' }).click()

    expect(await screen.findByText('Pedro')).toBeInTheDocument()
  })
})