// Wrapper de rutas protegidas (guia frontend seccion 8): sin sesion
// valida redirige a /login recordando la ruta intentada (spec
// frontend-routing, redirect post-login). Con sesion o mientras se
// restaura, renderiza el children. No consulta permisos de Telegram —
// solo el estado de sesion local.
import { Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../lib/auth-context'

export default function RequireAuth({ children }: { children: React.ReactNode }) {
  const { loading, user } = useAuth()
  const location = useLocation()

  if (loading) {
    return null // breve espera mientras /me resuelve; evita flash de login
  }

  if (!user) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return <>{children}</>
}