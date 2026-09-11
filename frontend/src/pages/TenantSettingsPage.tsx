// Pagina de configuracion del tenant (spec tenant-settings REQ 7):
// datos readonly + formulario de rotacion de token con Zod + feedback.
import { useEffect, useState } from 'react'
import { Paper, Title, Text, Stack, TextInput, Button, Group, Divider, Box } from '@mantine/core'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod/v4'
import { getTenantMe, rotateBotToken } from '../features/tenant/api'
import type { TenantMe } from '../features/tenant/types'
import { notifySuccess, notifyError } from '../lib/notifications'
import { ApiError } from '../lib/api-client'
import LicenseCard from '../components/LicenseCard'

const rotateSchema = z.object({
  password: z.string().min(1, 'La password es requerida'),
  bot_token: z.string().min(1, 'El bot token es requerido'),
})

type RotateForm = z.infer<typeof rotateSchema>

export default function TenantSettingsPage() {
  const [tenant, setTenant] = useState<TenantMe | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<RotateForm>({
    resolver: zodResolver(rotateSchema),
  })

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const data = await getTenantMe()
        if (!cancelled) setTenant(data)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Error al cargar datos del tenant')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [])

  const onSubmit = async (data: RotateForm) => {
    try {
      await rotateBotToken(data)
      notifySuccess('Token rotado correctamente')
      reset()
      // Recargar datos para mostrar el nuevo bot_status
      const refreshed = await getTenantMe()
      setTenant(refreshed)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'UNAUTHORIZED') {
          notifyError('Password incorrecta', 'Error de autenticacion')
        } else if (err.code === 'TELEGRAM_ERROR') {
          notifyError('Telegram rechazo el bot token. Verifica que sea correcto.', 'Token invalido')
        } else {
          notifyError(err.message, 'Error')
        }
      } else {
        notifyError('Error inesperado', 'Error')
      }
    }
  }

  if (loading) {
    return (
      <Paper p="md" withBorder>
        <Text c="dimmed">Cargando configuracion...</Text>
      </Paper>
    )
  }

  if (error) {
    return (
      <Paper p="md" withBorder>
        <Text c="red">{error}</Text>
      </Paper>
    )
  }

  return (
    <Stack gap="lg">
      <Title order={3}>Configuracion del Tenant</Title>

      {/* Datos readonly del tenant */}
      <Paper p="md" withBorder>
        <Stack gap="xs">
          <Group>
            <Text fw={600}>Slug:</Text>
            <Text>{tenant?.slug}</Text>
          </Group>
          <Group>
            <Text fw={600}>Bot Username:</Text>
            <Text>{tenant?.bot_username ?? 'No conectado'}</Text>
          </Group>
          <Group>
            <Text fw={600}>Estado:</Text>
            <Text
              c={tenant?.bot_status === 'connected' ? 'green' : tenant?.bot_status === 'disconnected' ? 'red' : 'dimmed'}
            >
              {tenant?.bot_status === 'connected' ? 'Conectado' :
               tenant?.bot_status === 'disconnected' ? 'Desconectado' : 'Desconocido'}
            </Text>
          </Group>
          <Group>
            <Text fw={600}>Creado:</Text>
            <Text>{tenant?.created_at ? new Date(tenant.created_at).toLocaleDateString('es-AR') : '-'}</Text>
          </Group>
        </Stack>
      </Paper>

      {/* License card */}
      {tenant && <LicenseCard tenant={tenant} />}

      <Divider />

      {/* Formulario de rotacion de token */}
      <Paper p="md" withBorder>
        <Title order={4} mb="md">Rotar Bot Token</Title>
        <Box component="form" onSubmit={handleSubmit(onSubmit)}>
          <Stack gap="md">
            <TextInput
              label="Password actual"
              type="password"
              placeholder="Tu password"
              error={errors.password?.message}
              {...register('password')}
            />
            <TextInput
              label="Nuevo Bot Token"
              placeholder="123456:ABC-DEF..."
              error={errors.bot_token?.message}
              {...register('bot_token')}
            />
            <Group justify="flex-end">
              <Button type="submit" loading={isSubmitting}>
                Rotar Token
              </Button>
            </Group>
          </Stack>
        </Box>
      </Paper>
    </Stack>
  )
}
