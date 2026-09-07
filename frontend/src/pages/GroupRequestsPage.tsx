// Solicitudes de ingreso del grupo (spec frontend-pages-moderation
// REQ-4..5): lista de join_requests en <Table> Mantine con Badge por
// estado, acciones Aprobar/Rechazar clic directo + notifySuccess (sin
// Modal — la acción es reversible: el admin puede deshacerla desde
// Telegram). Reemplaza los banners inline de la versión CSS plana y
// delega feedback a lib/notifications (slice 1 helper).
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
} from '@mantine/core'
import { IconArrowLeft, IconCheck, IconX } from '@tabler/icons-react'
import { formatModerationError } from '../features/moderation/error'
import {
  useApproveJoinRequest,
  useJoinRequests,
  useRejectJoinRequest,
} from '../features/moderation/hooks'
import type { JoinRequest } from '../features/moderation/types'
import { notifyError, notifySuccess } from '../lib/notifications'

const STATUS_LABEL: Record<JoinRequest['status'], string> = {
  pending: 'Pendiente',
  approved: 'Aprobada',
  rejected: 'Rechazada',
}

const STATUS_COLOR: Record<JoinRequest['status'], string> = {
  pending: 'yellow',
  approved: 'green',
  rejected: 'red',
}

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

function RequestsSkeleton() {
  return (
    <Table striped highlightOnHover>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Usuario</Table.Th>
          <Table.Th>Fecha</Table.Th>
          <Table.Th>Estado</Table.Th>
          <Table.Th>Acciones</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {[0, 1, 2].map((i) => (
          <Table.Tr key={i}>
            <Table.Td><Skeleton height={16} width="60%" /></Table.Td>
            <Table.Td><Skeleton height={16} width="80%" /></Table.Td>
            <Table.Td><Skeleton height={16} width={80} /></Table.Td>
            <Table.Td><Skeleton height={24} width={120} /></Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}

export default function GroupRequestsPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const requests = useJoinRequests(groupId)
  const approve = useApproveJoinRequest()
  const reject = useRejectJoinRequest()

  const pending = approve.isPending || reject.isPending

  const runApprove = (requestId: number) => {
    approve.mutate(
      { groupId, requestId },
      {
        onSuccess: () => notifySuccess('Solicitud aprobada'),
        onError: (e) => notifyError(formatModerationError(e)),
      },
    )
  }

  const runReject = (requestId: number) => {
    reject.mutate(
      { groupId, requestId },
      {
        onSuccess: () => notifySuccess('Solicitud rechazada'),
        onError: (e) => notifyError(formatModerationError(e)),
      },
    )
  }

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

        <Title order={2}>Solicitudes de ingreso</Title>

        {requests.isPending ? <RequestsSkeleton /> : null}

        {requests.isError ? (
          <Alert color="red" variant="light" title="Error">
            <Stack gap="xs">
              <Text size="sm">
                {requests.error instanceof Error
                  ? requests.error.message
                  : 'No se pudieron cargar las solicitudes.'}
              </Text>
              <Group>
                <Button variant="default" size="xs" onClick={() => requests.refetch()}>
                  Reintentar
                </Button>
              </Group>
            </Stack>
          </Alert>
        ) : null}

        {!requests.isPending && !requests.isError && requests.data && requests.data.length === 0 ? (
          <Text c="dimmed">
            Sin solicitudes de ingreso pendientes ni resueltas.
          </Text>
        ) : null}

        {!requests.isPending && !requests.isError && requests.data && requests.data.length > 0 ? (
          <Table.ScrollContainer minWidth={500}>
            <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Usuario</Table.Th>
                  <Table.Th>Fecha</Table.Th>
                  <Table.Th>Estado</Table.Th>
                  <Table.Th>Acciones</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {requests.data.map((req) => (
                  <Table.Tr key={req.id}>
                    <Table.Td>
                      <Text fw={500}>{req.first_name}</Text>
                      <Text size="xs" c="dimmed">
                        ID: {req.user_id}
                        {req.username ? ` · @${req.username}` : ''}
                      </Text>
                    </Table.Td>
                    <Table.Td>{formatDate(req.requested_at)}</Table.Td>
                    <Table.Td>
                      <Badge color={STATUS_COLOR[req.status]} variant="light">
                        {STATUS_LABEL[req.status]}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      {req.status === 'pending' ? (
                        <Group gap="xs">
                          <Button
                            size="xs"
                            variant="light"
                            color="green"
                            leftSection={<IconCheck size={14} />}
                            disabled={pending}
                            onClick={() => runApprove(req.id)}
                          >
                            Aprobar
                          </Button>
                          <Button
                            size="xs"
                            variant="light"
                            color="red"
                            leftSection={<IconX size={14} />}
                            disabled={pending}
                            onClick={() => runReject(req.id)}
                          >
                            Rechazar
                          </Button>
                        </Group>
                      ) : (
                        <Text size="sm" c="dimmed">
                          {req.decided_by ? `Decidida por admin ${req.decided_by}` : 'Decidida'}
                        </Text>
                      )}
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        ) : null}
      </Stack>
    </Paper>
  )
}