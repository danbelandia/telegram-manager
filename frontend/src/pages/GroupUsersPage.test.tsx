// Tests del GroupUsersPage (spec frontend-pages-moderation REQ-1..3):
// render de Cards en SimpleGrid, lookup puntual, accion destructiva ban
// confirmada con <Modal> en lugar de window.confirm. Wrapper compartido
// con Mantine + Notifications (frontend-refresh slice 1).
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupUsersPage from './GroupUsersPage'
import { errorJson, mockFetchRoutes, okJson, renderWithProviders } from '../test/helpers'

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
  // renderWithProviders ya envuelve con MemoryRouter. Registramos la
  // ruta `/groups/:id/users` para que useParams resuelva :id desde la URL.
  return renderWithProviders(
    <Routes>
      <Route path="/groups/:id/users" element={<GroupUsersPage />} />
    </Routes>,
    ['/groups/123/users'],
  )
}

describe('GroupUsersPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('reencuadra la seccion con el titulo y la nota de alcance', async () => {
    mockFetchRoutes({ '/api/groups/123/users': () => okJson(admins) })

    renderUsers()

    expect(await screen.findByRole('heading', { name: 'Membresía y moderación' })).toBeInTheDocument()
    expect(
      screen.getByText(/Telegram no expone la lista completa de miembros/i),
    ).toBeInTheDocument()
  })

  it('lista los administradores del grupo', async () => {
    mockFetchRoutes({
      '/api/groups/123/users?userId=': async () => {
        throw new Error('no debe llamarse')
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    // Espera que las dos Cards aparezcan (por data-testid).
    const cards = await screen.findAllByTestId('user-card')
    expect(cards).toHaveLength(2)
    expect(screen.getByText('Juan')).toBeInTheDocument()
    // El username se renderiza dentro de un Text con varias partes
    // ("ID: 42 · @juanito"), asi que usamos regex.
    expect(screen.getByText(/@juanito/)).toBeInTheDocument()
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

  it('abre el Modal de confirmacion al hacer clic en Banear', async () => {
    const user = userEvent.setup()
    mockFetchRoutes({ '/api/groups/123/users': () => okJson(admins) })

    renderUsers()

    const cards = await screen.findAllByTestId('user-card')
    const juanCard = cards.find((c) => c.textContent?.includes('Juan'))!
    const banButton = juanCard.querySelector('button')! // primer button del card = Banear
    await user.click(banButton)

    expect(await screen.findByRole('heading', { name: 'Confirmar baneo' })).toBeInTheDocument()
    expect(screen.getByText(/¿Banear a Juan\? Esta acción es irreversible\./i)).toBeInTheDocument()
  })

  it('banea un usuario cuando el admin confirma desde el Modal', async () => {
    const user = userEvent.setup()
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/users/42/ban': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const cards = await screen.findAllByTestId('user-card')
    const juanCard = cards.find((c) => c.textContent?.includes('Juan'))!
    await user.click(juanCard.querySelector('button')!) // Banear (primer button)

    expect(await screen.findByRole('heading', { name: 'Confirmar baneo' })).toBeInTheDocument()
    await user.click(screen.getByTestId('confirm-ban'))

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(await screen.findByText('Usuario baneado')).toBeInTheDocument()
  })

  it('no llama a la API si el admin cancela la confirmacion', async () => {
    const user = userEvent.setup()
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/users/42/ban': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const cards = await screen.findAllByTestId('user-card')
    const juanCard = cards.find((c) => c.textContent?.includes('Juan'))!
    await user.click(juanCard.querySelector('button')!)

    expect(await screen.findByRole('heading', { name: 'Confirmar baneo' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancelar' }))

    // El Modal se cerro (heading fuera del DOM).
    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Confirmar baneo' })).not.toBeInTheDocument()
    })
    expect(spy).not.toHaveBeenCalled()
  })

  it('hace lookup puntual por userId', async () => {
    const user = userEvent.setup()
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
    await user.type(input, '99')
    await user.click(screen.getByRole('button', { name: 'Buscar' }))

    expect(await screen.findByText('Pedro')).toBeInTheDocument()
  })

  it('mutea un usuario con notification directa (sin Modal)', async () => {
    const user = userEvent.setup()
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/users/42/mute': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/users': () => okJson(admins),
    })

    renderUsers()

    const cards = await screen.findAllByTestId('user-card')
    const juanCard = cards.find((c) => c.textContent?.includes('Juan'))!
    await user.click(juanCard.querySelectorAll('button')[1]!) // Mutear (segundo button)

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(await screen.findByText('Usuario muteado')).toBeInTheDocument()
  })
})