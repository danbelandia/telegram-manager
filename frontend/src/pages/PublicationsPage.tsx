// Publicaciones (Fase 2, slice 3 — programacion + cancel + paginacion).
// Migrado a Mantine v7 (frontend-refresh slice 2 ad-hoc para esta
// pagina): la pagina completa usa Stack/Group/Card/Textarea/TextInput/
// Radio/Checkbox/Table/Badge/Alert. La logica de negocio no cambia
// (validation cliente + hooks existentes en features/publications/).
//
// Slice photos-videos-upload: reemplaza TextInput de foto URL por
// MediaUploader (Dropzone) que soporta fotos JPG/PNG/GIF/WebP y
// videos MP4 subidos desde el PC.
import { useMemo, useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Checkbox,
  Group,
  Loader,
  NativeSelect,
  Paper,
  Radio,
  Stack,
  Table,
  Text,
  Textarea,
  TextInput,
  Title,
} from '@mantine/core'
import { DateTimePicker } from '@mantine/dates'
import { IconCalendar, IconSend, IconStack3, IconTrash } from '@tabler/icons-react'
import {
  formatPublicationsError,
  validateButtonsClient,
  validateGroupIdsClient,
  validatePhotoUrlClient,
} from '../features/publications/error'
import {
  useCancelPublication,
  useCreatePublication,
  usePublications,
} from '../features/publications/hooks'
import {
  validateScheduledAtClient,
} from '../features/publications/validateScheduledAtClient'
import { dateToLocalInput } from '../features/publications/dateHelpers'
import type {
  InlineButton,
  Publication,
  PublicationStatus,
} from '../features/publications/types'
import { useGroups } from '../features/groups/hooks'
import ButtonsEditor from '../features/publications/ButtonsEditor'
import BatchWizard from '../features/publications-batch/BatchWizard'
import MediaUploader from '../components/MediaUploader'
import type { MediaValue } from '../components/MediaUploader'

const STATUS_LABEL: Record<PublicationStatus, string> = {
  draft: 'Borrador',
  scheduled: 'Programada',
  sending: 'Enviando',
  sent: 'Enviada',
  failed: 'Fallida',
}

const STATUS_COLOR: Record<PublicationStatus, string> = {
  draft: 'gray',
  scheduled: 'blue',
  sending: 'yellow',
  sent: 'green',
  failed: 'red',
}

const MAX_TEXT_NO_PHOTO = 4096
const MAX_TEXT_WITH_PHOTO = 1024
const MAX_GROUPS = 10
const DEFAULT_LIST_LIMIT = 10

type PublishMode = 'now' | 'schedule'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

function truncate(text: string, max = 80): string {
  return text.length > max ? `${text.slice(0, max)}…` : text
}

function nonEmptyRows(rows: InlineButton[][]): InlineButton[][] {
  // Filtra filas completamente vacias (sin botones o con todos los
  // botones vacios) para no enviar payload inutil al backend.
  return rows
    .map((row) => row.filter((btn) => btn.text.trim() || btn.url.trim()))
    .filter((row) => row.length > 0)
}

export default function PublicationsPage() {
  const groups = useGroups()
  const [filterGid, setFilterGid] = useState<number | 'all'>('all')
  const [offset, setOffset] = useState(0)

  const publications = usePublications({
    group_id: filterGid === 'all' ? undefined : filterGid,
    limit: DEFAULT_LIST_LIMIT,
    offset,
  })
  const create = useCreatePublication()
  const cancel = useCancelPublication()

  const [mode, setMode] = useState<PublishMode>('now')
  const [text, setText] = useState('')
  const [photoUrl, setPhotoUrl] = useState('')
  const [mediaFile, setMediaFile] = useState<MediaValue | null>(null)
  const [buttons, setButtons] = useState<InlineButton[][]>([])
  const [selectedGroupIds, setSelectedGroupIds] = useState<number[]>([])
  const [scheduledDate, setScheduledDate] = useState<Date | null>(null)
  const [clientError, setClientError] = useState<string | null>(null)
  const [batchOpen, setBatchOpen] = useState(false)

  // Media: archivo subido (Dropzone) tiene prioridad sobre URL manual
  const hasMedia = mediaFile !== null
  const hasPhotoUrl = !hasMedia && photoUrl.trim().length > 0
  const hasAnyMedia = hasMedia || hasPhotoUrl
  // Con foto/video, el caption baja a 1024 (limite de sendPhoto/sendVideo)
  const textMax = hasAnyMedia ? MAX_TEXT_WITH_PHOTO : MAX_TEXT_NO_PHOTO
  const textOver = text.length > textMax

  const createError = create.error ? formatPublicationsError(create.error) : null
  const cancelError = cancel.error ? formatPublicationsError(cancel.error) : null
  const created = create.isSuccess
    ? mode === 'schedule'
      ? 'Publicación programada.'
      : 'Publicación enviada.'
    : null

  const toggleGroup = (gid: number) => {
    setSelectedGroupIds((cur) =>
      cur.includes(gid) ? cur.filter((x) => x !== gid) : [...cur, gid],
    )
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setClientError(null)

    const trimmedText = text.trim()
    if (!trimmedText) {
      setClientError('Escribí un mensaje antes de publicar.')
      return
    }
    if (textOver) {
      setClientError(
        hasAnyMedia
          ? 'El texto excede 1024 caracteres cuando se envía con media.'
          : `El texto excede ${MAX_TEXT_NO_PHOTO} caracteres.`,
      )
      return
    }

    // Validar URL solo si no hay archivo subido y se pegó una URL
    if (!hasMedia && photoUrl.trim().length > 0) {
      const photoErr = validatePhotoUrlClient(photoUrl.trim())
      if (photoErr) {
        setClientError(photoErr)
        return
      }
    }

    const rows = nonEmptyRows(buttons)
    const btnErr = validateButtonsClient(rows)
    if (btnErr) {
      setClientError(btnErr)
      return
    }

    const groupErr = validateGroupIdsClient(selectedGroupIds)
    if (groupErr) {
      setClientError(groupErr)
      return
    }

    let scheduledAt: string | undefined
    if (mode === 'schedule') {
      const res = validateScheduledAtClient(scheduledDate ? dateToLocalInput(scheduledDate) : '', new Date())
      if (!res.ok) {
        setClientError(res.error)
        return
      }
      scheduledAt = res.iso
    }

    // Construir payload: archivo subido o URL manual
    let photo_url: string | undefined
    let video_url: string | undefined
    if (hasMedia) {
      if (mediaFile!.type === 'photo') {
        photo_url = mediaFile!.url
      } else {
        video_url = mediaFile!.url
      }
    } else if (hasPhotoUrl) {
      photo_url = photoUrl.trim()
    }

    create.mutate({
      text: trimmedText,
      photo_url,
      video_url,
      buttons: rows.length > 0 ? rows : undefined,
      group_ids: selectedGroupIds,
      scheduled_at: scheduledAt,
    })
  }

  const handleCancel = (id: number) => {
    const ok = window.confirm('¿Cancelar esta publicación programada?')
    if (!ok) return
    cancel.mutate(id)
  }

  const groupName = (telegramId: number): string =>
    groups.data?.find((g) => g.telegram_id === telegramId)?.title ?? String(telegramId)

  const selectedCount = selectedGroupIds.length

  // Paginacion: Next deshabilitado cuando returned < limit.
  const canPrev = offset > 0
  const canNext = (publications.data?.length ?? 0) >= DEFAULT_LIST_LIMIT

  return (
    <Stack gap="lg" p="md">
      <Title order={1}>Publicaciones</Title>

      <Stack gap="sm">
        {created ? (
          <Alert color="green" variant="light">
            {created}
          </Alert>
        ) : null}
        {createError ? (
          <Alert color="red" variant="light" data-testid="create-error">
            {createError}
          </Alert>
        ) : null}
        {cancelError ? (
          <Alert color="red" variant="light" data-testid="cancel-error">
            {cancelError}
          </Alert>
        ) : null}
        {clientError ? (
          <Alert color="red" variant="light" data-testid="client-error">
            {clientError}
          </Alert>
        ) : null}
      </Stack>

      <Group justify="flex-end">
        <Button
          variant="default"
          leftSection={<IconStack3 size={16} />}
          onClick={() => setBatchOpen(true)}
          data-testid="open-batch-wizard"
        >
          Programar en lote
        </Button>
      </Group>

      <Paper withBorder p="lg" radius="md">
        <form onSubmit={handleSubmit}>
          <Stack gap="lg">
            <Title order={3}>Nueva publicación</Title>

            <Radio.Group
              label="Modo"
              value={mode}
              onChange={(v) => setMode(v as PublishMode)}
            >
              <Group mt="xs" gap="lg">
                <Radio value="now" label="Publicar ahora" />
                <Radio value="schedule" label="Programar" />
              </Group>
            </Radio.Group>

            <Textarea
              label={`Texto (${text.length}/${textMax})`}
              value={text}
              onChange={(e) => setText(e.currentTarget.value)}
              placeholder="Escribí el mensaje a publicar…"
              minRows={5}
              autosize
              maxRows={20}
              maxLength={textMax}
              radius="lg"
              error={textOver ? `Excede el límite (${textMax}).${hasAnyMedia ? ' Con media el máximo es 1024 (caption).' : ''}` : undefined}
              styles={{ input: { fontSize: '0.95rem' } }}
            />

            <Stack gap="xs">
              <Text fw={500} size="sm">
                Media (opcional)
              </Text>
              <MediaUploader
                value={mediaFile}
                onChange={setMediaFile}
                onError={(msg) => setClientError(msg)}
              />
              {!hasMedia && (
                <TextInput
                  type="url"
                  label="O pegá una URL de imagen"
                  value={photoUrl}
                  onChange={(e) => setPhotoUrl(e.currentTarget.value)}
                  placeholder="https://ejemplo.com/imagen.jpg"
                  radius="md"
                  size="sm"
                />
              )}
            </Stack>

            <Stack gap="xs">
              <Text fw={500} size="sm">
                Grupos ({selectedCount}/{MAX_GROUPS})
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
                      checked={selectedGroupIds.includes(g.telegram_id)}
                      onChange={() => toggleGroup(g.telegram_id)}
                      disabled={
                        !selectedGroupIds.includes(g.telegram_id) && selectedCount >= MAX_GROUPS
                      }
                    />
                  ))}
                </Stack>
              )}
            </Stack>

            {mode === 'schedule' ? (
              <DateTimePicker
                label="Fecha y hora (zona horaria local del navegador)"
                value={scheduledDate}
                onChange={setScheduledDate}
                leftSection={<IconCalendar size={16} />}
                radius="md"
                valueFormat="DD/MM/YYYY HH:mm"
                locale="es"
                dropdownType="popover"
                size="md"
                clearable
              />
            ) : null}

            <Stack gap="xs">
              <Text fw={500} size="sm">
                Botones inline (URL, opcional)
              </Text>
              <ButtonsEditor value={buttons} onChange={setButtons} />
            </Stack>

            <Group justify="flex-end">
              <Button
                type="submit"
                leftSection={<IconSend size={16} />}
                disabled={create.isPending || !text.trim() || selectedCount === 0 || textOver}
                loading={create.isPending}
              >
                {create.isPending
                  ? mode === 'schedule'
                    ? 'Programando…'
                    : 'Publicando…'
                  : mode === 'schedule'
                    ? 'Programar'
                    : 'Publicar ahora'}
              </Button>
            </Group>
          </Stack>
        </form>
      </Paper>

      <Paper withBorder p="lg" radius="md">
        <Stack gap="md">
          <Title order={3}>Historial</Title>

          <NativeSelect
            label="Filtrar por grupo"
            name="filter-group"
            value={filterGid === 'all' ? 'all' : String(filterGid)}
            onChange={(e) => {
              const v = e.currentTarget.value
              setFilterGid(v === 'all' ? 'all' : Number(v))
              setOffset(0) // reset paginacion al cambiar filtro
            }}
            data={[
              { value: 'all', label: 'Todos' },
              ...(groups.data?.map((g) => ({
                value: String(g.telegram_id),
                label: g.title,
              })) ?? []),
            ]}
            radius="md"
          />

          {publications.isPending ? (
            <Group gap="xs">
              <Loader size="sm" />
              <Text size="sm" c="dimmed">Cargando publicaciones…</Text>
            </Group>
          ) : null}

          {publications.isError ? (
            <Alert color="red" variant="light">
              <Stack gap="xs">
                <Text size="sm">
                  {publications.error instanceof Error
                    ? publications.error.message
                    : 'No se pudieron cargar las publicaciones.'}
                </Text>
                <Group>
                  <Button variant="default" size="xs" onClick={() => publications.refetch()}>
                    Reintentar
                  </Button>
                </Group>
              </Stack>
            </Alert>
          ) : null}

          {publications.data && publications.data.length === 0 ? (
            <Text size="sm" c="dimmed">No hay publicaciones.</Text>
          ) : null}

          {publications.data && publications.data.length > 0 ? (
            <Table.ScrollContainer minWidth={900}>
              <Table striped highlightOnHover withTableBorder withColumnBorders verticalSpacing="sm">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Mensaje</Table.Th>
                    <Table.Th>Media</Table.Th>
                    <Table.Th>Botones</Table.Th>
                    <Table.Th>Grupo</Table.Th>
                    <Table.Th>Estado</Table.Th>
                    <Table.Th>Fecha</Table.Th>
                    <Table.Th>Acción</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {publications.data.map((pub) => (
                    <PublicationRow
                      key={pub.id}
                      pub={pub}
                      groupName={groupName}
                      onCancel={handleCancel}
                      isCanceling={cancel.isPending && cancel.variables === pub.id}
                    />
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          ) : null}

          <Group justify="space-between">
            <Button
              variant="default"
              disabled={!canPrev}
              onClick={() => setOffset(Math.max(0, offset - DEFAULT_LIST_LIMIT))}
            >
              ← Anterior
            </Button>
            <Text size="sm" c="dimmed">Página {Math.floor(offset / DEFAULT_LIST_LIMIT) + 1}</Text>
            <Button
              variant="default"
              disabled={!canNext}
              onClick={() => setOffset(offset + DEFAULT_LIST_LIMIT)}
            >
              Siguiente →
            </Button>
          </Group>
        </Stack>
      </Paper>

      <BatchWizard opened={batchOpen} onClose={() => setBatchOpen(false)} />
    </Stack>
  )
}

function PublicationRow({
  pub,
  groupName,
  onCancel,
  isCanceling,
}: {
  pub: Publication
  groupName: (id: number) => string
  onCancel: (id: number) => void
  isCanceling: boolean
}) {
  const buttonsPreview = useMemo(() => {
    if (!pub.buttons || pub.buttons.length === 0) return null
    return pub.buttons
  }, [pub.buttons])
  const canCancel = pub.status === 'scheduled'
  return (
    <Table.Tr>
      <Table.Td title={pub.text}>{truncate(pub.text)}</Table.Td>
      <Table.Td>
        {pub.photo_url ? (
          <img
            src={pub.photo_url}
            alt="foto publicación"
            style={{ maxWidth: 60, maxHeight: 40, borderRadius: 4 }}
          />
        ) : pub.video_url ? (
          <Badge color="blue" variant="light" size="sm">
            Video
          </Badge>
        ) : (
          '—'
        )}
      </Table.Td>
      <Table.Td>
        {buttonsPreview && buttonsPreview.length > 0 ? (
          <Stack gap={4}>
            {buttonsPreview.map((row, i) => (
              <Group gap={4} key={i}>
                {row.map((btn, j) => (
                  <Badge key={j} variant="light" title={btn.url}>
                    {btn.text || '(sin texto)'}
                  </Badge>
                ))}
              </Group>
            ))}
          </Stack>
        ) : (
          '—'
        )}
      </Table.Td>
      <Table.Td>{groupName(pub.telegram_id)}</Table.Td>
      <Table.Td>
        <Badge color={STATUS_COLOR[pub.status]} variant="light">
          {STATUS_LABEL[pub.status]}
        </Badge>
        {pub.status === 'scheduled' && pub.scheduled_at ? (
          <Text size="xs" c="dimmed" mt={4}>
            para {formatDate(pub.scheduled_at)}
          </Text>
        ) : null}
      </Table.Td>
      <Table.Td>{formatDate(pub.created_at)}</Table.Td>
      <Table.Td>
        {canCancel ? (
          <Button
            size="xs"
            color="red"
            variant="light"
            leftSection={<IconTrash size={14} />}
            onClick={() => onCancel(pub.id)}
            disabled={isCanceling}
          >
            {isCanceling ? 'Cancelando…' : 'Cancelar'}
          </Button>
        ) : (
          '—'
        )}
      </Table.Td>
    </Table.Tr>
  )
}
