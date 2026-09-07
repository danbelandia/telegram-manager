// Logs de auditoria del grupo (spec frontend-moderation req 7): todas
// las acciones administrativas se registran en el backend (AGENTS 11) y
// esta vista las muestra de la mas reciente a la mas antigua, con
// accion, status, target y fecha legible.
import { Link, useParams } from 'react-router-dom'
import { useGroupLogs } from '../features/moderation/hooks'

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

export default function GroupLogsPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const logs = useGroupLogs(groupId)

  return (
    <main className="page">
      <Link to={`/groups/${groupId}`} className="back-link">
        ← Volver al grupo
      </Link>

      <h1>Logs del grupo</h1>

      {logs.isPending ? <p className="state-block">Cargando logs…</p> : null}

      {logs.isError ? (
        <div className="state-block state-error">
          <p>{logs.error instanceof Error ? logs.error.message : 'No se pudieron cargar los logs.'}</p>
          <button type="button" className="btn" onClick={() => logs.refetch()}>
            Reintentar
          </button>
        </div>
      ) : null}

      {logs.data && logs.data.length === 0 ? (
        <p className="state-block">Sin acciones registradas en este grupo.</p>
      ) : null}

      {logs.data && logs.data.length > 0 ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Acción</th>
                <th>Estado</th>
                <th>Detalle</th>
              </tr>
            </thead>
            <tbody>
              {logs.data.map((entry) => (
                <tr key={entry.id}>
                  <td>{formatDate(entry.created_at)}</td>
                  <td>
                    <strong>{entry.action}</strong>
                    {entry.target_user_id ? ` → usuario ${entry.target_user_id}` : ''}
                  </td>
                  <td>
                    <span className={`badge badge-${entry.status.toLowerCase()}`}>{entry.status}</span>
                  </td>
                  <td>
                    {entry.error_message ? entry.error_message : entry.actor_id ? `por admin ${entry.actor_id}` : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </main>
  )
}