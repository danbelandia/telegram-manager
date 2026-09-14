// GroupDetailPage migrado a Mantine (frontend-refresh slice 1 — design D14,
// spec REQ-9). Header con Title + telegram_id + Badge de estado del bot.
// `<Tabs>` con 3 paneles: Detalle (info + permisos), Solicitudes (link a
// /requests), Logs (link a /logs). Las acciones destructivas (lock chat,
// delete/pin mensaje) se movieron a GroupUsersPage / vistas dedicadas para
// mantener este page como hub de navegacion del grupo.
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
  Title,
} from '@mantine/core'
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

      <Group>
        <Button
          component={Link}
          to={`/groups/${groupId}/users`}
          variant="light"
        >
          Membresía y moderación
        </Button>
        <Button
          component={Link}
          to={`/groups/${groupId}/automation`}
          variant="light"
          data-testid="automation-link"
        >
          Configurar reglas
        </Button>
        <Button
          component={Link}
          to={`/groups/${groupId}/moderation`}
          variant="light"
          data-testid="moderation-dashboard-link"
        >
          Ver dashboard
        </Button>
      </Group>

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