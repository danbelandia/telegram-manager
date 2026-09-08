// Ruta raiz de la app: el dashboard se muestra en /dashboard y la raiz
// redirige ahi (spec frontend-routing). Las rutas autenticadas viven en
// el layout padre con <Outlet /> (guia frontend seccion 8).
//
// Slice 2 de moderacion automatica (Fase 3): ruta /groups/:id/automation
// para el editor de settings + listas.
import { Navigate, Route, Routes } from 'react-router-dom'
import RequireAuth from './components/RequireAuth'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import GroupsPage from './pages/GroupsPage'
import GroupDetailPage from './pages/GroupDetailPage'
import GroupUsersPage from './pages/GroupUsersPage'
import GroupRequestsPage from './pages/GroupRequestsPage'
import GroupLogsPage from './pages/GroupLogsPage'
import GroupAutomationPage from './pages/GroupAutomationPage'
import PublicationsPage from './pages/PublicationsPage'
import NotFoundPage from './pages/NotFoundPage'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />

      <Route
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/groups" element={<GroupsPage />} />
        <Route path="/groups/:id" element={<GroupDetailPage />} />
        <Route path="/groups/:id/users" element={<GroupUsersPage />} />
        <Route path="/groups/:id/requests" element={<GroupRequestsPage />} />
        <Route path="/groups/:id/logs" element={<GroupLogsPage />} />
        <Route path="/groups/:id/automation" element={<GroupAutomationPage />} />
        <Route path="/publications" element={<PublicationsPage />} />
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}