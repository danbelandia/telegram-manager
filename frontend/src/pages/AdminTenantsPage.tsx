import { Component, type ReactNode, useEffect, useState } from 'react'
import {
  ActionIcon,
  Badge,
  Button,
  Group,
  Loader,
  Modal,
  Paper,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from '@mantine/core'
import { DateTimePicker } from '@mantine/dates'
import { IconEdit } from '@tabler/icons-react'
import { adminGetTenant, adminListTenants, adminUpdateTenant } from '../features/admin/api'
import type { AdminTenant } from '../features/admin/types'
import { ApiError } from '../lib/api-client'

const STATUS_OPTIONS = [
  { value: '', label: 'Todos' },
  { value: 'trial', label: 'Prueba' },
  { value: 'active', label: 'Activa' },
  { value: 'suspended', label: 'Suspendida' },
  { value: 'expired', label: 'Expirada' },
]

const PLAN_OPTIONS = [
  { value: 'pro', label: 'Pro' },
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

/** Transiciones validas de status (replica del backend). */
const VALID_TRANSITIONS: Record<string, string[]> = {
  trial: ['active', 'suspended', 'expired'],
  active: ['suspended'],
  suspended: ['active'],
  expired: ['active'],
}

function editableStatuses(current: string): { value: string; label: string }[] {
  const allowed = VALID_TRANSITIONS[current] ?? []
  return [
    { value: current, label: `${current} (actual)` },
    ...allowed.map((s) => ({ value: s, label: s })),
  ]
}

/** Temporary error boundary to catch and display the crash cause. */
class ModalErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: 16, background: '#fff3f3', borderRadius: 8, border: '1px solid red' }}>
          <Text fw={700} c="red">Error en el modal:</Text>
          <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{this.state.error.message}</pre>
          <pre style={{ whiteSpace: 'pre-wrap', fontSize: 10, color: '#666' }}>{this.state.error.stack}</pre>
        </div>
      )
    }
    return this.props.children
  }
}

/** Parse ISO string to Date, or null. */
function parseISO(s: string | null): Date | null {
  if (!s) return null
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? null : d
}

export default function AdminTenantsPage() {
  const [tenants, setTenants] = useState<AdminTenant[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState('')

  // Edit modal state
  const [editing, setEditing] = useState<AdminTenant | null>(null)
  const [editForm, setEditForm] = useState({
    status: '',
    plan: '',
    trial_ends_at: null as Date | null,
    expires_at: null as Date | null,
    max_groups: -1,
    max_messages_day: -1,
  })
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  const loadTenants = async () => {
    try {
      const data = await adminListTenants()
      setTenants(data)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Error al cargar tenants')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadTenants()
  }, [])

  const openEdit = async (tenant: AdminTenant) => {
    setSaveError(null)
    try {
      // Fetch full detail (same data for now, but future-proof).
      const detail = await adminGetTenant(tenant.id)
      setEditing(detail)
      setEditForm({
        status: detail.status,
        plan: detail.plan,
        trial_ends_at: parseISO(detail.trial_ends_at),
        expires_at: parseISO(detail.expires_at),
        max_groups: detail.max_groups,
        max_messages_day: detail.max_messages_day,
      })
    } catch {
      setEditing(tenant)
      setEditForm({
        status: tenant.status,
        plan: tenant.plan,
        trial_ends_at: parseISO(tenant.trial_ends_at),
        expires_at: parseISO(tenant.expires_at),
        max_groups: tenant.max_groups,
        max_messages_day: tenant.max_messages_day,
      })
    }
  }

  const handleSave = async () => {
    if (!editing) return
    setSaving(true)
    setSaveError(null)
    try {
      await adminUpdateTenant(editing.id, {
        status: editForm.status,
        plan: editForm.plan,
        trial_ends_at: editForm.trial_ends_at ? editForm.trial_ends_at.toISOString() : null,
        expires_at: editForm.expires_at ? editForm.expires_at.toISOString() : null,
        max_groups: editForm.max_groups,
        max_messages_day: editForm.max_messages_day,
      })
      setEditing(null)
      await loadTenants()
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : 'Error al guardar')
    } finally {
      setSaving(false)
    }
  }

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
              <Table.Th>Grupos</Table.Th>
              <Table.Th>Msg/día</Table.Th>
              <Table.Th />
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
                <Table.Td>{t.max_groups === -1 ? '∞' : t.max_groups}</Table.Td>
                <Table.Td>{t.max_messages_day === -1 ? '∞' : t.max_messages_day}</Table.Td>
                <Table.Td>
                  <ActionIcon variant="subtle" onClick={() => openEdit(t)}>
                    <IconEdit size={16} />
                  </ActionIcon>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Paper>

      {/* Edit modal */}
      <Modal
        opened={editing !== null}
        onClose={() => setEditing(null)}
        title={editing ? `Editar tenant: ${editing.slug}` : ''}
        size="md"
      >
        <ModalErrorBoundary>
        <Stack gap="md">
          <Select
            label="Estado"
            data={editing ? editableStatuses(editing.status) : []}
            value={editForm.status}
            onChange={(v) => { if (v) setEditForm((f) => ({ ...f, status: v })) }}
            allowDeselect={false}
            comboboxProps={{ withinPortal: false }}
          />

          <Select
            label="Plan"
            data={PLAN_OPTIONS}
            value={editForm.plan}
            onChange={(v) => { if (v) setEditForm((f) => ({ ...f, plan: v })) }}
            allowDeselect={false}
            comboboxProps={{ withinPortal: false }}
          />

          <DateTimePicker
            label="Trial hasta"
            value={editForm.trial_ends_at}
            onChange={(d) => setEditForm((f) => ({ ...f, trial_ends_at: d }))}
            valueFormat="DD/MM/YYYY HH:mm"
            clearable
          />

          <DateTimePicker
            label="Expira"
            value={editForm.expires_at}
            onChange={(d) => setEditForm((f) => ({ ...f, expires_at: d }))}
            valueFormat="DD/MM/YYYY HH:mm"
            clearable
          />

          <TextInput
            label="Máximo de grupos (-1 = ilimitado)"
            type="number"
            value={String(editForm.max_groups)}
            onChange={(e) => setEditForm((f) => ({ ...f, max_groups: Number(e.target.value) }))}
          />

          <TextInput
            label="Máximo mensajes por día (-1 = ilimitado)"
            type="number"
            value={String(editForm.max_messages_day)}
            onChange={(e) => setEditForm((f) => ({ ...f, max_messages_day: Number(e.target.value) }))}
          />

          {saveError && <Text c="red" size="sm">{saveError}</Text>}

          <Group justify="flex-end">
            <Button variant="subtle" onClick={() => setEditing(null)}>Cancelar</Button>
            <Button loading={saving} onClick={handleSave}>Guardar</Button>
          </Group>
        </Stack>
        </ModalErrorBoundary>
      </Modal>
    </Stack>
  )
}
