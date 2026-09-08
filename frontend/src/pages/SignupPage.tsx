// SignupPage (change public-signup-landing, design D2–D4): alta publica
// de tenant con React Hook Form + Zod espejando la validacion del
// backend (backend/internal/auth/service.go: slug trim 1–63, username
// no vacio, password >= 8, token no vacio; sin trim en password como
// el backend). Flujo: signup() → login(u,p) → /dashboard; si el
// auto-login falla, degradacion a /login?username=&created=1 (Q2).
// Errores: 409 por substring (slug/username, fallback generico),
// 400 al campo identificado, 502 con guia @BotFather (D4).
// Higiene (REQ hygiene): el bot_token vive solo en el campo
// controlado + variable local; se limpia con reset en `finally`;
// jamas va a contexto, QueryClient, logs ni storage.
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import {
  Anchor,
  Button,
  Center,
  Paper,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core'
import { useAuth } from '../lib/auth-context'
import { ApiError } from '../lib/api-client'
import { signup } from '../features/auth/api'
import { notifyError, notifySuccess } from '../lib/notifications'

const signupSchema = z.object({
  slug: z
    .string()
    .trim()
    .min(1, 'El identificador es obligatorio')
    .max(63, 'Máximo 63 caracteres'),
  username: z.string().trim().min(1, 'El usuario es obligatorio'),
  password: z.string().min(8, 'Mínimo 8 caracteres'),
  bot_token: z.string().trim().min(1, 'El token del bot es obligatorio'),
})

type SignupForm = z.infer<typeof signupSchema>

export default function SignupPage() {
  const { user, login } = useAuth()
  const navigate = useNavigate()

  const {
    control,
    handleSubmit,
    getValues,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<SignupForm>({
    resolver: zodResolver(signupSchema),
    defaultValues: { slug: '', username: '', password: '', bot_token: '' },
  })

  if (user) {
    return <Navigate to="/dashboard" replace />
  }

  /** 409/400/502 → setError en el campo (o raiz); resto → raiz. */
  const mapSignupError = (err: unknown) => {
    if (err instanceof ApiError) {
      const msg = err.message.toLowerCase()
      if (err.status === 409) {
        if (msg.includes('slug')) {
          setError('slug', { message: err.message })
          return
        }
        if (msg.includes('username')) {
          setError('username', { message: err.message })
          return
        }
        setError('root', { message: err.message })
        return
      }
      if (err.status === 400) {
        if (msg.includes('bot_token')) {
          setError('bot_token', { message: err.message })
          return
        }
        if (msg.includes('password')) {
          setError('password', { message: err.message })
          return
        }
        if (msg.includes('slug')) {
          setError('slug', { message: err.message })
          return
        }
        if (msg.includes('username')) {
          setError('username', { message: err.message })
          return
        }
        setError('root', { message: err.message })
        return
      }
      if (err.status === 502) {
        const message = `${err.message}. Verificá el token con @BotFather y reintentá.`
        setError('root', { message })
        notifyError(message)
        return
      }
    }
    const message = err instanceof Error ? err.message : 'No se pudo crear la cuenta'
    setError('root', { message })
    notifyError(message)
  }

  const onSubmit = async (values: SignupForm) => {
    // El token vive solo en esta variable local durante el submit.
    const botToken = values.bot_token
    try {
      await signup({
        slug: values.slug,
        username: values.username,
        password: values.password,
        bot_token: botToken,
      })
      try {
        await login(values.username, values.password)
        notifySuccess('Cuenta creada')
        navigate('/dashboard', { replace: true })
      } catch {
        // Degradacion (Q2): el tenant existe pero el auto-login fallo.
        navigate(`/login?username=${encodeURIComponent(values.username)}&created=1`, {
          replace: true,
        })
      }
    } catch (err) {
      mapSignupError(err)
    } finally {
      // Higiene: el campo del token queda vacio; el resto se conserva
      // para corregir sin reescribir todo. `keepErrors` porque reset
      // limpia los errores que setError marco en el catch.
      reset({ ...getValues(), bot_token: '' }, { keepErrors: true })
    }
  }

  return (
    <Center mih="100vh" px="md">
      <Paper withBorder shadow="md" p="xl" radius="md" w={400}>
        <Title order={2} ta="center" mb="lg">
          Crear cuenta
        </Title>
        <form onSubmit={handleSubmit(onSubmit)} noValidate>
          <Stack>
            <Controller
              name="slug"
              control={control}
              render={({ field }) => (
                <TextInput
                  {...field}
                  label="Identificador del espacio"
                  description="Se usa en la URL de tu panel"
                  autoComplete="off"
                  error={errors.slug?.message}
                  withAsterisk={false}
                />
              )}
            />
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
                  description="Mínimo 8 caracteres"
                  autoComplete="new-password"
                  error={errors.password?.message}
                  withAsterisk={false}
                />
              )}
            />
            <Controller
              name="bot_token"
              control={control}
              render={({ field }) => (
                <PasswordInput
                  {...field}
                  label="Token del bot"
                  description="Hablá con @BotFather en Telegram para crear un bot y pegar su token"
                  autoComplete="off"
                  error={errors.bot_token?.message}
                  withAsterisk={false}
                />
              )}
            />
            {errors.root?.message ? (
              <Text size="sm" c="red" role="alert">
                {errors.root.message}
              </Text>
            ) : null}
            <Button type="submit" loading={isSubmitting} fullWidth>
              {isSubmitting ? 'Creando cuenta…' : 'Crear cuenta'}
            </Button>
            <Text size="sm" ta="center">
              ¿Ya tenés cuenta?{' '}
              <Anchor component={Link} to="/login">
                Iniciá sesión
              </Anchor>
            </Text>
          </Stack>
        </form>
      </Paper>
    </Center>
  )
}
