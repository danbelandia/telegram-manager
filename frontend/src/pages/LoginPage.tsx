// LoginPage migrado a Mantine (frontend-refresh slice 1 — design D8/D9).
// Form con React Hook Form + Zod (sin cambios en schema); inputs envueltos
// en `Controller` para conectar Mantine inputs con RHF (D8). Notificaciones
// globales: exito → `notifySuccess('Bienvenido')`, error → `notifyError(msg)`
// (slice 2 conecta el resto de los hooks siguiendo este patron).
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import {
  Alert,
  Button,
  Center,
  Paper,
  PasswordInput,
  Stack,
  TextInput,
  Title,
} from '@mantine/core'
import { useAuth } from '../lib/auth-context'
import { notifyError, notifySuccess } from '../lib/notifications'

const loginSchema = z.object({
  username: z.string().min(1, 'El usuario es obligatorio'),
  password: z.string().min(1, 'La contraseña es obligatoria'),
})

type LoginForm = z.infer<typeof loginSchema>

export default function LoginPage() {
  const { user, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  // Degradacion del signup (Q2/D6): ?username= pre-rellena y
  // ?created=1 muestra el aviso. Query params (no location.state)
  // para sobrevivir al redirect tras el auto-login fallido.
  const [searchParams] = useSearchParams()
  const prefilledUsername = searchParams.get('username') ?? ''
  const justCreated = searchParams.get('created') === '1'

  const {
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginForm>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: prefilledUsername, password: '' },
  })

  if (user) {
    return <Navigate to="/dashboard" replace />
  }

  const from = (location.state as { from?: string } | null)?.from ?? '/dashboard'

  const onSubmit = async (values: LoginForm) => {
    try {
      await login(values.username, values.password)
      notifySuccess('Bienvenido')
      navigate(from, { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'No se pudo iniciar sesión'
      notifyError(message)
    }
  }

  return (
    <Center mih="100vh" px="md">
      <Paper withBorder shadow="md" p="xl" radius="md" w={360}>
        <Title order={2} ta="center" mb="lg">
          Iniciar sesión
        </Title>
        <form onSubmit={handleSubmit(onSubmit)} noValidate>
          <Stack>
            {justCreated ? (
              <Alert color="green" title="Cuenta creada, iniciá sesión" />
            ) : null}
            <Controller
              name="username"
              control={control}
              render={({ field }) => (
                <TextInput
                  {...field}
                  label="Usuario"
                  autoComplete="username"
                  error={errors.username?.message}
                  withAsterisk={false}
                />
              )}
            />
            <Controller
              name="password"
              control={control}
              render={({ field }) => (
                <PasswordInput
                  {...field}
                  label="Contraseña"
                  autoComplete="current-password"
                  error={errors.password?.message}
                  withAsterisk={false}
                />
              )}
            />
            <Button type="submit" loading={isSubmitting} fullWidth>
              {isSubmitting ? 'Ingresando…' : 'Ingresar'}
            </Button>
          </Stack>
        </form>
      </Paper>
    </Center>
  )
}