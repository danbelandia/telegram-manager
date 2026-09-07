// Vista de usuarios del grupo (spec frontend-moderation req 2-3): lista
// de administradores que Telegram expone (getChatAdministrators; la Bot
// API NO lista todos los miembros, AGENTS 7) + lookup puntual por
// userId + acciones ban/unban/mute/unmute con confirmacion para ban y
// mute (acciones destructivas, AGENTS 4).
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
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

// Acciones individuales por usuario.
function UserActions({ groupId, user }: { groupId: string; user: GroupUser }) {
  const ban = useBanUser()
  const unban = useUnbanUser()
  const mute = useMuteUser()
  const unmute = useUnmuteUser()

  const [actionError, setActionError] = useState<string | null>(null)

  const runAction = (fn: () => void) => {
    setActionError(null)
    try {
      fn()
    } catch (err) {
      setActionError(formatModerationError(err))
    }
  }

  const confirmBan = () => {
    if (!window.confirm(`¿Banear a ${user.first_name} del grupo? Esta acción es irreversible.`)) {
      return
    }
    runAction(() => ban.mutate({ groupId, userId: user.user_id }))
  }

  const confirmMute = () => {
    if (!window.confirm(`¿Mutear a ${user.first_name} (restringir envío de mensajes)?`)) {
      return
    }
    runAction(() => mute.mutate({ groupId, userId: user.user_id }))
  }

  return (
    <div className="user-actions">
      <button type="button" className="btn btn-danger" onClick={confirmBan}>
        Banear
      </button>
      <button type="button" className="btn" onClick={() => runAction(() => unban.mutate({ groupId, userId: user.user_id }))}>
        Desbanear
      </button>
      <button type="button" className="btn" onClick={confirmMute}>
        Mutear
      </button>
      <button type="button" className="btn" onClick={() => runAction(() => unmute.mutate({ groupId, userId: user.user_id }))}>
        Desmutear
      </button>
      {actionError ? <p className="state-block state-error">{actionError}</p> : null}
    </div>
  )
}

export default function GroupUsersPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const [lookupId, setLookupId] = useState('')
  const [lookupSubmitted, setLookupSubmitted] = useState('')

  const users = useGroupUsers(groupId)
  const lookup = useGroupUser(groupId, lookupSubmitted)

  return (
    <main className="page">
      <Link to={`/groups/${groupId}`} className="back-link">
        ← Volver al grupo
      </Link>

      <h1>Usuarios del grupo</h1>

      {/* Lookup puntual (la Bot API no lista todos los miembros). */}
      <form
        className="user-lookup"
        onSubmit={(e) => {
          e.preventDefault()
          setLookupSubmitted(lookupId.trim())
        }}
      >
        <input
          type="text"
          inputMode="numeric"
          placeholder="ID de Telegram del usuario"
          value={lookupId}
          onChange={(e) => setLookupId(e.target.value)}
        />
        <button type="submit" className="btn">
          Buscar
        </button>
      </form>

      {lookupSubmitted ? (
        <section className="user-lookup-result">
          <h2>Resultado de búsqueda</h2>
          {lookup.isPending ? <p>Cargando…</p> : null}
          {lookup.isError ? (
            <p className="state-block state-error">
              {lookup.error instanceof Error ? lookup.error.message : 'No se pudo consultar al usuario.'}
            </p>
          ) : null}
          {lookup.data ? (
            <div className="group-card">
              <p>
                <strong>{lookup.data.first_name}</strong>
                {lookup.data.username ? ` @${lookup.data.username}` : ''}
                <span className="group-card-meta">{lookup.data.status}</span>
              </p>
              <UserActions groupId={groupId} user={lookup.data} />
            </div>
          ) : null}
        </section>
      ) : null}

      <h2>Administradores</h2>
      {users.isPending ? <p className="state-block">Cargando usuarios…</p> : null}

      {users.isError ? (
        <div className="state-block state-error">
          <p>{users.error instanceof Error ? users.error.message : 'No se pudieron cargar los usuarios.'}</p>
          <button type="button" className="btn" onClick={() => users.refetch()}>
            Reintentar
          </button>
        </div>
      ) : null}

      {users.data && users.data.length === 0 ? (
        <p className="state-block">
          Sin administradores visibles. Telegram no expone la lista completa de miembros: el panel
          muestra los administradores y permite consultar usuarios puntuales por ID.
        </p>
      ) : null}

      {users.data && users.data.length > 0 ? (
        <div className="user-list">
          {users.data.map((u) => (
            <div key={u.user_id} className="group-card">
              <p>
                <strong>{u.first_name}</strong>
                {u.username ? ` @${u.username}` : ''}
                <span className="group-card-meta">{u.status}</span>
              </p>
              <UserActions groupId={groupId} user={u} />
            </div>
          ))}
        </div>
      ) : null}
    </main>
  )
}