// Singleton de QueryClient (frontend-refresh): se exporta una unica
// instancia para que `main.tsx` la monte como `<QueryClientProvider>`
// y `auth-context` pueda limpiarla (`queryClient.clear()`) al cerrar
// sesion. Sin este clear, los queries cacheados del usuario anterior
// (groups, logs, etc.) se mostrarian al siguiente usuario hasta que
// cada query volviera a fetchear (staleTime: 30_000) — fuga de datos
// cross-tenant (slice 4 spec).
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, staleTime: 30_000 },
  },
})