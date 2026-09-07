// Entry point del panel. Importa los CSS de Mantine ANTES del arbol
// React para que las variables `--mantine-*` existan en el primer paint
// (sin flicker en dark mode, design D5). El orden de providers es:
//   StrictMode > QueryClientProvider > BrowserRouter > AuthProvider >
//   MantineProvider > Notifications > App
// Notifications se monta UNA sola vez aca; los helpers
// `notifySuccess/notifyError` viven en lib/notifications.ts y delegan
// al singleton del provider (frontend-refresh slice 1).
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'

import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import './styles.css'

import App from './App'
import { AuthProvider } from './lib/auth-context'
import { colorSchemeManager, mantineTheme } from './theme'

// Un QueryClient por app: los hooks de datos (features/*/hooks.ts) no
// configuran cliente propio, dependen de este provider (guia seccion 4).
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, staleTime: 30_000 },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider
      theme={mantineTheme}
      defaultColorScheme="auto"
      colorSchemeManager={colorSchemeManager}
    >
      <Notifications position="top-right" zIndex={2077} limit={5} />
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthProvider>
            <App />
          </AuthProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </MantineProvider>
  </StrictMode>,
)