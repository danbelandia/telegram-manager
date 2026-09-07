// Pagina de login (AGENTS 17/17.1). Formulario con React Hook Form +
// Zod (guia frontend seccion 5): validacion cliente antes de llamar a
// la API. Tras login exitoso redirige a la ruta intentada
// (location.state.from, spec frontend-routing) o al dashboard.
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth-context'

// Schema del formulario: los dos campos son obligatorios (el backend
// valida de nuevo; esta validacion es solo UX, guia seccion 5).
const loginSchema = z.object({
  username: z.string().min(1, 'El usuario es obligatorio'),
  password: z.string().min(1, 'La contraseña es obligatoria'),
})

type LoginForm = z.infer<typeof loginSchema>

export default function LoginPage() {
  const { user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [formError, setFormError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>({ resolver: zodResolver(loginSchema) })

  // Ya hay sesion: ahi afuera.
  if (user) {
    return <Navigate to="/dashboard" replace />
  }

  const from = (location.state as { from?: string } | null)?.from ?? '/dashboard'

  const onSubmit = async (values: LoginForm) => {
    setFormError(null)
    try {
      await login(values.username, values.password)
      navigate(from, { replace: true })
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'No se pudo iniciar sesión')
    }
  }

  return (
    <main className="login-page">
      <h1>Iniciar sesión</h1>
      <form className="login-form" onSubmit={handleSubmit(onSubmit)} noValidate>
        <label className="form-field">
          <span>Usuario</span>
          <input type="text" autoComplete="username" {...register('username')} />
          {errors.username ? <span className="form-error">{errors.username.message}</span> : null}
        </label>

        <label className="form-field">
          <span>Contraseña</span>
          <input type="password" autoComplete="current-password" {...register('password')} />
          {errors.password ? <span className="form-error">{errors.password.message}</span> : null}
        </label>

        {formError ? <p className="form-error form-error-global">{formError}</p> : null}

        <button type="submit" className="btn btn-primary" disabled={isSubmitting}>
          {isSubmitting ? 'Ingresando…' : 'Ingresar'}
        </button>
      </form>
    </main>
  )
}