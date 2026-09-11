import { Paper, Title, Text, Stack, Group, Badge } from '@mantine/core'
import type { TenantMe } from '../features/tenant/types'

interface LicenseCardProps {
  tenant: TenantMe
}

function statusColor(status: string): string {
  switch (status) {
    case 'active': return 'green'
    case 'trial': return 'blue'
    case 'suspended': return 'red'
    case 'expired': return 'gray'
    default: return 'dimmed'
  }
}

function statusLabel(status: string): string {
  switch (status) {
    case 'active': return 'Activa'
    case 'trial': return 'Prueba'
    case 'suspended': return 'Suspendida'
    case 'expired': return 'Expirada'
    default: return status
  }
}

export default function LicenseCard({ tenant }: LicenseCardProps) {
  const trialRemaining = tenant.trial_ends_at
    ? Math.max(0, Math.ceil(
        (new Date(tenant.trial_ends_at).getTime() - Date.now()) / (1000 * 60 * 60 * 24)
      ))
    : null

  return (
    <Paper p="md" withBorder>
      <Title order={4} mb="md">Licencia</Title>
      <Stack gap="xs">
        <Group>
          <Text fw={600}>Estado:</Text>
          <Badge color={statusColor(tenant.license_status)}>
            {statusLabel(tenant.license_status)}
          </Badge>
        </Group>
        <Group>
          <Text fw={600}>Plan:</Text>
          <Text>{tenant.plan}</Text>
        </Group>
        {tenant.license_status === 'trial' && trialRemaining !== null && (
          <Group>
            <Text fw={600}>Dias restantes:</Text>
            <Text>{trialRemaining}</Text>
          </Group>
        )}
        {tenant.trial_ends_at && (
          <Group>
            <Text fw={600}>Trial hasta:</Text>
            <Text>{new Date(tenant.trial_ends_at).toLocaleDateString('es-AR')}</Text>
          </Group>
        )}
        {tenant.expires_at && (
          <Group>
            <Text fw={600}>Expira:</Text>
            <Text>{new Date(tenant.expires_at).toLocaleDateString('es-AR')}</Text>
          </Group>
        )}
        <Group>
          <Text fw={600}>Max grupos:</Text>
          <Text>{tenant.max_groups === -1 ? 'Sin limite' : tenant.max_groups}</Text>
        </Group>
        <Group>
          <Text fw={600}>Max mensajes/dia:</Text>
          <Text>{tenant.max_messages_day === -1 ? 'Sin limite' : tenant.max_messages_day}</Text>
        </Group>
      </Stack>
    </Paper>
  )
}
