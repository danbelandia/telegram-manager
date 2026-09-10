// Ruta raiz de la app: el dashboard se muestra en /dashboard y la raiz
// redirige ahi (spec frontend-routing). Las rutas autenticadas viven en
// el layout padre con <Outlet /> (guia frontend seccion 8).
//
// Slice 2 de moderacion automatica (Fase 3): ruta /groups/:id/automation
// para el editor de settings + listas.
// Slice 3 (Fase 3): ruta /groups/:id/moderation para el dashboard de
// observacion (stats + advertencias activas + reset manual).
import { Route, Routes } from 'react-router-dom'
import RequireAuth from './components/RequireAuth'
import PublicOnly from './components/PublicOnly'
import Layout from './components/Layout'
import LandingPage from './pages/LandingPage'
import SignupPage from './pages/SignupPage'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import GroupsPage from './pages/GroupsPage'
import GroupDetailPage from './pages/GroupDetailPage'
import GroupUsersPage from './pages/GroupUsersPage'
import GroupRequestsPage from './pages/GroupRequestsPage'
import GroupLogsPage from './pages/GroupLogsPage'
import GroupAutomationPage from './pages/GroupAutomationPage'
import GroupModerationPage from './pages/GroupModerationPage'
import PublicationsPage from './pages/PublicationsPage'
import TenantSettingsPage from './pages/TenantSettingsPage'
import NotFoundPage from './pages/NotFoundPage'

export default function App() {
  return (
    <Routes>
      {/* Rutas publicas (spec frontend-routing): fuera de RequireAuth;
          PublicOnly manda a /dashboard cuando ya hay sesion. */}
      <Route
        path="/"
        element={
          <PublicOnly>
            <LandingPage />
          </PublicOnly>
        }
      />
      <Route
        path="/signup"
        element={
          <PublicOnly>
            <SignupPage />
          </PublicOnly>
        }
      />
      <Route
        path="/login"
        element={
          <PublicOnly>
            <LoginPage />
          </PublicOnly>
        }
      />

      <Route
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route path="/dashboard" element={<DashboardPage />} />
        <Route path="/groups" element={<GroupsPage />} />
        <Route path="/groups/:id" element={<GroupDetailPage />} />
        <Route path="/groups/:id/users" element={<GroupUsersPage />} />
        <Route path="/groups/:id/requests" element={<GroupRequestsPage />} />
        <Route path="/groups/:id/logs" element={<GroupLogsPage />} />
        <Route path="/groups/:id/automation" element={<GroupAutomationPage />} />
        <Route path="/groups/:id/moderation" element={<GroupModerationPage />} />
        <Route path="/publications" element={<PublicationsPage />} />
        <Route path="/tenant" element={<TenantSettingsPage />} />
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}