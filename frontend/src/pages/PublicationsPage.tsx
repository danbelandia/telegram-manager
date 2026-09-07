// Publicaciones (Fase 2, slice 1 — publicar texto ahora): la pagina
// /publications lista las publicaciones del bot y permite crear una
// nueva eligiendo grupo y escribiendo el texto. Estados loading/vacio/
// error y feedback con errores legibles (seccion 18).
import { useState } from 'react'
import { formatPublicationsError } from '../features/publications/error'
import { useCreatePublication, usePublications } from '../features/publications/hooks'
import type { PublicationStatus } from '../features/publications/types'
import { useGroups } from '../features/groups/hooks'

const STATUS_LABEL: Record<PublicationStatus, string> = {
  draft: 'Borrador',
  scheduled: 'Programada',
  sending: 'Enviando',
  sent: 'Enviada',
  failed: 'Fallida',
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

function truncate(text: string, max = 80): string {
  return text.length > max ? `${text.slice(0, max)}…` : text
}

export default function PublicationsPage() {
  const publications = usePublications()
  const groups = useGroups()
  const create = useCreatePublication()

  const [text, setText] = useState('')
  const [groupId, setGroupId] = useState('')

  // Los errores de mutate() son asincronos: llegan por mutation.error,
  // no por excepcion sincrona.
  const createError = create.error ? formatPublicationsError(create.error) : null
  const created = create.isSuccess ? 'Publicación enviada.' : null

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const num = Number(groupId)
    if (!text.trim() || !Number.isFinite(num)) {
      return
    }
    create.mutate({ text, group_id: num })
  }

  const groupName = (telegramId: number): string =>
    groups.data?.find((g) => g.telegram_id === telegramId)?.title ?? String(telegramId)

  return (
    <main className="page">
      <h1>Publicaciones</h1>

      {created ? <p className="state-block state-ok">{created}</p> : null}
      {createError ? <p className="state-block state-error">{createError}</p> : null}

      <section className="panel">
        <h2>Nueva publicación</h2>
        <form onSubmit={handleSubmit}>
          <label className="field">
            Grupo
            <select value={groupId} onChange={(e) => setGroupId(e.target.value)}>
              <option value="">Seleccioná un grupo…</option>
              {groups.data?.map((g) => (
                <option key={g.telegram_id} value={g.telegram_id}>
                  {g.title}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            Mensaje
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={5}
              maxLength={4096}
              placeholder="Escribí el mensaje a publicar…"
            />
          </label>
          <button type="submit" className="btn btn-primary" disabled={create.isPending || !text.trim() || !groupId}>
            {create.isPending ? 'Publicando…' : 'Publicar ahora'}
          </button>
        </form>
      </section>

      <section>
        <h2>Historial</h2>

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
                  <th>Grupo</th>
                  <th>Estado</th>
                  <th>Fecha</th>
                </tr>
              </thead>
              <tbody>
                {publications.data.map((pub) => (
                  <tr key={pub.id}>
                    <td title={pub.text}>{truncate(pub.text)}</td>
                    <td>{groupName(pub.telegram_id)}</td>
                    <td>
                      <span className={`badge badge-${pub.status}`}>{STATUS_LABEL[pub.status]}</span>
                    </td>
                    <td>{formatDate(pub.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </main>
  )
}
