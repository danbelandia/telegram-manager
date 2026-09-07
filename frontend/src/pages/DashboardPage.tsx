// Dashboard migrado a Mantine (frontend-refresh slice 1 — design REQ-7).
// SimpleGrid de Cards por grupo (titulo, tipo, miembros, permisos, bot
// status, boton "Administrar"). Estados: Skeleton en loading, Alert de
// error con reintento, Text vacio. Conserva los textos que las pruebas
// existentes buscan (titulo, @username, "ID: -100...", "Miembros: 4.821",
// permisos formateados, link Administrar → /groups/:telegram_id).
import { Link } from 'react-router-dom'
import {
  Alert,
  Badge,
  Button,
  Card,
  Group,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  Title,
} from '@mantine/core'
import { formatPermissions } from '../features/groups/permissions'
import { useGroups } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

function GroupPermissions({ permissions }: { permissions: Record<string, boolean> }) {
  const active = formatPermissions(permissions)
  if (active.length === 0) return <Text size="sm">Sin permisos</Text>
  return (
    <Text size="sm" c="dimmed">
      {active.join(', ')}
    </Text>
  )
}

function DashboardSkeleton() {
  return (
    <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
      {[0, 1, 2].map((i) => (
        <Card key={i} withBorder padding="md" radius="md">
          <Skeleton height={20} width="60%" mb="sm" />
          <Skeleton height={14} width="40%" mb="xs" />
          <Skeleton height={14} width="80%" mb="md" />
          <Skeleton height={32} width={120} />
        </Card>
      ))}
    </SimpleGrid>
  )
}

export default function DashboardPage() {
  const { data: groups, isPending, isError, error, refetch } = useGroups()

  return (
    <Stack gap="md">
      <Title order={2}>Dashboard</Title>

      {isPending ? <DashboardSkeleton /> : null}

      {isError ? (
        <Alert color="red" variant="light" title="Error">
          <Stack gap="xs">
            <Text size="sm">
              {error instanceof Error
                ? error.message
                : 'No se pudieron cargar los grupos. Intente de nuevo.'}
            </Text>
            <Button variant="default" w={140} onClick={() => refetch()}>
              Reintentar
            </Button>
          </Stack>
        </Alert>
      ) : null}

      {!isPending && !isError && groups && groups.length === 0 ? (
        <Text c="dimmed">
          Todavía no hay grupos. Añadí el bot a un grupo y dale permisos de administrador.
        </Text>
      ) : null}

      {!isPending && !isError && groups && groups.length > 0 ? (
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
          {groups.map((g) => (
            <Card key={g.id} withBorder padding="md" radius="md" data-testid="group-card">
              <Stack gap={4} mb="sm">
                <Group gap="xs" wrap="nowrap" align="baseline">
                  <Title order={4} fw={600}>
                    {g.title}
                  </Title>
                  {g.username ? (
                    <Text size="sm" c="dimmed">
                      @{g.username}
                    </Text>
                  ) : null}
                </Group>
                <Text size="sm" c="dimmed">
                  ID: {g.telegram_id}
                </Text>
                <Text size="sm" c="dimmed">
                  Miembros: {formatMembers(g.member_count)}
                </Text>
                <Group gap="xs">
                  <Badge variant="light" color="blue">
                    Bot: {g.bot_status}
                  </Badge>
                </Group>
                <GroupPermissions permissions={g.bot_permissions} />
              </Stack>
              <Button component={Link} to={`/groups/${g.telegram_id}`} variant="light">
                Administrar
              </Button>
            </Card>
          ))}
        </SimpleGrid>
      ) : null}
    </Stack>
  )
}