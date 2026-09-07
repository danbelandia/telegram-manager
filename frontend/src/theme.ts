// Theme central de Mantine (frontend-refresh slice 1 — design D4/D5).
// `primaryColor: 'blue'` matchea el visual cercano al `--primary: #3b82f6`
// previo usando la paleta built-in (`#228be6`) — no se customiza paleta
// para evitar scope creep. `defaultRadius: 'md'` respeta la densidad
// visual del CSS plano anterior. El `colorSchemeManager` persiste en
// localStorage con la clave `mantine-color-scheme-value` (lockeada por
// spec REQ-2 + escenario "Persistencia del color scheme").
import { createTheme, localStorageColorSchemeManager } from '@mantine/core'

export const mantineTheme = createTheme({
  primaryColor: 'blue',
  defaultRadius: 'md',
  fontFamily:
    '-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica, Arial, sans-serif, Apple Color Emoji, Segoe UI Emoji',
})

export const colorSchemeManager = localStorageColorSchemeManager({
  key: 'mantine-color-scheme-value',
})