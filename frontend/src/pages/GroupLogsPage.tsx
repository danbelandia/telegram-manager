// Placeholder de logs del grupo (spec frontend-routing: la ruta existe
// en este cambio; la implementacion real es un cambio posterior).
import { Link, useParams } from 'react-router-dom'

export default function GroupLogsPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section className="page">
      <Link to={`/groups/${id}`} className="back-link">
        ← Volver al grupo
      </Link>
      <h1>Logs</h1>
      <p className="state-block">El historial de acciones administrativas se implementa en un próximo cambio.</p>
    </section>
  )
}