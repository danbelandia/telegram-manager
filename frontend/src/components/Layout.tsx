// Layout persistente del panel (wireframe AGENTS 16): sidebar + header
// y las rutas hijas se renderizan en <Outlet /> — no se duplica en cada
// pagina (guia frontend seccion 8).
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth-context'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `layout-nav-link${isActive ? ' layout-nav-link-active' : ''}`

export default function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="layout">
      <aside className="layout-sidebar">
        <div className="layout-brand">Telegram Manager</div>
        <nav className="layout-nav">
          <NavLink to="/dashboard" end className={navLinkClass}>
            Dashboard
          </NavLink>
          <NavLink to="/groups" className={navLinkClass}>
            Grupos
          </NavLink>
          <NavLink to="/publications" className={navLinkClass}>
            Publicaciones
          </NavLink>
        </nav>
      </aside>

      <div className="layout-main">
        <header className="layout-header">
          {user ? <span className="layout-user">{user.username}</span> : null}
          <button type="button" className="btn btn-secondary" onClick={handleLogout}>
            Salir
          </button>
        </header>
        <main className="layout-content">
          <Outlet />
        </main>
      </div>
    </div>
  )
}