// Vista de usuarios del grupo (spec frontend-pages-moderation REQ-1..3):
// renderiza los administradores como Cards dentro de SimpleGrid (mismo
// patron que DashboardPage), lookup puntual por telegram_id, y 4 botones
// de accion por usuario. Banear es destructiva irreversible (AGENTS 4):
// se confirma con <Modal> en vez de window.confirm. Mutear / Desbanear /
// Desmutear son reversibles: clic directo + notifySuccess. Las mutaciones
// invierten las queries correctas via useBanUser/useUnbanUser/useMuteUser
// /useUnmuteUser (call-sites only — features/moderation/hooks.ts intacto).
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import {
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  Container,
  Group,
  Modal,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  TextInput,
  Title,
} from '@mantine/core'
import { IconSearch } from '@tabler/icons-react'
import { formatModerationError } from '../features/moderation/error'
import {
  useBanUser,
  useGroupUser,
  useGroupUsers,
  useMuteUser,
  useUnbanUser,
  useUnmuteUser,
} from '../features/moderation/hooks'
import type { GroupUser } from '../features/moderation/types'
import { notifyError, notifySuccess } from '../lib/notifications'

// Color del Badge por status (memberResponse.status del backend).
const STATUS_COLOR: Record<string, string> = {
  administrator: 'blue',
  creator: 'gray',
  member: 'gray',
  restricted: 'yellow',
  left: 'yellow',
  kicked: 'yellow',
}

function statusColor(status: string): string {
  return STATUS_COLOR[status] ?? 'gray'
}

function permissionsBadges(user: GroupUser) {
  const flags: Array<[string, string]> = [
    ['Restringir', 'can_restrict_members'],
    ['Borrar', 'can_delete_messages'],
    ['Fijar', 'can_pin_messages'],
    ['Invitar', 'can_invite_users'],
  ]
  return flags.map(([label, key]) => {
    const allowed = (user as unknown as Record<string, boolean | null>)[key]
    return (
      <Badge
        key={key}
        variant="light"
        color={allowed ? 'blue' : 'gray'}
        size="sm"
      >
        {label}
      </Badge>
    )
  })
}

function UserCard({ groupId, user, onAskBan }: { groupId: string; user: GroupUser; onAskBan: (user: GroupUser) => void }) {
  // Ban se dispara desde el Modal (estado en el padre). Las otras 3
  // mutaciones viven en la propia card.
  const unban = useUnbanUser()
  const mute = useMuteUser()
  const unmute = useUnmuteUser()

  const pending =
    unban.isPending || mute.isPending || unmute.isPending

  const run = (
    mut: typeof mute,
    successText: string,
    args: { groupId: string; userId: number },
  ) => {
    mut.mutate(args, {
      onSuccess: () => notifySuccess(successText),
      onError: (e) => notifyError(formatModerationError(e)),
    })
  }

  return (
    <Card withBorder padding="md" radius="md" data-testid="user-card">
      <Stack gap="xs">
        <Group gap="sm" wrap="nowrap" align="center">
          <Avatar color="blue" radius="xl" size="md">
            {user.first_name.slice(0, 1).toUpperCase()}
          </Avatar>
          <Stack gap={2}>
            <Text fw={500}>{user.first_name}</Text>
            <Text size="xs" c="dimmed">
              ID: {user.user_id}
              {user.username ? ` · @${user.username}` : ''}
            </Text>
          </Stack>
        </Group>

        <Group gap="xs">
          <Badge color={statusColor(user.status)} variant="light">
            {user.status}
          </Badge>
        </Group>

        <Group gap={6}>{permissionsBadges(user)}</Group>

        <Group gap="xs" mt="xs">
          <Button
            size="xs"
            color="red"
            variant="filled"
            disabled={pending}
            onClick={() => onAskBan(user)}
          >
            Banear
          </Button>
          <Button
            size="xs"
            color="yellow"
            variant="filled"
            disabled={pending}
            onClick={() => run(mute, 'Usuario muteado', { groupId, userId: user.user_id })}
          >
            Mutear
          </Button>
          <Button
            size="xs"
            variant="light"
            disabled={pending}
            onClick={() => run(unban, 'Usuario desbaneado', { groupId, userId: user.user_id })}
          >
            Desbanear
          </Button>
          <Button
            size="xs"
            variant="light"
            disabled={pending}
            onClick={() => run(unmute, 'Usuario desmuteado', { groupId, userId: user.user_id })}
          >
            Desmutear
          </Button>
        </Group>
      </Stack>
    </Card>
  )
}

function LookupResultCard({ groupId, user, onAskBan }: { groupId: string; user: GroupUser; onAskBan: (user: GroupUser) => void }) {
  return (
    <Stack gap="xs">
      <Title order={4}>Resultado de búsqueda</Title>
      <UserCard groupId={groupId} user={user} onAskBan={onAskBan} />
    </Stack>
  )
}

function UsersSkeleton() {
  return (
    <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
      {[0, 1, 2].map((i) => (
        <Card key={i} withBorder padding="md" radius="md">
          <Skeleton height={56} mb="sm" />
          <Skeleton height={16} width="40%" mb="xs" />
          <Skeleton height={16} width="80%" mb="md" />
          <Skeleton height={28} width={120} />
        </Card>
      ))}
    </SimpleGrid>
  )
}

export default function GroupUsersPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const [lookupId, setLookupId] = useState('')
  const [lookupSubmitted, setLookupSubmitted] = useState('')
  const [banTarget, setBanTarget] = useState<GroupUser | null>(null)

  const users = useGroupUsers(groupId)
  const lookup = useGroupUser(groupId, lookupSubmitted)

  // Hook a nivel de padre para que el Modal pueda disparar la mutacion
  // sin acoplarse al UserCard (mismo patron que UserCard.run() pero con
  // estado controlado).
  const banMutation = useBanUser()

  const confirmBan = () => {
    if (!banTarget) return
    const target = banTarget
    setBanTarget(null)
    banMutation.mutate(
      { groupId, userId: target.user_id },
      {
        onSuccess: () => notifySuccess('Usuario baneado'),
        onError: (e) => notifyError(formatModerationError(e)),
      },
    )
  }

  return (
    <Container size="lg" py="md">
      <Stack gap="md">
        <Stack gap={4}>
          <Title order={2}>Membresía y moderación</Title>
          <Text size="sm" c="dimmed">
            Telegram no expone la lista completa de miembros. Acá se muestran los
            administradores del grupo y se puede buscar cualquier miembro por su ID de
            Telegram para moderarlo (banear, mutear, desbanear, desmutear).
          </Text>
        </Stack>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            setLookupSubmitted(lookupId.trim())
          }}
        >
          <Group gap="xs" wrap="nowrap" align="end">
            <TextInput
              label="Buscar usuario por ID de Telegram"
              placeholder="ID de Telegram del usuario"
              leftSection={<IconSearch size={16} />}
              value={lookupId}
              onChange={(e) => setLookupId(e.currentTarget.value)}
              inputMode="numeric"
              style={{ flex: 1 }}
            />
            <Button type="submit">Buscar</Button>
          </Group>
        </form>

        {lookupSubmitted ? (
          <Stack gap="xs">
            {lookup.isPending ? <Skeleton height={120} /> : null}
            {lookup.isError ? (
              <Alert color="red" variant="light" title="No se pudo consultar al usuario">
                {lookup.error instanceof Error
                  ? lookup.error.message
                  : 'No se pudo consultar al usuario.'}
              </Alert>
            ) : null}
            {lookup.data ? (
              <LookupResultCard
                groupId={groupId}
                user={lookup.data}
                onAskBan={setBanTarget}
              />
            ) : null}
          </Stack>
        ) : null}

        <Title order={4}>Administradores</Title>

        {users.isPending ? <UsersSkeleton /> : null}

        {users.isError ? (
          <Alert color="red" variant="light" title="Error">
            <Stack gap="xs">
              <Text size="sm">
                {users.error instanceof Error
                  ? users.error.message
                  : 'No se pudieron cargar los usuarios.'}
              </Text>
              <Button variant="default" w={140} onClick={() => users.refetch()}>
                Reintentar
              </Button>
            </Stack>
          </Alert>
        ) : null}

        {!users.isPending && !users.isError && users.data && users.data.length === 0 ? (
          <Text c="dimmed">
            Sin administradores visibles. Telegram no expone la lista completa de miembros:
            el panel muestra los administradores y permite consultar usuarios puntuales por ID.
          </Text>
        ) : null}

        {!users.isPending && !users.isError && users.data && users.data.length > 0 ? (
          <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="md">
            {users.data.map((u) => (
              <UserCard key={u.user_id} groupId={groupId} user={u} onAskBan={setBanTarget} />
            ))}
          </SimpleGrid>
        ) : null}
      </Stack>

      <Modal
        opened={banTarget !== null}
        onClose={() => setBanTarget(null)}
        title="Confirmar baneo"
        centered
      >
        {banTarget ? (
          <Stack gap="md">
            <Text>
              ¿Banear a {banTarget.first_name}? Esta acción es irreversible.
            </Text>
            <Group justify="flex-end" gap="xs">
              <Button variant="default" onClick={() => setBanTarget(null)}>
                Cancelar
              </Button>
              <Button
                color="red"
                variant="filled"
                data-testid="confirm-ban"
                loading={banMutation.isPending}
                onClick={confirmBan}
              >
                Banear
              </Button>
            </Group>
          </Stack>
        ) : null}
      </Modal>
    </Container>
  )
}