// Solicitudes de ingreso del grupo (spec frontend-pages-moderation
// REQ-4..5): lista de join_requests en <Table> Mantine con Badge por
// estado, acciones Aprobar/Rechazar clic directo + notifySuccess (sin
// Modal — la accion es reversible: el admin puede deshacerla desde
// Telegram). Reemplaza los banners inline de la version CSS plana y
// delega feedback a lib/notifications (slice 1 helper).
//
// Batch approve/reject: permite seleccionar hasta 50 solicitudes
// pendientes via checkboxes y ejecutar approve/reject en lote.
// Patron similar a publications-batch (BatchWizard) pero inline en la
// tabla (sin modal): checkboxes + botones de accion por debajo del
// titulo.
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Group,
  NativeSelect,
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
  useBatchJoinRequest,
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

/** Maximo de solicitudes seleccionables por batch. */
const MAX_BATCH_SIZE = 50
const PAGE_SIZE = 50

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

function RequestsSkeleton() {
  return (
    <Table striped highlightOnHover>
      <Table.Thead>
        <Table.Tr>
          <Table.Th w={40} />
          <Table.Th>Usuario</Table.Th>
          <Table.Th>Fecha</Table.Th>
          <Table.Th>Estado</Table.Th>
          <Table.Th>Acciones</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {[0, 1, 2].map((i) => (
          <Table.Tr key={i}>
            <Table.Td><Skeleton height={16} width={16} /></Table.Td>
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

  // ── Filter / pagination state ──────────────────────────────────
  const [statusFilter, setStatusFilter] = useState('')
  const [offset, setOffset] = useState(0)

  const requests = useJoinRequests(groupId, {
    status: statusFilter || undefined,
    limit: PAGE_SIZE,
    offset,
  })
  const approve = useApproveJoinRequest()
  const reject = useRejectJoinRequest()
  const batchMutate = useBatchJoinRequest()

  const pending = approve.isPending || reject.isPending || batchMutate.isPending

  // ── Batch selection state ──────────────────────────────────────
  const [selected, setSelected] = useState<Set<number>>(new Set())

  // Current page items (new response format: { data, total })
  const pageItems = requests.data?.data ?? []

  const pendingIds = pageItems
    .filter((r) => r.status === 'pending')
    .map((r) => r.id)

  const allPendingSelected =
    pendingIds.length > 0 && pendingIds.every((id) => selected.has(id))

  const toggleAll = () => {
    if (allPendingSelected) {
      setSelected(new Set())
    } else {
      setSelected(new Set(pendingIds.slice(0, MAX_BATCH_SIZE)))
    }
  }

  const toggleOne = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        if (next.size >= MAX_BATCH_SIZE) {
          notifyError(`Maximo ${MAX_BATCH_SIZE} solicitudes seleccionables`)
          return prev
        }
        next.add(id)
      }
      return next
    })
  }

  const clearSelection = () => setSelected(new Set())

  // Reset selection when filter or page changes
  useEffect(() => {
    setSelected(new Set())
  }, [statusFilter, offset])

  // ── Batch actions ──────────────────────────────────────────────
  const runBatch = (action: 'approve' | 'reject') => {
    const ids = Array.from(selected)
    if (ids.length === 0) return

    const label = action === 'approve' ? 'Aprobar' : 'Rechazar'
    batchMutate.mutate(
      { groupId, action, requestIds: ids },
      {
        onSuccess: (data) => {
          const ok = data.results.filter((r) => r.status !== 'error').length
          const fail = data.results.filter((r) => r.status === 'error').length
          const parts: string[] = []
          if (ok > 0) parts.push(`${ok} ${label.toLowerCase()}${ok > 1 ? 's' : ''}`)
          if (fail > 0) parts.push(`${fail} error${fail > 1 ? 'es' : ''}`)
          notifySuccess(parts.join(', '))
          clearSelection()
        },
        onError: (e) => notifyError(formatModerationError(e)),
      },
    )
  }

  // ── Single actions ─────────────────────────────────────────────
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

  const batchMode = selected.size > 0

  // ── Pagination ─────────────────────────────────────────────────
  const canPrev = offset > 0
  const canNext = pageItems.length >= PAGE_SIZE
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1

  // Empty state message depends on active filter
  const emptyMessage = statusFilter
    ? `Sin solicitudes con estado "${STATUS_LABEL[statusFilter as JoinRequest['status']] ?? statusFilter}".`
    : 'Sin solicitudes de ingreso pendientes ni resueltas.'

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

        {/* ── Filter ─────────────────────────────────────────── */}
        <NativeSelect
          label="Filtrar por estado"
          name="filter-status"
          value={statusFilter}
          onChange={(e) => {
            setStatusFilter(e.currentTarget.value)
            setOffset(0)
          }}
          data={[
            { value: '', label: 'Todos' },
            { value: 'pending', label: 'Pendiente' },
            { value: 'approved', label: 'Aprobada' },
          ]}
          radius="md"
        />

        {/* ── Batch action bar ──────────────────────────────── */}
        {batchMode ? (
          <Group gap="xs">
            <Button
              size="sm"
              variant="filled"
              color="green"
              leftSection={<IconCheck size={16} />}
              disabled={pending}
              onClick={() => runBatch('approve')}
              loading={batchMutate.isPending}
            >
              Aprobar ({selected.size})
            </Button>
            <Button
              size="sm"
              variant="filled"
              color="red"
              leftSection={<IconX size={16} />}
              disabled={pending}
              onClick={() => runBatch('reject')}
              loading={batchMutate.isPending}
            >
              Rechazar ({selected.size})
            </Button>
            <Button
              size="sm"
              variant="subtle"
              color="gray"
              disabled={pending}
              onClick={clearSelection}
            >
              Limpiar seleccion
            </Button>
          </Group>
        ) : null}

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

        {!requests.isPending && !requests.isError && pageItems.length === 0 ? (
          <Text c="dimmed">
            {emptyMessage}
          </Text>
        ) : null}

        {!requests.isPending && !requests.isError && pageItems.length > 0 ? (
          <Table.ScrollContainer minWidth={500}>
            <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={40}>
                    <Checkbox
                      checked={allPendingSelected}
                      indeterminate={selected.size > 0 && !allPendingSelected}
                      onChange={toggleAll}
                      disabled={pendingIds.length === 0}
                      aria-label="Seleccionar todas las pendientes"
                    />
                  </Table.Th>
                  <Table.Th>Usuario</Table.Th>
                  <Table.Th>Fecha</Table.Th>
                  <Table.Th>Estado</Table.Th>
                  <Table.Th>Acciones</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {pageItems.map((req) => (
                  <Table.Tr key={req.id}>
                    <Table.Td>
                      {req.status === 'pending' ? (
                        <Checkbox
                          checked={selected.has(req.id)}
                          onChange={() => toggleOne(req.id)}
                          disabled={pending}
                          aria-label={`Seleccionar solicitud de ${req.first_name}`}
                        />
                      ) : null}
                    </Table.Td>
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
                            disabled={pending || batchMode}
                            onClick={() => runApprove(req.id)}
                          >
                            Aprobar
                          </Button>
                          <Button
                            size="xs"
                            variant="light"
                            color="red"
                            leftSection={<IconX size={14} />}
                            disabled={pending || batchMode}
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

        {/* ── Pagination ─────────────────────────────────────── */}
        {!requests.isPending && !requests.isError && (pageItems.length > 0 || offset > 0) ? (
          <Group justify="space-between">
            <Button
              variant="default"
              disabled={!canPrev}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              ← Anterior
            </Button>
            <Text size="sm" c="dimmed">Página {currentPage}</Text>
            <Button
              variant="default"
              disabled={!canNext}
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              Siguiente →
            </Button>
          </Group>
        ) : null}
      </Stack>
    </Paper>
  )
}
