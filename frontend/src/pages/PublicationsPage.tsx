// Publicaciones (Fase 2, slice 3 — programacion + cancel + paginacion).
// La pagina /publications lista publicaciones paginadas y permite:
//   - "Publicar ahora": flujo inmediato slice 2 (multi-grupo + foto + botones).
//   - "Programar": envia `scheduled_at` (RFC3339 con offset); el worker
//     in-process las procesa.
//   - Cancelar una fila `schededuled` (DELETE /api/publications/:id).
//   - Paginar el historial (Prev / Next).
//
// Validacion cliente antes de mutate() (text/url/grupos/botones/scheduled_at).
import { useMemo, useState } from 'react'
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
import type {
  InlineButton,
  Publication,
  PublicationStatus,
} from '../features/publications/types'
import { useGroups } from '../features/groups/hooks'
import ButtonsEditor from '../features/publications/ButtonsEditor'

const STATUS_LABEL: Record<PublicationStatus, string> = {
  draft: 'Borrador',
  scheduled: 'Programada',
  sending: 'Enviando',
  sent: 'Enviada',
  failed: 'Fallida',
}

const MAX_TEXT_NO_PHOTO = 4096
const MAX_TEXT_WITH_PHOTO = 1024
const MAX_GROUPS = 10
const DEFAULT_LIST_LIMIT = 50

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
  const [buttons, setButtons] = useState<InlineButton[][]>([])
  const [selectedGroupIds, setSelectedGroupIds] = useState<number[]>([])
  const [scheduledLocal, setScheduledLocal] = useState('')
  const [clientError, setClientError] = useState<string | null>(null)

  const hasPhoto = photoUrl.trim().length > 0
  const textMax = hasPhoto ? MAX_TEXT_WITH_PHOTO : MAX_TEXT_NO_PHOTO
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
        hasPhoto
          ? 'El texto excede 1024 caracteres cuando se envía con foto.'
          : `El texto excede ${MAX_TEXT_NO_PHOTO} caracteres.`,
      )
      return
    }

    const photoErr = validatePhotoUrlClient(photoUrl.trim())
    if (photoErr) {
      setClientError(photoErr)
      return
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
      const res = validateScheduledAtClient(scheduledLocal, new Date())
      if (!res.ok) {
        setClientError(res.error)
        return
      }
      scheduledAt = res.iso
    }

    create.mutate({
      text: trimmedText,
      photo_url: hasPhoto ? photoUrl.trim() : undefined,
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
    <main className="page">
      <h1>Publicaciones</h1>

      {created ? <p className="state-block state-ok">{created}</p> : null}
      {createError ? <p className="state-block state-error">{createError}</p> : null}
      {cancelError ? <p className="state-block state-error">{cancelError}</p> : null}
      {clientError ? <p className="state-block state-error">{clientError}</p> : null}

      <section className="panel">
        <h2>Nueva publicación</h2>
        <form onSubmit={handleSubmit}>
          <fieldset className="field">
            <legend>Modo</legend>
            <label className="radio">
              <input
                type="radio"
                name="mode"
                value="now"
                checked={mode === 'now'}
                onChange={() => setMode('now')}
              />
              Publicar ahora
            </label>
            <label className="radio">
              <input
                type="radio"
                name="mode"
                value="schedule"
                checked={mode === 'schedule'}
                onChange={() => setMode('schedule')}
              />
              Programar
            </label>
          </fieldset>

          <label className="field">
            Texto ({text.length}/{textMax})
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={5}
              maxLength={textMax}
              placeholder="Escribí el mensaje a publicar…"
            />
            {textOver ? (
              <span className="form-error">
                Excede el límite ({textMax}).{hasPhoto ? ' Con foto el máximo es 1024 (caption).' : ''}
              </span>
            ) : null}
          </label>

          <label className="field">
            Foto (URL pública, opcional, ≤ 5 MB)
            <input
              type="url"
              value={photoUrl}
              onChange={(e) => setPhotoUrl(e.target.value)}
              placeholder="https://ejemplo.com/imagen.jpg"
            />
          </label>

          <fieldset className="field">
            <legend>Grupos ({selectedCount}/{MAX_GROUPS})</legend>
            {groups.data?.length === 0 ? (
              <p className="state-block-weak">Cargando grupos…</p>
            ) : (
              <div className="group-checkboxes">
                {groups.data?.map((g) => (
                  <label key={g.telegram_id} className="checkbox">
                    <input
                      type="checkbox"
                      checked={selectedGroupIds.includes(g.telegram_id)}
                      onChange={() => toggleGroup(g.telegram_id)}
                      disabled={
                        !selectedGroupIds.includes(g.telegram_id) && selectedCount >= MAX_GROUPS
                      }
                    />
                    {g.title}
                  </label>
                ))}
              </div>
            )}
          </fieldset>

          {mode === 'schedule' ? (
            <label className="field">
              Fecha y hora (zona horaria local del navegador)
              <input
                type="datetime-local"
                value={scheduledLocal}
                onChange={(e) => setScheduledLocal(e.target.value)}
              />
            </label>
          ) : null}

          <div className="field">
            <span>Botones inline (URL, opcional)</span>
            <ButtonsEditor value={buttons} onChange={setButtons} />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={create.isPending || !text.trim() || selectedCount === 0 || textOver}
          >
            {create.isPending
              ? mode === 'schedule'
                ? 'Programando…'
                : 'Publicando…'
              : mode === 'schedule'
                ? 'Programar'
                : 'Publicar ahora'}
          </button>
        </form>
      </section>

      <section>
        <h2>Historial</h2>

        <label className="field">
          Filtrar por grupo
          <select
            value={filterGid === 'all' ? 'all' : String(filterGid)}
            onChange={(e) => {
              const v = e.target.value
              setFilterGid(v === 'all' ? 'all' : Number(v))
              setOffset(0) // reset paginacion al cambiar filtro
            }}
          >
            <option value="all">Todos</option>
            {groups.data?.map((g) => (
              <option key={g.telegram_id} value={String(g.telegram_id)}>
                {g.title}
              </option>
            ))}
          </select>
        </label>

        {publications.isPending ? <p className="state-block">Cargando publicaciones…</p> : null}

        {publications.isError ? (
          <div className="state-block state-error">
            <p>
              {publications.error instanceof Error
                ? publications.error.message
                : 'No se pudieron cargar las publicaciones.'}
            </p>
            <button type="button" className="btn" onClick={() => publications.refetch()}>
              Reintentar
            </button>
          </div>
        ) : null}

        {publications.data && publications.data.length === 0 ? (
          <p className="state-block">No hay publicaciones.</p>
        ) : null}

        {publications.data && publications.data.length > 0 ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Mensaje</th>
                  <th>Foto</th>
                  <th>Botones</th>
                  <th>Grupo</th>
                  <th>Estado</th>
                  <th>Fecha</th>
                  <th>Acción</th>
                </tr>
              </thead>
              <tbody>
                {publications.data.map((pub) => (
                  <PublicationRow
                    key={pub.id}
                    pub={pub}
                    groupName={groupName}
                    onCancel={handleCancel}
                    isCanceling={cancel.isPending && cancel.variables === pub.id}
                  />
                ))}
              </tbody>
            </table>
          </div>
        ) : null}

        <nav className="pagination" aria-label="Paginación">
          <button
            type="button"
            className="btn"
            disabled={!canPrev}
            onClick={() => setOffset(Math.max(0, offset - DEFAULT_LIST_LIMIT))}
          >
            ← Anterior
          </button>
          <span className="pagination-info">Página offset={offset}</span>
          <button
            type="button"
            className="btn"
            disabled={!canNext}
            onClick={() => setOffset(offset + DEFAULT_LIST_LIMIT)}
          >
            Siguiente →
          </button>
        </nav>
      </section>
    </main>
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
    <tr>
      <td title={pub.text}>{truncate(pub.text)}</td>
      <td>
        {pub.photo_url ? (
          <img
            src={pub.photo_url}
            alt="foto publicación"
            style={{ maxWidth: 60, maxHeight: 40, borderRadius: 4 }}
          />
        ) : (
          '—'
        )}
      </td>
      <td>
        {buttonsPreview && buttonsPreview.length > 0 ? (
          <div className="button-chips">
            {buttonsPreview.map((row, i) => (
              <div key={i} className="button-chip-row">
                {row.map((btn, j) => (
                  <span key={j} className="button-chip" title={btn.url}>
                    {btn.text || '(sin texto)'}
                  </span>
                ))}
              </div>
            ))}
          </div>
        ) : (
          '—'
        )}
      </td>
      <td>{groupName(pub.telegram_id)}</td>
      <td>
        <span className={`badge badge-${pub.status}`}>{STATUS_LABEL[pub.status]}</span>
        {pub.status === 'scheduled' && pub.scheduled_at ? (
          <div className="state-block-weak">para {formatDate(pub.scheduled_at)}</div>
        ) : null}
      </td>
      <td>{formatDate(pub.created_at)}</td>
      <td>
        {canCancel ? (
          <button
            type="button"
            className="btn btn-danger"
            onClick={() => onCancel(pub.id)}
            disabled={isCanceling}
          >
            {isCanceling ? 'Cancelando…' : 'Cancelar'}
          </button>
        ) : (
          '—'
        )}
      </td>
    </tr>
  )
}