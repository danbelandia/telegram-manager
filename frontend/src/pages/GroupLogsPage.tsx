// Logs de auditoria del grupo (spec frontend-pages-moderation REQ-6):
// entradas de la API ordenadas desc por created_at, renderizadas en
// <Table> Mantine con <Table.Thead sticky> y <Badge> por status segun
// el mapa STATUS_BADGE_COLOR. La columna Penta muestra error_message
// cuando existe (envuelta en <Tooltip>), si no "por admin {actor_id}",
// si no un guion. Paginacion diferida a slice 3 (REQ-7 non-goal).
import { Link, useParams } from 'react-router-dom'
import {
  Alert,
  Badge,
  Button,
  Group,
  Paper,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core'
import { IconArrowLeft } from '@tabler/icons-react'
import { useGroupLogs } from '../features/moderation/hooks'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

// Mapa de color del Badge segun AGENTS 11/18. Backend devuelve el
// status en MAYUSCULAS; el mapa usa minusculas como key porque el
// backend puede devolver variantes (TELEGRAM_ERROR vs ERROR).
const STATUS_BADGE_COLOR: Record<string, string> = {
  success: 'green',
  permission_denied: 'red',
  internal_error: 'red',
  error: 'red',
  validation_error: 'yellow',
  telegram_error: 'orange',
  not_found: 'gray',
}

function statusColor(status: string): string {
  return STATUS_BADGE_COLOR[status.toLowerCase()] ?? 'gray'
}

function LogsSkeleton() {
  return (
    <Table striped highlightOnHover withTableBorder>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Fecha</Table.Th>
          <Table.Th>Actor</Table.Th>
          <Table.Th>Acción</Table.Th>
          <Table.Th>Estado</Table.Th>
          <Table.Th>Mensaje</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {[0, 1, 2, 3, 4].map((i) => (
          <Table.Tr key={i}>
            <Table.Td><Skeleton height={16} width="80%" /></Table.Td>
            <Table.Td><Skeleton height={16} width="60%" /></Table.Td>
            <Table.Td><Skeleton height={16} width="70%" /></Table.Td>
            <Table.Td><Skeleton height={16} width={80} /></Table.Td>
            <Table.Td><Skeleton height={16} width="90%" /></Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}

export default function GroupLogsPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const logs = useGroupLogs(groupId)

  return (
    <Paper p="md" withBorder radius="md">
      <Stack gap="md">
        <Group gap="xs">
          <Button
            component={Link}
            to={`/groups/${groupId}`}
            variant="subtle"
            size="xs"
            leftSection={<IconArrowLeft size={14} />}
          >
            Volver al grupo
          </Button>
        </Group>

        <Title order={2}>Logs del grupo</Title>

        {logs.isPending ? <LogsSkeleton /> : null}

        {logs.isError ? (
          <Alert color="red" variant="light" title="Error">
            <Stack gap="xs">
              <Text size="sm">
                {logs.error instanceof Error
                  ? logs.error.message
                  : 'No se pudieron cargar los logs.'}
              </Text>
              <Group>
                <Button variant="default" size="xs" onClick={() => logs.refetch()}>
                  Reintentar
                </Button>
              </Group>
            </Stack>
          </Alert>
        ) : null}

        {!logs.isPending && !logs.isError && logs.data && logs.data.length === 0 ? (
          <Text c="dimmed">Sin acciones registradas en este grupo.</Text>
        ) : null}

        {!logs.isPending && !logs.isError && logs.data && logs.data.length > 0 ? (
          <Table.ScrollContainer minWidth={500}>
            <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
<Table.Thead
                style={{
                  position: 'sticky',
                  top: 0,
                  zIndex: 1,
                  background: 'var(--mantine-color-body)',
                }}
              >
                <Table.Tr>
                  <Table.Th>Fecha</Table.Th>
                  <Table.Th>Actor</Table.Th>
                  <Table.Th>Acción</Table.Th>
                  <Table.Th>Estado</Table.Th>
                  <Table.Th>Mensaje</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {logs.data.map((entry) => {
                  const mensaje = entry.error_message
                    ?? (entry.actor_id ? `por admin ${entry.actor_id}` : '—')
                  return (
                    <Table.Tr key={entry.id}>
                      <Table.Td>{formatDate(entry.created_at)}</Table.Td>
                      <Table.Td>
                        <Text size="sm">
                          {entry.actor_id ? `admin ${entry.actor_id}` : '—'}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text fw={500}>{entry.action}</Text>
                        {entry.target_user_id ? (
                          <Text size="xs" c="dimmed">→ usuario {entry.target_user_id}</Text>
                        ) : null}
                      </Table.Td>
                      <Table.Td>
                        <Badge color={statusColor(entry.status)} variant="light">
                          {entry.status}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        {entry.error_message ? (
                          <Tooltip label={entry.error_message} withArrow>
                            <Text size="sm" lineClamp={1}>
                              {entry.error_message}
                            </Text>
                          </Tooltip>
                        ) : (
                          <Text size="sm" c="dimmed">{mensaje}</Text>
                        )}
                      </Table.Td>
                    </Table.Tr>
                  )
                })}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        ) : null}
      </Stack>
    </Paper>
  )
}