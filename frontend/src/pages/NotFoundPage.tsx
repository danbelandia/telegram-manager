// Vista 404 para rutas desconocidas (spec frontend-routing).
import { Link } from 'react-router-dom'

export default function NotFoundPage() {
  return (
    <main className="notfound-page">
      <h1>Página no encontrada</h1>
      <p>La ruta que buscas no existe.</p>
      <Link to="/dashboard" className="btn btn-primary">
        Ir al Dashboard
      </Link>
    </main>
  )
}