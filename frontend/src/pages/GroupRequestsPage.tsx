// Solicitudes de ingreso del grupo (spec frontend-moderation req 6): el
// backend procesa los eventos de join_request de Telegram y esta vista
// permite aprobar/rechazar las pendientes. Estados loading/vacio/error
// y feedback de accion con manejo de concurrencia ("ya fue decidida").
import { Link, useParams } from 'react-router-dom'
import { formatModerationError } from '../features/moderation/error'
import { useApproveJoinRequest, useJoinRequests, useRejectJoinRequest } from '../features/moderation/hooks'
import type { JoinRequest } from '../features/moderation/types'

const STATUS_LABEL: Record<JoinRequest['status'], string> = {
  pending: 'Pendiente',
  approved: 'Aprobada',
  rejected: 'Rechazada',
}

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })
}

export default function GroupRequestsPage() {
  const { id } = useParams<{ id: string }>()
  const groupId = id ?? ''

  const requests = useJoinRequests(groupId)
  const approve = useApproveJoinRequest()
  const reject = useRejectJoinRequest()

  // Los errores de mutate() son asincronos: llegan por mutation.error,
  // no por excepcion sincrona (ver GroupUsersPage).
  const actionError = approve.error ?? reject.error
  const pending = approve.isPending || reject.isPending
  const lastResult = approve.isSuccess || reject.isSuccess ? 'Decisión enviada.' : null

  return (
    <main className="page">
      <Link to={`/groups/${groupId}`} className="back-link">
        ← Volver al grupo
      </Link>

      <h1>Solicitudes de ingreso</h1>

      {lastResult ? <p className="state-block state-ok">{lastResult}</p> : null}
      {actionError ? <p className="state-block state-error">{formatModerationError(actionError)}</p> : null}

      {requests.isPending ? <p className="state-block">Cargando solicitudes…</p> : null}

      {requests.isError ? (
        <div className="state-block state-error">
          <p>{requests.error instanceof Error ? requests.error.message : 'No se pudieron cargar las solicitudes.'}</p>
          <button type="button" className="btn" onClick={() => requests.refetch()}>
            Reintentar
          </button>
        </div>
      ) : null}

      {requests.data && requests.data.length === 0 ? (
        <p className="state-block">Sin solicitudes de ingreso pendientes ni resueltas.</p>
      ) : null}

      {requests.data && requests.data.length > 0 ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Usuario</th>
                <th>Fecha</th>
                <th>Estado</th>
                <th>Acciones</th>
              </tr>
            </thead>
            <tbody>
              {requests.data.map((req) => (
                <tr key={req.id}>
                  <td>
                    <strong>{req.first_name}</strong>
                    {req.username ? ` @${req.username}` : ''}
                  </td>
                  <td>{formatDate(req.requested_at)}</td>
                  <td>
                    <span className={`badge badge-${req.status}`}>{STATUS_LABEL[req.status]}</span>
                  </td>
                  <td>
                    {req.status === 'pending' ? (
                      <>
                        <button
                          type="button"
                          className="btn"
                          disabled={pending}
                          onClick={() => approve.mutate({ groupId, requestId: req.id })}
                        >
                          Aprobar
                        </button>{' '}
                        <button
                          type="button"
                          className="btn"
                          disabled={pending}
                          onClick={() => reject.mutate({ groupId, requestId: req.id })}
                        >
                          Rechazar
                        </button>
                      </>
                    ) : (
                      <span className="state-block-weak">
                        {req.decided_by ? `Decidida por admin ${req.decided_by}` : ''}
                      </span>
                    )}
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