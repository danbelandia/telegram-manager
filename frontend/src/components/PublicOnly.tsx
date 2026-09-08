// Wrapper de rutas publicas (change public-signup-landing, design D1):
// simetrico a RequireAuth pero invertido — un visitante sin sesion ve
// el contenido (landing, signup, login); un usuario autenticado va a
// /dashboard. `/`, `/signup` y `/login` viven fuera de RequireAuth
// envueltas en este componente (spec frontend-routing).
import { Navigate } from 'react-router-dom'
import { useAuth } from '../lib/auth-context'

export default function PublicOnly({ children }: { children: React.ReactNode }) {
  const { loading, user } = useAuth()

  if (loading) {
    return null // breve espera mientras /me resuelve; evita flash publico
  }

  if (user) {
    return <Navigate to="/dashboard" replace />
  }

  return <>{children}</>
}
