import { Route, Routes } from 'react-router-dom'
import DashboardPage from './pages/DashboardPage'
import LoginPage from './pages/LoginPage'

// Rutas base del panel (seccion 16 del spec). Cada pagina real se
// desarrolla en su propio cambio; aca viven los placeholders que
// permiten validar navegacion y build desde el bootstrap.
export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<DashboardPage />} />
    </Routes>
  )
}