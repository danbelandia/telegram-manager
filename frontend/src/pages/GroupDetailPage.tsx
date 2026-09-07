// Detalle de un grupo (spec frontend-dashboard req 3): carga con
// useGroup, muestra datos administrables y navega a las secciones de
// usuarios/solicitudes/logs (AGENTS 12). Estados loading/error/vacio.
import { Link, useParams } from 'react-router-dom'
import { useGroup } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { data: group, isPending, isError, error, refetch } = useGroup(id ?? '')

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
          <dt>Permisos</dt>
          <dd>
            {Object.entries(group.bot_permissions)
              .filter(([, v]) => v)
              .map(([k]) => k)
              .join(', ') || 'Sin permisos'}
          </dd>
        </div>
      </dl>

      <nav className="group-sections">
        <Link to={`/groups/${id}/users`}>Usuarios</Link>
        <Link to={`/groups/${id}/requests`}>Solicitudes de ingreso</Link>
        <Link to={`/groups/${id}/logs`}>Logs</Link>
      </nav>
    </main>
  )
}