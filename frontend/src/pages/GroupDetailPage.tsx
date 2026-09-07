// Detalle de un grupo (spec frontend-dashboard req 3). En el PR3 se
// implementa la carga real con useGroup; aqui queda el shell con la
// navegacion a las secciones para que las rutas existan.
import { Link, useParams } from 'react-router-dom'

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>()

  return (
    <section>
      <h1>Detalle del grupo {id}</h1>
      <p>La información del grupo se carga en el próximo cambio del panel.</p>
      <nav className="group-sections">
        <Link to={`/groups/${id}/users`}>Usuarios</Link>
        <Link to={`/groups/${id}/requests`}>Solicitudes de ingreso</Link>
        <Link to={`/groups/${id}/logs`}>Logs</Link>
      </nav>
    </section>
  )
}