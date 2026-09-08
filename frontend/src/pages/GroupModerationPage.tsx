// Pagina /groups/:id/moderation: dashboard de moderacion automatica
// (Fase 3 — slice 3). Read-only + reset manual. Es la contraparte de
// observacion de GroupAutomationPage (slice 2, settings + listas):
// ahi el admin edita, aca observa lo que la moderacion automatica
// esta haciendo.
//
// Layout en 2 secciones sin Save button:
//   1. Estadisticas: 3 Cards con counts (rule_triggered, automute,
//      autoban) + Select de periodo (24h/7d, default 24h) + boton
//      Refrescar que invalida las queries (sin auto-poll, YAGNI).
//   2. Advertencias activas: Tabla con display_name + count + ultima
//      advertencia (Intl.RelativeTimeFormat es-AR) + accion Reset (Modal
//      de confirmacion). Cap top 100 con Alert amarillo si truncated.
//      Empty state cuando no hay activas. Reset NO desmutea al user
//      (intencional; el admin usa POST /unmute por separado).
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Alert,
  Anchor,
  Badge,
  Box,
  Button,
  Card,
  Container,
  Group,
  Modal,
  Select,
  SimpleGrid,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
} from '@mantine/core'
import { IconRefresh } from '@tabler/icons-react'
import { notifications } from '@mantine/notifications'
import { useQueryClient } from '@tanstack/react-query'
import {
  formatDashboardError,
} from '../features/automation/error'
import {
  useResetWarning,
  useStats,
  useWarnings,
} from '../features/automation/hooks'
import type { StatsPeriod, WarningStateRow } from '../features/automation/types'

const PERIOD_OPTIONS: Array<{ value: StatsPeriod; label: string }> = [
  { value: '24h', label: 'Últimas 24 horas' },
  { value: '7d', label: 'Últimos 7 días' },
]

// FormatRelative devuelve "hace 2h" / "hace 3d" / "—" via
// Intl.RelativeTimeFormat (es-AR). YAGNI para una lib de fechas
// completa: el dashboard solo necesita "hace X".
function formatRelative(iso: string | null): string {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const diffSec = Math.round((t - Date.now()) / 1000)
  const abs = Math.abs(diffSec)
  // Rango legible: minutos / horas / días. Mas alla de 30d mostramos
  // la fecha absoluta para no quedar en "hace 45 días" en un campo
  // pequeno.
  if (abs < 60) return diffSec >= 0 ? 'recién' : 'hace instantes'
  if (abs < 3600) {
    const m = Math.round(diffSec / 60)
    return new Intl.RelativeTimeFormat('es-AR', { numeric: 'auto' }).format(m, 'minute')
  }
  if (abs < 86400) {
    const h = Math.round(diffSec / 3600)
    return new Intl.RelativeTimeFormat('es-AR', { numeric: 'auto' }).format(h, 'hour')
  }
  if (abs < 30 * 86400) {
    const d = Math.round(diffSec / 86400)
    return new Intl.RelativeTimeFormat('es-AR', { numeric: 'auto' }).format(d, 'day')
  }
  return new Date(iso).toLocaleDateString('es-AR')
}

function StatsCard({
  label,
  value,
  loading,
  testId,
}: {
  label: string
  value: number
  loading: boolean
  testId: string
}) {
  return (
    <Card withBorder padding="md" radius="md" data-testid={testId}>
      <Stack gap={4}>
        <Text size="sm" c="dimmed">
          {label}
        </Text>
        {loading ? (
          <Skeleton height={28} width={60} />
        ) : (
          <Text size="xl" fw={700}>
            {value}
          </Text>
        )}
      </Stack>
    </Card>
  )
}

function WarningsTableSkeleton() {
  return (
    <Stack gap="xs">
      {[0, 1, 2].map((i) => (
        <Skeleton key={i} height={36} />
      ))}
    </Stack>
  )
}

export default function GroupModerationPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = Number(id)

  const [period, setPeriod] = useState<StatsPeriod>('24h')
  const [resetTarget, setResetTarget] = useState<WarningStateRow | null>(null)

  const warnings = useWarnings(groupId)
  const stats = useStats(groupId, period)
  const resetMutation = useResetWarning(groupId)
  const qc = useQueryClient()

  const handleRefresh = () => {
    qc.invalidateQueries({ queryKey: ['automation', 'warnings', groupId] })
    qc.invalidateQueries({ queryKey: ['automation', 'stats', groupId, period] })
  }

  const handleResetConfirm = () => {
    if (!resetTarget) return
    const target = resetTarget
    setResetTarget(null)
    resetMutation.mutate(target.user_id, {
      onError: (e) => {
        notifications.show({
          color: 'red',
          title: 'No se pudo resetear',
          message: formatDashboardError(e),
        })
      },
    })
  }

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Anchor component={Link} to={`/groups/${groupId}`} size="sm">
          ← Volver al grupo
        </Anchor>
        <Box>
          <Title order={2}>Dashboard de moderación</Title>
          <Text size="sm" c="dimmed">
            Observación de la moderación automática: contadores de las
            últimas horas/días y advertencias activas por usuario.
          </Text>
        </Box>

        {/* ── Sección 1 — Estadísticas ───────────────────────────── */}
        <Stack gap="xs">
          <Group justify="space-between" align="center">
            <Title order={4}>Estadísticas</Title>
            <Group gap="xs">
              <Select
                data={PERIOD_OPTIONS}
                value={period}
                onChange={(v) => {
                  if (v === '24h' || v === '7d') setPeriod(v)
                }}
                allowDeselect={false}
                w={180}
                data-testid="stats-period-select"
                aria-label="Período"
              />
              <Button
                variant="default"
                leftSection={<IconRefresh size={16} />}
                onClick={handleRefresh}
                data-testid="stats-refresh-button"
              >
                Refrescar
              </Button>
            </Group>
          </Group>

          {stats.isError ? (
            <Alert color="red" variant="light" title="Error">
              <Text size="sm">
                {stats.error instanceof Error
                  ? stats.error.message
                  : 'No se pudieron cargar las estadísticas.'}
              </Text>
            </Alert>
          ) : null}

          <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="md">
            <StatsCard
              label="Reglas disparadas"
              value={stats.data?.rule_triggered ?? 0}
              loading={stats.isPending}
              testId="stats-card-rule-triggered"
            />
            <StatsCard
              label="Auto-mute"
              value={stats.data?.automute ?? 0}
              loading={stats.isPending}
              testId="stats-card-automute"
            />
            <StatsCard
              label="Auto-ban"
              value={stats.data?.autoban ?? 0}
              loading={stats.isPending}
              testId="stats-card-autoban"
            />
          </SimpleGrid>
        </Stack>

        {/* ── Sección 2 — Advertencias activas ────────────────────── */}
        <Stack gap="xs">
          <Title order={4}>Advertencias activas</Title>

          {warnings.isError ? (
            <Alert color="red" variant="light" title="Error">
              <Text size="sm">
                {warnings.error instanceof Error
                  ? warnings.error.message
                  : 'No se pudieron cargar las advertencias.'}
              </Text>
            </Alert>
          ) : null}

          {warnings.data?.truncated ? (
            <Alert color="yellow" variant="light" data-testid="warnings-truncated-alert">
              Mostrando las primeras 100 advertencias activas.
            </Alert>
          ) : null}

          {warnings.isPending ? (
            <WarningsTableSkeleton />
          ) : null}

          {!warnings.isPending && warnings.data && warnings.data.warnings.length === 0 ? (
            <Text c="dimmed" ta="center" data-testid="warnings-empty-state">
              No hay advertencias activas.
            </Text>
          ) : null}

          {!warnings.isPending && warnings.data && warnings.data.warnings.length > 0 ? (
            <Table withTableBorder data-testid="warnings-table">
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Usuario</Table.Th>
                  <Table.Th>Advertencias</Table.Th>
                  <Table.Th>Última advertencia</Table.Th>
                  <Table.Th>Acciones</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {warnings.data.warnings.map((w) => (
                  <Table.Tr key={w.user_id} data-testid={`warning-row-${w.user_id}`}>
                    <Table.Td>
                      <Stack gap={2}>
                        <Text fw={500}>{w.display_name}</Text>
                        <Text size="xs" c="dimmed">
                          id: {w.user_id}
                          {w.username ? ` · @${w.username}` : ''}
                        </Text>
                      </Stack>
                    </Table.Td>
                    <Table.Td>
                      <Badge variant="light" color={w.warning_count >= 5 ? 'red' : 'yellow'}>
                        {w.warning_count}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{formatRelative(w.last_warning_at)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Button
                        size="xs"
                        variant="default"
                        leftSection={<IconRefresh size={14} />}
                        onClick={() => setResetTarget(w)}
                        data-testid={`warning-reset-${w.user_id}`}
                      >
                        Reset
                      </Button>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          ) : null}
        </Stack>
      </Stack>

      {/* ── Modal de confirmacion de reset ──────────────────────────── */}
      <Modal
        opened={resetTarget !== null}
        onClose={() => setResetTarget(null)}
        title="Resetear advertencias"
        centered
        data-testid="reset-warning-modal"
      >
        {resetTarget ? (
          <Stack gap="md">
            <Text>
              ¿Resetear las advertencias de <strong>{resetTarget.display_name}</strong>?
              Esto limpia el contador en la base de datos. <strong>No desmutea</strong>
              {' '}al usuario en Telegram — si querés desmutearlo, usá la sección de
              membresía y moderación.
            </Text>
            <Group justify="flex-end" gap="xs">
              <Button variant="default" onClick={() => setResetTarget(null)} data-testid="reset-warning-cancel">
                Cancelar
              </Button>
              <Button
                color="red"
                variant="filled"
                loading={resetMutation.isPending}
                onClick={handleResetConfirm}
                data-testid="reset-warning-confirm"
              >
                Resetear
              </Button>
            </Group>
          </Stack>
        ) : null}
      </Modal>
    </Container>
  )
}