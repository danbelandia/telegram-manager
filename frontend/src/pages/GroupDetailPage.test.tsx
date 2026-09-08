// Tests del GroupDetailPage (spec frontend-dashboard req 3 +
// frontend-moderation req 4-5): detalle del grupo, acciones de chat
// lock/unlock con confirmacion y delete/pin por messageId (confirm para
// delete, cancelado no llama a la API). Wrapper incluye MantineProvider
// + Notifications porque el componente migrado usa Tabs / Stack /
// TextInput / Skeleton de Mantine v7 (frontend-refresh slice 1).
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { afterEach, describe, expect, it, vi } from 'vitest'
import GroupDetailPage from './GroupDetailPage'
import { errorJson, mockFetchRoutes, okJson } from '../test/helpers'
import { mantineTheme } from '../theme'

const group = {
  id: 'g123',
  telegram_id: -100123,
  title: 'MU Online Comunidad',
  username: null,
  type: 'supergroup',
  member_count: 4821,
  bot_status: 'administrator',
  bot_permissions: {
    can_restrict_members: true,
    can_delete_messages: true,
    can_pin_messages: true,
    can_invite_users: true,
    can_change_info: true,
  },
}

function renderDetail(entries: string[] = ['/groups/123']) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <MantineProvider theme={mantineTheme} defaultColorScheme="light">
      <Notifications position="top-right" />
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={entries}>
          <Routes>
            <Route path="/groups/:id" element={<GroupDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </MantineProvider>,
  )
}

describe('GroupDetailPage', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('muestra el detalle del grupo y sus permisos', async () => {
    mockFetchRoutes({
      '/api/groups/123': () => okJson(group),
    })

    renderDetail()

    expect(await screen.findByText('MU Online Comunidad')).toBeInTheDocument()
    expect(screen.getByText('-100123')).toBeInTheDocument()
    expect(screen.getByText('4.821')).toBeInTheDocument()
    // Permisos legibles en es-AR (la API los manda como claves can_*).
    expect(
      screen.getByText(
        'Restringir miembros, Eliminar mensajes, Fijar mensajes, Invitar usuarios, Cambiar la información del grupo',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Membresía y moderación' })).toHaveAttribute(
      'href',
      '/groups/123/users',
    )
    // Slice 2 (moderacion automatica): link al editor de settings + listas.
    // Slice 3 (Fase 3): label renombrado + nuevo link al dashboard.
    expect(screen.getByTestId('automation-link')).toHaveAttribute(
      'href',
      '/groups/123/automation',
    )
    expect(screen.getByTestId('automation-link')).toHaveTextContent(
      'Configurar reglas de moderación',
    )
    expect(screen.getByTestId('moderation-dashboard-link')).toHaveAttribute(
      'href',
      '/groups/123/moderation',
    )
    expect(screen.getByTestId('moderation-dashboard-link')).toHaveTextContent(
      'Ver dashboard de moderación',
    )
  })

  it('muestra grupo inexistente cuando el backend responde NOT_FOUND', async () => {
    mockFetchRoutes({
      '/api/groups/123': () => errorJson(404, 'NOT_FOUND', 'Grupo no encontrado'),
    })

    renderDetail()

    expect(await screen.findByText(/Grupo no encontrado/i)).toBeInTheDocument()
  })

  it('cierra el chat cuando el admin confirma', async () => {
    window.confirm = vi.fn(() => true)
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/lock': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/logs': () => okJson([]),
      '/api/groups/123': () => okJson(group),
    })

    renderDetail()

    const lockButton = await screen.findByRole('button', { name: /Cerrar chat/i })
    lockButton.click()

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(window.confirm).toHaveBeenCalled()
  })

  it('elimina un mensaje por id cuando el admin confirma', async () => {
    window.confirm = vi.fn(() => true)
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123/messages/987/delete': () => {
        spy()
        return okJson({ status: 'ok' })
      },
      '/api/groups/123/logs': () => okJson([]),
      '/api/groups/123': () => okJson(group),
    })

    renderDetail()

    const input = await screen.findByPlaceholderText(/ID del mensaje/i)
    await userEvent.type(input, '987')

    const deleteButton = screen.getByRole('button', { name: 'Eliminar' })
    deleteButton.click()

    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(window.confirm).toHaveBeenCalled()
  })

  it('no llama a la API si el admin cancela el borrado', async () => {
    window.confirm = vi.fn(() => false)
    const spy = vi.fn()
    mockFetchRoutes({
      '/api/groups/123': () => okJson(group),
      '/api/groups/123/messages/987/delete': () => {
        spy()
        return okJson({ status: 'ok' })
      },
    })

    renderDetail()

    const input = await screen.findByPlaceholderText(/ID del mensaje/i)
    await userEvent.type(input, '987')

    const deleteButton = screen.getByRole('button', { name: 'Eliminar' })
    deleteButton.click()

    expect(spy).not.toHaveBeenCalled()
  })
})