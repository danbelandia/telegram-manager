// Placeholder de usuarios del grupo (spec frontend-routing: la ruta
// existe en este cambio; la implementacion real es un cambio posterior).
import { useParams } from 'react-router-dom'

export default function GroupUsersPage() {
  const { id } = useParams<{ id: string }>()
  return (
    <section>
      <h1>Usuarios del grupo</h1>
      <p>Grupo {id}. La gestión de usuarios se implementa en un próximo cambio.</p>
    </section>
  )
}