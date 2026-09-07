// Publicaciones (Fase 2, slice 2 — foto URL + botones inline + multi-grupo
// + filtro). La pagina /publications lista publicaciones y permite crear
// una nueva con contenido enriquecido y envio a varios grupos.
// Validacion cliente antes de mutate() (text/url/grupos/botones).
import { useMemo, useState } from 'react'
import {
  formatPublicationsError,
  validateButtonsClient,
  validateGroupIdsClient,
  validatePhotoUrlClient,
} from '../features/publications/error'
import { useCreatePublication, usePublications } from '../features/publications/hooks'
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
  const publications = usePublications(filterGid === 'all' ? undefined : { group_id: filterGid })
  const create = useCreatePublication()

  const [text, setText] = useState('')
  const [photoUrl, setPhotoUrl] = useState('')
  const [buttons, setButtons] = useState<InlineButton[][]>([])
  const [selectedGroupIds, setSelectedGroupIds] = useState<number[]>([])
  const [clientError, setClientError] = useState<string | null>(null)

  const hasPhoto = photoUrl.trim().length > 0
  const textMax = hasPhoto ? MAX_TEXT_WITH_PHOTO : MAX_TEXT_NO_PHOTO
  const textOver = text.length > textMax

  const createError = create.error ? formatPublicationsError(create.error) : null
  const created = create.isSuccess ? 'Publicación enviada.' : null

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

    create.mutate({
      text: trimmedText,
      photo_url: hasPhoto ? photoUrl.trim() : undefined,
      buttons: rows.length > 0 ? rows : undefined,
      group_ids: selectedGroupIds,
    })
  }

  const groupName = (telegramId: number): string =>
    groups.data?.find((g) => g.telegram_id === telegramId)?.title ?? String(telegramId)

  const selectedCount = selectedGroupIds.length

  return (
    <main className="page">
      <h1>Publicaciones</h1>

      {created ? <p className="state-block state-ok">{created}</p> : null}
      {createError ? <p className="state-block state-error">{createError}</p> : null}
      {clientError ? <p className="state-block state-error">{clientError}</p> : null}

      <section className="panel">
        <h2>Nueva publicación</h2>
        <form onSubmit={handleSubmit}>
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

          <div className="field">
            <span>Botones inline (URL, opcional)</span>
            <ButtonsEditor value={buttons} onChange={setButtons} />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            disabled={create.isPending || !text.trim() || selectedCount === 0 || textOver}
          >
            {create.isPending ? 'Publicando…' : 'Publicar ahora'}
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
                </tr>
              </thead>
              <tbody>
                {publications.data.map((pub) => (
                  <PublicationRow key={pub.id} pub={pub} groupName={groupName} />
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </main>
  )
}

function PublicationRow({
  pub,
  groupName,
}: {
  pub: Publication
  groupName: (id: number) => string
}) {
  const buttonsPreview = useMemo(() => {
    if (!pub.buttons || pub.buttons.length === 0) return null
    return pub.buttons
  }, [pub.buttons])
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
      </td>
      <td>{formatDate(pub.created_at)}</td>
    </tr>
  )
}
