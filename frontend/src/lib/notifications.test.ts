// Smoke test de los helpers de notificacion (frontend-refresh slice 1 —
// design D17). Verifica que notifySuccess/notifyError delegan a
// `notifications.show` del provider de Mantine. El spy se monta sobre
// el modulo importado por lib/notifications.
import { afterEach, describe, expect, it, vi } from 'vitest'
import { notifications } from '@mantine/notifications'
import { notifyError, notifySuccess } from './notifications'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('notifications helpers', () => {
  it('notifySuccess delega en notifications.show con color green', () => {
    const spy = vi.spyOn(notifications, 'show').mockImplementation(() => 'mock-id')

    notifySuccess('Bienvenido')

    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith({ color: 'green', message: 'Bienvenido' })
  })

  it('notifyError delega en notifications.show con color red y title opcional', () => {
    const spy = vi.spyOn(notifications, 'show').mockImplementation(() => 'mock-id')

    notifyError('Boom', 'Error grave')

    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith({
      color: 'red',
      title: 'Error grave',
      message: 'Boom',
    })
  })

  it('notifyError sin title funciona (parametro opcional)', () => {
    const spy = vi.spyOn(notifications, 'show').mockImplementation(() => 'mock-id')

    notifyError('Solo mensaje')

    expect(spy).toHaveBeenCalledWith({
      color: 'red',
      title: undefined,
      message: 'Solo mensaje',
    })
  })
})