// GroupDetailPage migrado a Mantine (frontend-refresh slice 1 — design D14,
// spec REQ-9). Header con Title + telegram_id + Badge de estado del bot.
// `<Tabs>` con 4 paneles: Detalle (info + permisos), Membresia y moderacion
// (lock/unlock + delete/pin + link a /users), Solicitudes (link a /requests),
// Logs (link a /logs). Los hooks de moderacion y useGroup se mantienen sin
// cambios (NO se tocan features/* — el alcance del slice es solo view layer).
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Anchor,
  Badge,
  Box,
  Button,
  Group,
  Skeleton,
  Stack,
  Tabs,
  Text,
  TextInput,
  Title,
} from '@mantine/core'
import { formatModerationError } from '../features/moderation/error'
import { useDeleteMessage, useLockGroup, usePinMessage, useUnlockGroup } from '../features/moderation/hooks'
import { formatPermissions } from '../features/groups/permissions'
import { useGroup } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''
  const { data: group, isPending, isError, error, refetch } = useGroup(groupId)

  const lock = useLockGroup()
  const unlock = useUnlockGroup()
  const deleteMsg = useDeleteMessage()
  const pin = usePinMessage()

  const [messageId, setMessageId] = useState('')

  const chatError = lock.error ?? unlock.error
  const messageError = deleteMsg.error ?? pin.error
  const chatPending = lock.isPending || unlock.isPending
  const messagePending = deleteMsg.isPending || pin.isPending

  const confirmLock = () => {
    if (!window.confirm('¿Cerrar el envío de mensajes en este grupo?')) return
    lock.mutate(groupId)
  }

  const confirmDelete = () => {
    const parsed = Number(messageId)
    if (!Number.isInteger(parsed) || parsed <= 0) {
      window.alert('Ingresá un ID de mensaje válido (número positivo).')
      return
    }
    if (!window.confirm(`¿Eliminar el mensaje ${parsed}? Esta acción es irreversible.`)) return
    deleteMsg.mutate({ groupId, messageId: parsed })
  }

  const pinMessage = () => {
    const parsed = Number(messageId)
    if (!Number.isInteger(parsed) || parsed <= 0) {
      window.alert('Ingresá un ID de mensaje válido (número positivo).')
      return
    }
    pin.mutate({ groupId, messageId: parsed })
  }

  if (isPending) {
    return (
      <Stack gap="md">
        <Skeleton height={28} width="40%" />
        <Skeleton height={16} width="60%" />
        <Skeleton height={120} />
      </Stack>
    )
  }

  if (isError) {
    return (
      <Stack gap="md">
        <Title order={2}>Grupo</Title>
        <Text c="red">
          {error instanceof Error ? error.message : 'No se pudo cargar el grupo.'}
        </Text>
        <Button variant="default" w={140} onClick={() => refetch()}>
          Reintentar
        </Button>
      </Stack>
    )
  }

  if (!group) {
    return (
      <Stack gap="md">
        <Title order={2}>Grupo</Title>
        <Text c="dimmed">Grupo no encontrado.</Text>
        <Button component={Link} to="/groups" variant="default" w={140}>
          Volver a grupos
        </Button>
      </Stack>
    )
  }

  const permissions = formatPermissions(group.bot_permissions)

  return (
    <Stack gap="md">
      <Anchor component={Link} to="/groups" size="sm">
        ← Grupos
      </Anchor>

      <Box>
        <Title order={2}>{group.title}</Title>
        {group.username ? (
          <Text size="sm" c="dimmed">
            @{group.username}
          </Text>
        ) : null}
        <Group gap="sm" mt="xs" align="center">
          <Text size="sm" c="dimmed">
            ID:
          </Text>
          <Text size="sm">{group.telegram_id}</Text>
          <Badge variant="light" color="blue">
            Bot: {group.bot_status}
          </Badge>
        </Group>
      </Box>

      <Stack gap="xs">
        <Title order={4}>Acciones de moderación</Title>
        {chatError ? (
          <Text c="red" size="sm">
            {formatModerationError(chatError)}
          </Text>
        ) : null}
        <Group>
          <Button variant="default" disabled={chatPending} onClick={() => unlock.mutate(groupId)}>
            🔓 Abrir chat
          </Button>
          <Button color="red" disabled={chatPending} onClick={confirmLock}>
            🔒 Cerrar chat
          </Button>
        </Group>
        {messageError ? (
          <Text c="red" size="sm">
            {formatModerationError(messageError)}
          </Text>
        ) : null}
        <Group align="flex-end">
          <TextInput
            label="ID del mensaje en Telegram"
            placeholder="ID del mensaje"
            inputMode="numeric"
            value={messageId}
            onChange={(e) => setMessageId(e.currentTarget.value)}
            style={{ flex: 1 }}
          />
          <Button color="red" disabled={messagePending} onClick={confirmDelete}>
            Eliminar
          </Button>
          <Button variant="default" disabled={messagePending} onClick={pinMessage}>
            Fijar
          </Button>
        </Group>
      </Stack>

      <Tabs defaultValue="detalle">
        <Tabs.List>
          <Tabs.Tab value="detalle">Detalle</Tabs.Tab>
          <Tabs.Tab value="solicitudes">Solicitudes</Tabs.Tab>
          <Tabs.Tab value="logs">Logs</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="detalle" pt="md">
          <Stack gap="md">
            <Stack gap={4}>
              <Text size="sm" c="dimmed">
                Tipo
              </Text>
              <Text>{group.type}</Text>
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="dimmed">
                Miembros
              </Text>
              <Text>{formatMembers(group.member_count)}</Text>
            </Stack>
            <Stack gap={4}>
              <Text size="sm" c="dimmed">
                Permisos del bot
              </Text>
              <Text>
                {permissions.length > 0 ? permissions.join(', ') : 'Sin permisos'}
              </Text>
            </Stack>
            <Button
              component={Link}
              to={`/groups/${groupId}/users`}
              variant="light"
              w={260}
            >
              Membresía y moderación
            </Button>
            <Button
              component={Link}
              to={`/groups/${groupId}/automation`}
              variant="light"
              w={260}
              data-testid="automation-link"
            >
              Configurar reglas de moderación
            </Button>
            <Button
              component={Link}
              to={`/groups/${groupId}/moderation`}
              variant="light"
              w={260}
              data-testid="moderation-dashboard-link"
            >
              Ver dashboard de moderación
            </Button>
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="solicitudes" pt="md">
          <Stack gap="md">
            <Text c="dimmed">
              Las solicitudes de ingreso se gestionan en una vista dedicada.
            </Text>
            <Button
              component={Link}
              to={`/groups/${groupId}/requests`}
              variant="default"
              w={260}
            >
              Ver solicitudes
            </Button>
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="logs" pt="md">
          <Stack gap="md">
            <Text c="dimmed">
              El historial de acciones administrativas vive en la vista de logs.
            </Text>
            <Button
              component={Link}
              to={`/groups/${groupId}/logs`}
              variant="default"
              w={260}
            >
              Ver logs
            </Button>
          </Stack>
        </Tabs.Panel>
      </Tabs>
    </Stack>
  )
}