// Placeholder de logs del grupo (spec frontend-routing: la ruta existe
// en este cambio; la implementacion real es un cambio posterior).
import { useParams } from 'react-router-dom'

export default function GroupLogsPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section>
      <h1>Logs del grupo</h1>
      <p>Grupo {id}. Los logs se implementan en un próximo cambio.</p>
    </section>
  )
}