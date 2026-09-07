// Detalle de un grupo (spec frontend-dashboard req 3 + frontend-moderation
// req 4-5): carga con useGroup, muestra datos administrables, navega a las
// secciones hijas (AGENTS 12) y ejecuta acciones de chat lock/unlock con
// confirmacion y de mensajes delete/pin por messageId (la Bot API no
// lista mensajes, AGENTS 8; el admin provee el id).
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { formatModerationError } from '../features/moderation/error'
import { useDeleteMessage, useLockGroup, usePinMessage, useUnlockGroup } from '../features/moderation/hooks'
import { formatPermissions } from '../features/groups/permissions'
import { useGroup } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''
  const { data: group, isPending, isError, error, refetch } = useGroup(groupId)

  const lock = useLockGroup()
  const unlock = useUnlockGroup()
  const deleteMsg = useDeleteMessage()
  const pin = usePinMessage()

  const [messageId, setMessageId] = useState('')

  const chatError = lock.error ?? unlock.error
  const messageError = deleteMsg.error ?? pin.error
  const chatPending = lock.isPending || unlock.isPending
  const messagePending = deleteMsg.isPending || pin.isPending

  const confirmLock = () => {
    if (!window.confirm('¿Cerrar el envío de mensajes en este grupo?')) return
    lock.mutate(groupId)
  }

  const confirmDelete = () => {
    const parsed = Number(messageId)
    if (!Number.isInteger(parsed) || parsed <= 0) {
      window.alert('Ingresá un ID de mensaje válido (número positivo).')
      return
    }
    if (!window.confirm(`¿Eliminar el mensaje ${parsed}? Esta acción es irreversible.`)) return
    deleteMsg.mutate({ groupId, messageId: parsed })
  }

  const pinMessage = () => {
    const parsed = Number(messageId)
    if (!Number.isInteger(parsed) || parsed <= 0) {
      window.alert('Ingresá un ID de mensaje válido (número positivo).')
      return
    }
    pin.mutate({ groupId, messageId: parsed })
  }

  if (isPending) {
    return <main className="page">Cargando grupo…</main>
  }

  if (isError) {
    return (
      <main className="page">
        <h1>Grupo</h1>
        <div className="state-block state-error">
          <p>{error instanceof Error ? error.message : 'No se pudo cargar el grupo.'}</p>
          <button type="button" className="btn" onClick={() => refetch()}>
            Reintentar
          </button>
        </div>
      </main>
    )
  }

  if (!group) {
    return (
      <main className="page">
        <h1>Grupo</h1>
        <p className="state-block">Grupo no encontrado.</p>
        <Link to="/groups" className="btn">
          Volver a grupos
        </Link>
      </main>
    )
  }

  return (
    <main className="page">
      <Link to="/groups" className="back-link">
        ← Grupos
      </Link>

      <h1>{group.title}</h1>
      {group.username ? <p className="group-card-username">@{group.username}</p> : null}

      <dl className="group-detail">
        <div>
          <dt>ID de Telegram</dt>
          <dd>{group.telegram_id}</dd>
        </div>
        <div>
          <dt>Tipo</dt>
          <dd>{group.type}</dd>
        </div>
        <div>
          <dt>Miembros</dt>
          <dd>{formatMembers(group.member_count)}</dd>
        </div>
        <div>
          <dt>Bot</dt>
          <dd>{group.bot_status}</dd>
        </div>
        <div>
          <dt>Permisos del bot</dt>
          <dd>
            {formatPermissions(group.bot_permissions).join(', ') || 'Sin permisos'}
          </dd>
        </div>
      </dl>

      <section className="detail-actions">
        <h2>Envío de mensajes</h2>
        {chatError ? <p className="state-block state-error">{formatModerationError(chatError)}</p> : null}
        <button type="button" className="btn" disabled={chatPending} onClick={() => unlock.mutate(groupId)}>
          🔓 Abrir chat
        </button>{' '}
        <button type="button" className="btn btn-danger" disabled={chatPending} onClick={confirmLock}>
          🔒 Cerrar chat
        </button>
      </section>

      <section className="detail-actions">
        <h2>Mensajes</h2>
        {messageError ? <p className="state-block state-error">{formatModerationError(messageError)}</p> : null}
        <div className="user-lookup">
          <input
            type="text"
            inputMode="numeric"
            placeholder="ID del mensaje en Telegram"
            value={messageId}
            onChange={(e) => setMessageId(e.target.value)}
          />
          <button type="button" className="btn btn-danger" disabled={messagePending} onClick={confirmDelete}>
            Eliminar
          </button>{' '}
          <button type="button" className="btn" disabled={messagePending} onClick={pinMessage}>
            Fijar
          </button>
        </div>
      </section>

      <nav className="group-sections">
        <Link to={`/groups/${groupId}/users`}>Membresía y moderación</Link>
        <Link to={`/groups/${groupId}/requests`}>Solicitudes de ingreso</Link>
        <Link to={`/groups/${groupId}/logs`}>Logs</Link>
      </nav>
    </main>
  )
}