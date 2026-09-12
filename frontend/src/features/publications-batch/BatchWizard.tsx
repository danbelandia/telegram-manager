// BatchWizard — modal Mantine v7 para enviar N publicaciones en una
// sola request (publications-batch change, spec REQ-21..24 + REQ-25).
// Reusa helpers de features/publications/ (sin duplicar validaciones).
//
// Estructura del modal (state machine):
//   - editing: lista de slots editables + Alert resumen + submit.
//   - submitting: spinner; submit deshabilitado.
//   - result: 2 Alert separados (verde/rojo) + botones Reintentar/Cerrar.
//
// Per slot: Textarea + foto URL + ButtonsEditor (reusado) + multi-grupo
// via Checkbox + datetime-local opcional. Cada slot mantiene su propio
// estado local; el slot ES la unidad atomica del request batch.

import { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Group,
  Loader,
  Modal,
  Paper,
  Stack,
  Text,
  Textarea,
  TextInput,
  Title,
} from '@mantine/core'
import { DateTimePicker } from '@mantine/dates'
import { IconCalendar, IconPlus, IconSend, IconTrash, IconX } from '@tabler/icons-react'
import {
  formatPublicationsError,
  validateButtonsClient,
  validateGroupIdsClient,
  validatePhotoUrlClient,
} from '../publications/error'
import {
  validateScheduledAtClient,
} from '../publications/validateScheduledAtClient'
import { dateToLocalInput } from '../publications/dateHelpers'
import type {
  InlineButton,
  Publication,
} from '../publications/types'
import { useCreatePublicationBatch } from './hooks'
import type {
  BatchItemInput,
  BatchFailedItem,
  BatchResponse,
} from './types'
import ButtonsEditor from '../publications/ButtonsEditor'
import { useGroups } from '../groups/hooks'

/** Cap maximo por batch (server-side REQ-16). */
const MAX_BATCH_SIZE = 10
/** Default slots al abrir el modal (REQ-21 escenario 1). */
const DEFAULT_SLOT_COUNT = 2

/** Forma interna de un slot en el modal (estado editable del form). */
interface SlotDraft {
  text: string
  photoUrl: string
  buttons: InlineButton[][]
  groupIds: number[]
  scheduledDate: Date | null
}

function emptySlot(): SlotDraft {
  return {
    text: '',
    photoUrl: '',
    buttons: [],
    groupIds: [],
    scheduledDate: null,
  }
}

function slotToInput(s: SlotDraft): BatchItemInput {
  const trimmedText = s.text.trim()
  const photoUrl = s.photoUrl.trim()
  const buttons = nonEmptyRows(s.buttons)
  const item: BatchItemInput = {
    text: trimmedText,
    group_ids: s.groupIds,
  }
  if (photoUrl) item.photo_url = photoUrl
  if (buttons.length > 0) item.buttons = buttons
  if (s.scheduledDate) {
    const res = validateScheduledAtClient(dateToLocalInput(s.scheduledDate), new Date())
    if (res.ok) item.scheduled_at = res.iso
  }
  return item
}

function nonEmptyRows(rows: InlineButton[][]): InlineButton[][] {
  return rows
    .map((row) => row.filter((btn) => btn.text.trim() || btn.url.trim()))
    .filter((row) => row.length > 0)
}

function truncate(text: string, max = 60): string {
  return text.length > max ? `${text.slice(0, max)}…` : text
}

function formatScheduledPreview(date: Date | null): string | null {
  if (!date) return null
  return date.toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

interface BatchWizardProps {
  opened: boolean
  onClose: () => void
}

/**
 * BatchWizard — modal de publicacion en lote. Vive como componente
 * autocontenido; PublicationsPage solo lo monta y maneja opened/onClose.
 */
export default function BatchWizard({ opened, onClose }: BatchWizardProps) {
  const groups = useGroups()
  const [slots, setSlots] = useState<SlotDraft[]>(
    () => Array.from({ length: DEFAULT_SLOT_COUNT }, () => emptySlot()),
  )
  const [result, setResult] = useState<BatchResponse | null>(null)
  const [attempt, setAttempt] = useState(0)
  const create = useCreatePublicationBatch()

  // Reset al cerrar: limpiar slots, resultado y contador de intentos.
  useEffect(() => {
    if (!opened) {
      setSlots(Array.from({ length: DEFAULT_SLOT_COUNT }, () => emptySlot()))
      setResult(null)
      setAttempt(0)
      create.reset()
    }
    // create.reset es estable; el effect solo debe correr al cambiar opened.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened])

  const addSlot = () => {
    if (slots.length >= MAX_BATCH_SIZE) return
    setSlots((cur) => [...cur, emptySlot()])
  }

  const removeSlot = (index: number) => {
    if (slots.length <= 1) return
    setSlots((cur) => cur.filter((_, i) => i !== index))
  }

  const updateSlot = (index: number, patch: Partial<SlotDraft>) => {
    setSlots((cur) => cur.map((s, i) => (i === index ? { ...s, ...patch } : s)))
  }

  const toggleGroup = (slotIdx: number, gid: number) => {
    setSlots((cur) =>
      cur.map((s, i) => {
        if (i !== slotIdx) return s
        const has = s.groupIds.includes(gid)
        return {
          ...s,
          groupIds: has ? s.groupIds.filter((x) => x !== gid) : [...s.groupIds, gid],
        }
      }),
    )
  }

  /** Validacion per-slot; devuelve null si OK o string con mensaje. */
  const slotError = (s: SlotDraft): string | null => {
    if (!s.text.trim()) return 'texto no puede estar vacio'
    const photoErr = validatePhotoUrlClient(s.photoUrl.trim())
    if (photoErr) return photoErr
    const btnErr = validateButtonsClient(nonEmptyRows(s.buttons))
    if (btnErr) return btnErr
    const groupErr = validateGroupIdsClient(s.groupIds)
    if (groupErr) return groupErr
    if (s.scheduledDate) {
      const r = validateScheduledAtClient(dateToLocalInput(s.scheduledDate), new Date())
      if (!r.ok) return r.error
    }
    return null
  }

  const errors = useMemo(() => slots.map(slotError), [slots])
  const hasErrors = errors.some((e) => e !== null)
  const submitting = create.isPending

  const handleSubmit = () => {
    if (hasErrors) return
    const payload = {
      publications: slots.map(slotToInput),
    }
    setAttempt((n) => n + 1)
    create.mutate(payload, {
      onSuccess: (resp) => {
        setResult(resp)
      },
    })
  }

  /** Reintentar fallidas: filtra los slots a los indices en failed[]. */
  const handleRetryFailed = () => {
    if (!result) return
    const failedIndexes = new Set(result.failed.map((f) => f.index))
    const remaining = slots.filter((_, i) => failedIndexes.has(i))
    setSlots(remaining.length > 0 ? remaining : [emptySlot()])
    setResult(null)
  }

  const handleClose = () => {
    onClose()
  }

  const groupName = (gid: number): string =>
    groups.data?.find((g) => g.telegram_id === gid)?.title ?? String(gid)

  const createdCount = result?.created.length ?? 0
  const failedCount = result?.failed.length ?? 0
  const scheduledCount = result?.created.filter((c) => c.publication.status === 'scheduled').length ?? 0
  const sentCount = result?.created.filter((c) => c.publication.status === 'sent').length ?? 0

  return (
    <Modal
      opened={opened}
      onClose={handleClose}
      title={<Text fw={600}>Publicar en lote</Text>}
      size="xl"
      centered
      data-testid="batch-wizard"
    >
      <Stack gap="md">
        {result ? (
          <ResultStep
            result={result}
            groupName={groupName}
            onRetry={handleRetryFailed}
            onClose={handleClose}
            attempt={attempt}
          />
        ) : (
          <>
            <Group justify="space-between" align="center">
              <Text size="sm" c="dimmed">
                Publicaciones: {slots.length} de {MAX_BATCH_SIZE} máx
              </Text>
              <Button
                type="button"
                size="xs"
                variant="default"
                leftSection={<IconPlus size={14} />}
                onClick={addSlot}
                disabled={slots.length >= MAX_BATCH_SIZE}
                data-testid="batch-add-slot"
              >
                + Agregar slot
              </Button>
            </Group>

            <Stack gap="md" data-testid="batch-slots">
              {slots.map((s, i) => (
                <Paper key={i} withBorder p="md" radius="md" data-testid={`batch-slot-${i}`}>
                  <Stack gap="sm">
                    <Group justify="space-between">
                      <Text fw={600} size="sm">Slot {i + 1}</Text>
                      {slots.length > 1 ? (
                        <Button
                          type="button"
                          size="xs"
                          color="red"
                          variant="light"
                          leftSection={<IconTrash size={14} />}
                          onClick={() => removeSlot(i)}
                          aria-label={`Remover slot ${i + 1}`}
                          data-testid={`batch-remove-slot-${i}`}
                        >
                          Remover
                        </Button>
                      ) : null}
                    </Group>

                    <Textarea
                      label={`Texto`}
                      value={s.text}
                      onChange={(e) => updateSlot(i, { text: e.currentTarget.value })}
                      placeholder="Escribí el mensaje…"
                      minRows={3}
                      autosize
                      maxRows={10}
                      error={errors[i] && errors[i]?.includes('texto') ? errors[i] : undefined}
                    />

                    <TextInput
                      type="url"
                      label="Foto (URL pública, opcional)"
                      value={s.photoUrl}
                      onChange={(e) => updateSlot(i, { photoUrl: e.currentTarget.value })}
                      placeholder="https://ejemplo.com/imagen.jpg"
                      error={errors[i] && errors[i]?.includes('URL') ? errors[i] : undefined}
                    />

                    <Stack gap="xs">
                      <Text fw={500} size="sm">
                        Grupos ({s.groupIds.length}/10)
                      </Text>
                      {groups.data?.length === 0 ? (
                        <Group gap="xs">
                          <Loader size="xs" />
                          <Text size="sm" c="dimmed">Cargando grupos…</Text>
                        </Group>
                      ) : (
                        <Stack gap={6}>
                          {groups.data?.map((g) => (
                            <Checkbox
                              key={g.telegram_id}
                              label={g.title}
                              checked={s.groupIds.includes(g.telegram_id)}
                              onChange={() => toggleGroup(i, g.telegram_id)}
                              disabled={
                                !s.groupIds.includes(g.telegram_id) && s.groupIds.length >= 10
                              }
                            />
                          ))}
                        </Stack>
                      )}
                      {errors[i] && errors[i]?.includes('grupo') ? (
                        <Text size="xs" c="red">
                          {errors[i]}
                        </Text>
                      ) : null}
                    </Stack>

                    <DateTimePicker
                      label="Programar para (opcional)"
                      value={s.scheduledDate}
                      onChange={(d) => updateSlot(i, { scheduledDate: d })}
                      leftSection={<IconCalendar size={16} />}
                      valueFormat="DD/MM/YYYY HH:mm"
                      clearable
                      error={errors[i] && errors[i]?.includes('fecha') ? errors[i] : undefined}
                    />

                    <Stack gap="xs">
                      <Text fw={500} size="sm">
                        Botones inline (opcional)
                      </Text>
                      <ButtonsEditor value={s.buttons} onChange={(rows) => updateSlot(i, { buttons: rows })} />
                    </Stack>

                    {errors[i] && !['texto', 'URL', 'grupo', 'fecha'].some((k) => errors[i]?.includes(k)) ? (
                      <Text size="xs" c="red">
                        {errors[i]}
                      </Text>
                    ) : null}
                  </Stack>
                </Paper>
              ))}
            </Stack>

            <SummaryAlert
              slots={slots}
              errors={errors}
              groupName={groupName}
              createdCount={createdCount}
              failedCount={failedCount}
              sentCount={sentCount}
              scheduledCount={scheduledCount}
            />

            {create.error ? (
              <Alert color="red" variant="light" data-testid="batch-create-error">
                {formatPublicationsError(create.error)}
              </Alert>
            ) : null}

            <Group justify="space-between">
              <Button variant="default" onClick={handleClose} leftSection={<IconX size={16} />}>
                Cancelar
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={hasErrors || submitting}
                loading={submitting}
                leftSection={<IconSend size={16} />}
                data-testid="batch-submit"
              >
                {submitting
                  ? 'Enviando…'
                  : slots.length === 1
                    ? 'Enviar 1 publicación'
                    : `Enviar ${slots.length} publicaciones`}
              </Button>
            </Group>
          </>
        )}
      </Stack>
    </Modal>
  )
}

interface SummaryAlertProps {
  slots: SlotDraft[]
  errors: (string | null)[]
  groupName: (gid: number) => string
  createdCount: number
  failedCount: number
  sentCount: number
  scheduledCount: number
}

function SummaryAlert({ slots, errors, groupName }: SummaryAlertProps) {
  const hasErrors = errors.some((e) => e !== null)
  return (
    <Alert
      color={hasErrors ? 'red' : 'blue'}
      variant="light"
      title={hasErrors ? 'Revisá los slots antes de enviar' : 'Resumen'}
      data-testid="batch-summary"
    >
      <Stack gap={4}>
        {slots.map((s, i) => {
          const err = errors[i]
          return (
            <Group key={i} gap="xs" wrap="nowrap">
              <Badge size="sm" color={err ? 'red' : 'gray'} variant="light">
                {i + 1}
              </Badge>
              <Text size="xs" c={err ? 'red' : undefined}>
                {err ? `Slot ${i + 1}: ${err}` : truncate(s.text || '(sin texto)')}
                {s.groupIds.length > 0 ? ` → ${s.groupIds.map(groupName).join(', ')}` : ''}
                {formatScheduledPreview(s.scheduledDate)
                  ? ` · programado ${formatScheduledPreview(s.scheduledDate)}`
                  : ''}
              </Text>
            </Group>
          )
        })}
      </Stack>
    </Alert>
  )
}

interface ResultStepProps {
  result: BatchResponse
  groupName: (gid: number) => string
  onRetry: () => void
  onClose: () => void
  attempt: number
}

function ResultStep({ result, groupName, onRetry, onClose, attempt }: ResultStepProps) {
  const createdCount = result.created.length
  const failedCount = result.failed.length

  return (
    <Stack gap="md" data-testid="batch-result">
      <Title order={4}>Resultado del lote (intento #{attempt})</Title>

      {createdCount > 0 ? (
        <Alert color="green" variant="light" data-testid="batch-success-alert">
          <Stack gap={4}>
            <Text fw={600} size="sm">
              {createdCount === 1
                ? '1 publicación creada correctamente'
                : `${createdCount} publicaciones creadas correctamente`}
            </Text>
            {result.created.map((c) => (
              <Group key={`${c.index}-${c.publication.id}`} gap="xs">
                <Badge size="xs" variant="light" color="green">
                  Slot {c.index + 1}
                </Badge>
                <Text size="xs" c="dimmed">
                  → {groupName(c.publication.telegram_id)} ·{' '}
                  <strong>{c.publication.status}</strong>
                </Text>
              </Group>
            ))}
            <Text size="xs" c="dimmed" mt={4}>
              Las publicaciones que ya se enviaron o programaron siguen vigentes — Reintentar
              solo republica los fallidos.
            </Text>
          </Stack>
        </Alert>
      ) : null}

      {failedCount > 0 ? (
        <Alert color="red" variant="light" data-testid="batch-failure-alert">
          <Stack gap={4}>
            <Text fw={600} size="sm">
              {failedCount === 1
                ? '1 publicación falló'
                : `${failedCount} publicaciones fallaron`}
            </Text>
            {result.failed.map((f) => (
              <FailedRow key={f.index} failure={f} />
            ))}
          </Stack>
        </Alert>
      ) : null}

      {createdCount === 0 && failedCount === 0 ? (
        <Alert color="yellow" variant="light">
          El batch no produjo resultados.
        </Alert>
      ) : null}

      <Group justify="space-between">
        <Button variant="default" onClick={onClose} data-testid="batch-close">
          Cerrar
        </Button>
        {failedCount > 0 ? (
          <Button color="orange" onClick={onRetry} data-testid="batch-retry">
            Reintentar fallidas
          </Button>
        ) : null}
      </Group>
    </Stack>
  )
}

function FailedRow({ failure }: { failure: BatchFailedItem }) {
  return (
    <Group gap="xs" wrap="nowrap">
      <Badge size="xs" variant="light" color="red">
        Slot {failure.index + 1}
      </Badge>
      <Text size="xs">
        <strong>{failure.code}</strong>: {failure.message || 'sin detalle'}
      </Text>
    </Group>
  )
}

// Helper exportado para tests: default slot count. No se usa fuera del
// wizard pero permite verificar la constante sin importar el componente.
export const __test = { DEFAULT_SLOT_COUNT, MAX_BATCH_SIZE }

// Re-exports de tipos del publication para tests que quieran tipar
// helpers externos (ej. mocks del wizard).
export type { Publication }
