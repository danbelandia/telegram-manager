// Ruta raiz de la app: la ruta / redirige a /groups (spec frontend-routing).
// Las rutas autenticadas viven en el layout padre con <Outlet /> (guia
// frontend seccion 8).
//
// Slice 2 de moderacion automatica (Fase 3): ruta /groups/:id/automation
// para el editor de settings + listas.
// Slice 3 (Fase 3): ruta /groups/:id/moderation para el dashboard de
// observacion (stats + advertencias activas + reset manual).
import { Navigate, Route, Routes } from 'react-router-dom'
import RequireAuth from './components/RequireAuth'
import PublicOnly from './components/PublicOnly'
import Layout from './components/Layout'
import LandingPage from './pages/LandingPage'
import TermsPage from './pages/TermsPage'
import PrivacyPage from './pages/PrivacyPage'
import SignupPage from './pages/SignupPage'
import LoginPage from './pages/LoginPage'
import GroupsPage from './pages/GroupsPage'
import GroupDetailPage from './pages/GroupDetailPage'
import GroupUsersPage from './pages/GroupUsersPage'
import GroupRequestsPage from './pages/GroupRequestsPage'
import GroupLogsPage from './pages/GroupLogsPage'
import GroupAutomationPage from './pages/GroupAutomationPage'
import GroupModerationPage from './pages/GroupModerationPage'
import PublicationsPage from './pages/PublicationsPage'
import TenantSettingsPage from './pages/TenantSettingsPage'
import AdminTenantsPage from './pages/AdminTenantsPage'
import NotFoundPage from './pages/NotFoundPage'

export default function App() {
  return (
    <Routes>
      {/* Rutas publicas (spec frontend-routing): fuera de RequireAuth;
          PublicOnly manda a /groups cuando ya hay sesion. */}
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

      {/* Paginas legales: publicas para cualquier visitante
          (autenticado o no), sin PublicOnly — no redirigimos a /groups
          si el usuario ya esta logueado. */}
      <Route path="/terms" element={<TermsPage />} />
      <Route path="/privacy" element={<PrivacyPage />} />

      <Route
        element={
          <RequireAuth>
            <Layout />
          </RequireAuth>
        }
      >
        <Route path="/groups" element={<GroupsPage />} />
        <Route path="/dashboard" element={<Navigate to="/groups" replace />} />
        <Route path="/groups/:id" element={<GroupDetailPage />} />
        <Route path="/groups/:id/users" element={<GroupUsersPage />} />
        <Route path="/groups/:id/requests" element={<GroupRequestsPage />} />
        <Route path="/groups/:id/logs" element={<GroupLogsPage />} />
        <Route path="/groups/:id/automation" element={<GroupAutomationPage />} />
        <Route path="/groups/:id/moderation" element={<GroupModerationPage />} />
        <Route path="/publications" element={<PublicationsPage />} />
        <Route path="/tenant" element={<TenantSettingsPage />} />
        <Route path="/admin/tenants" element={<AdminTenantsPage />} />
      </Route>

      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}