// Placeholder de usuarios del grupo (spec frontend-routing: la ruta
// existe en este cambio; la implementacion real es un cambio posterior).
import { Link, useParams } from 'react-router-dom'

export default function GroupUsersPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section className="page">
      <Link to={`/groups/${id}`} className="back-link">
        ← Volver al grupo
      </Link>
      <h1>Usuarios del grupo</h1>
      <p className="state-block">
        La gestión de usuarios (búsqueda, baneo, muteo) se implementa en un próximo cambio.
      </p>
    </section>
  )
}