// Placeholder de solicitudes de ingreso (spec frontend-routing: la ruta
// existe en este cambio; la implementacion real es un cambio posterior).
import { useParams } from 'react-router-dom'

export default function GroupRequestsPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section>
      <h1>Solicitudes de ingreso</h1>
      <p>Grupo {id}. Las solicitudes se implementan en un próximo cambio.</p>
    </section>
  )
}