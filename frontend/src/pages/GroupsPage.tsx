// GroupsPage migrado a Mantine (frontend-refresh slice 1 — design D13,
// spec REQ-8). Implementacion real con `<Table>` (no alias de Dashboard):
// escala mejor cuando hay muchos grupos. Columnas: Titulo, Tipo,
// Miembros, Bot (Badge), Acciones (boton "Administrar" al detalle).
// Loading: filas Skeleton. Vacio: Text "Todavia no hay grupos".
import { Link } from 'react-router-dom'
import {
  Badge,
  Button,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
} from '@mantine/core'
import { formatPermissions } from '../features/groups/permissions'
import { useGroups } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

function GroupsTableSkeleton() {
  return (
    <Table verticalSpacing="sm" striped highlightOnHover>
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Título</Table.Th>
          <Table.Th>Tipo</Table.Th>
          <Table.Th>Miembros</Table.Th>
          <Table.Th>Bot</Table.Th>
          <Table.Th>Acciones</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {[0, 1, 2].map((i) => (
          <Table.Tr key={i}>
            <Table.Td>
              <Skeleton height={16} width="70%" />
            </Table.Td>
            <Table.Td>
              <Skeleton height={16} width="60%" />
            </Table.Td>
            <Table.Td>
              <Skeleton height={16} width="50%" />
            </Table.Td>
            <Table.Td>
              <Skeleton height={20} width={80} />
            </Table.Td>
            <Table.Td>
              <Skeleton height={28} width={110} />
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )
}

export default function GroupsPage() {
  const { data: groups, isPending, isError } = useGroups()

  return (
    <Stack gap="md">
      <Title order={2}>Grupos</Title>

      {isPending ? <GroupsTableSkeleton /> : null}

      {isError ? (
        <Text c="red">No se pudieron cargar los grupos. Intente nuevamente.</Text>
      ) : null}

      {!isPending && !isError && groups && groups.length === 0 ? (
        <Text c="dimmed">Todavía no hay grupos administrables.</Text>
      ) : null}

      {!isPending && !isError && groups && groups.length > 0 ? (
        <Table verticalSpacing="sm" striped highlightOnHover withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Título</Table.Th>
              <Table.Th>Tipo</Table.Th>
              <Table.Th>Miembros</Table.Th>
              <Table.Th>Bot</Table.Th>
              <Table.Th>Permisos</Table.Th>
              <Table.Th>Acciones</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {groups.map((g) => {
              const perms = formatPermissions(g.bot_permissions)
              return (
                <Table.Tr key={g.id}>
                  <Table.Td>
                    <Text fw={600}>{g.title}</Text>
                    {g.username ? (
                      <Text size="xs" c="dimmed">
                        @{g.username}
                      </Text>
                    ) : null}
                  </Table.Td>
                  <Table.Td>{g.type}</Table.Td>
                  <Table.Td>{formatMembers(g.member_count)}</Table.Td>
                  <Table.Td>
                    <Badge variant="light" color="blue">
                      {g.bot_status}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" c="dimmed">
                      {perms.length > 0 ? perms.join(', ') : 'Sin permisos'}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Button
                      component={Link}
                      to={`/groups/${g.telegram_id}`}
                      variant="subtle"
                      size="xs"
                    >
                      Administrar
                    </Button>
                  </Table.Td>
                </Table.Tr>
              )
            })}
          </Table.Tbody>
        </Table>
      ) : null}
    </Stack>
  )
}