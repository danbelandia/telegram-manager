// Dashboard: lista de grupos administrables (AGENTS 6). TanStack Query
// con estados loading/vacio/error y reintento manual; cada grupo lleva
// "Administrar" -> /groups/:id (spec frontend-dashboard).
import { Link } from 'react-router-dom'
import { useGroups } from '../features/groups/hooks'

function formatMembers(count: number | null): string {
  if (count === null || count === undefined) return '—'
  return new Intl.NumberFormat('es-AR').format(count)
}

function GroupPermissions({ permissions }: { permissions: Record<string, boolean> }) {
  const active = Object.entries(permissions)
    .filter(([, value]) => value)
    .map(([key]) => key)
  if (active.length === 0) return <span>Sin permisos</span>
  return <span>{active.join(', ')}</span>
}

export default function DashboardPage() {
  const { data: groups, isPending, isError, error, refetch } = useGroups()

  return (
    <main className="page">
      <h1>Dashboard</h1>

      {isPending ? <p>Cargando grupos…</p> : null}

      {isError ? (
        <div className="state-block state-error">
          <p>
            {error instanceof Error
              ? error.message
              : 'No se pudieron cargar los grupos. Intente de nuevo.'}
          </p>
          <button type="button" className="btn" onClick={() => refetch()}>
            Reintentar
          </button>
        </div>
      ) : null}

      {!isPending && !isError && groups && groups.length === 0 ? (
        <p className="state-block">
          Todavía no hay grupos. Añadí el bot a un grupo y dale permisos de administrador.
        </p>
      ) : null}

      {!isPending && !isError && groups && groups.length > 0 ? (
        <ul className="group-list">
          {groups.map((g) => (
            <li key={g.id} className="group-card">
              <div className="group-card-info">
                <div className="group-card-title">
                  {g.title}
                  {g.username ? <span className="group-card-username">@{g.username}</span> : null}
                </div>
                <div className="group-card-meta">
                  <span>ID: {g.telegram_id}</span>
                  <span>Miembros: {formatMembers(g.member_count)}</span>
                  <span>Bot: {g.bot_status}</span>
                  <GroupPermissions permissions={g.bot_permissions} />
                </div>
              </div>
              <Link to={`/groups/${g.id}`} className="btn btn-primary">
                Administrar
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </main>
  )
}