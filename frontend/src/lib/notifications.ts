// Helpers de notificación (frontend-refresh slice 1 — design D9).
// Centralizan el formato para que cambiar copy/icon/color sea 1-line.
// Slice 1 los usa solo en LoginPage + Logout; slice 2 los conecta a los
// ~15 hooks restantes usando el mismo patron (nota en proposal §Scope).
import { notifications } from '@mantine/notifications'

export function notifySuccess(message: string): void {
  notifications.show({ color: 'green', message })
}

export function notifyError(message: string, title?: string): void {
  notifications.show({ color: 'red', title, message })
}