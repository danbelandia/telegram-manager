// Boton de dark/light mode en el header (frontend-refresh slice 1 —
// design D5). Usa el singleton del provider: el toggle cambia el atributo
// `data-mantine-color-scheme` en `:root` y el `localStorageColorSchemeManager`
// persiste la eleccion con la clave `mantine-color-scheme-value` (lock
// de spec REQ-2). Default del provider es `auto` → respeta el SO.
import { ActionIcon, useMantineColorScheme } from '@mantine/core'
import { IconSun, IconMoon } from '@tabler/icons-react'

export default function ColorSchemeToggle() {
  const { colorScheme, toggleColorScheme } = useMantineColorScheme()
  const isDark = colorScheme === 'dark'

  return (
    <ActionIcon
      onClick={toggleColorScheme}
      variant="default"
      size="lg"
      aria-label={isDark ? 'Cambiar a esquema claro' : 'Cambiar a esquema oscuro'}
      data-testid="color-scheme-toggle"
    >
      {isDark ? <IconSun size={18} /> : <IconMoon size={18} />}
    </ActionIcon>
  )
}