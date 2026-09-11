import { useEffect, useState } from 'react'
import {
  Paper, Title, Text, Table, Select, Group, Badge, Loader, Stack,
} from '@mantine/core'
import { adminListTenants } from '../features/admin/api'
import type { AdminTenant } from '../features/admin/types'
import { ApiError } from '../lib/api-client'

const STATUS_OPTIONS = [
  { value: '', label: 'Todos' },
  { value: 'trial', label: 'Prueba' },
  { value: 'active', label: 'Activa' },
  { value: 'suspended', label: 'Suspendida' },
  { value: 'expired', label: 'Expirada' },
]

function statusColor(status: string): string {
  switch (status) {
    case 'active': return 'green'
    case 'trial': return 'blue'
    case 'suspended': return 'red'
    case 'expired': return 'gray'
    default: return 'dimmed'
  }
}

export default function AdminTenantsPage() {
  const [tenants, setTenants] = useState<AdminTenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const data = await adminListTenants()
        if (!cancelled) setTenants(data)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Error al cargar tenants')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [])

  const filtered = filter
    ? tenants.filter((t) => t.status === filter)
    : tenants

  if (loading) return <Loader />
  if (error) return <Text c="red">{error}</Text>

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Title order={3}>Admin Panel — Tenants</Title>
        <Select
          data={STATUS_OPTIONS}
          value={filter}
          onChange={(v) => setFilter(v ?? '')}
          placeholder="Filtrar por estado"
          clearable
          w={200}
        />
      </Group>

      <Paper withBorder>
        <Table striped highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Slug</Table.Th>
              <Table.Th>Plan</Table.Th>
              <Table.Th>Estado</Table.Th>
              <Table.Th>Trial hasta</Table.Th>
              <Table.Th>Expira</Table.Th>
              <Table.Th>Bot</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {filtered.map((t) => (
              <Table.Tr key={t.id}>
                <Table.Td>{t.slug}</Table.Td>
                <Table.Td>{t.plan}</Table.Td>
                <Table.Td>
                  <Badge color={statusColor(t.status)}>{t.status}</Badge>
                </Table.Td>
                <Table.Td>
                  {t.trial_ends_at
                    ? new Date(t.trial_ends_at).toLocaleDateString('es-AR')
                    : '-'}
                </Table.Td>
                <Table.Td>
                  {t.expires_at
                    ? new Date(t.expires_at).toLocaleDateString('es-AR')
                    : '-'}
                </Table.Td>
                <Table.Td>{t.bot_username ?? '-'}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>
    </Stack>
  )
}
