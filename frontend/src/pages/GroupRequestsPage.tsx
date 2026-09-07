// Placeholder de solicitudes de ingreso (spec frontend-routing: la ruta
// existe en este cambio; la implementacion real es un cambio posterior).
import { Link, useParams } from 'react-router-dom'

export default function GroupRequestsPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section className="page">
      <Link to={`/groups/${id}`} className="back-link">
        ← Volver al grupo
      </Link>
      <h1>Solicitudes de ingreso</h1>
      <p className="state-block">
        Aprobar y rechazar solicitudes de ingreso se implementa en un próximo cambio.
      </p>
    </section>
  )
}