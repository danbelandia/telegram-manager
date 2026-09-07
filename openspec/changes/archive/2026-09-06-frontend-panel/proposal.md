# Proposal — frontend-panel

**Estado**: DRAFT (para aprobación del usuario)

## Intención

Construir el panel React real que reemplaza el bootstrap actual:
rutas completas de la sección 16 del spec, login funcional contra el
backend de auth ya implementado, y dashboard con la lista de grupos
que expone `GET /api/groups`. Cubre los pasos 11 (React + routing) y
12 (Dashboard) del orden de implementación AGENTS §26.

## Problema

El frontend actual (`frontend/src/`) es un bootstrap de Vite + React +
Router: `LoginPage` y `DashboardPage` son placeholders y `App.tsx` solo
tiene rutas `/` y `/login`. El backend ya expone auth (login/refresh/
me/logout) y los endpoints REST de grupos/moderation/solicitudes/logs
(archivados en rest-api). No hay forma de operar el sistema desde el
navegador.

## Alcance

### Incluido (pasos 11 y 12 del spec)

- **Rutas completas** (§16):
  - `/login`
  - `/dashboard`
  - `/groups`
  - `/groups/:id`
  - `/groups/:id/users`
  - `/groups/:id/requests`
  - `/groups/:id/logs`
  - Rutas sin sesión redirigen a `/login` (`RequireAuth`)
  - 404 para rutas desconocidas
- **Login funcional** (§17/17.1): formulario email/username + password
  → `POST /api/auth/login`; access token en memoria (contexto
  `useAuth()`), refresh token en cookie httpOnly manejada por el
  navegador; estado de sesión restaurado con `GET /api/auth/me`;
  logout.
- **Dashboard**: lista de grupos desde `GET /api/groups` con los campos
  del §6 (nombre, username, ID, miembros si aplica, estado del bot,
  permiso de administrador) y botón `[Administrar]` → `/groups/:id`.
- **Detalle de grupo** (`/groups/:id`): vista con los datos del grupo +
  navegación a users/requests/logs (las páginas hijas quedan como
  placeholders mínimos **en este cambio** — su implementación real es
  un cambio posterior por trabajo unit y límite de 400 líneas por PR).

### Fuera de alcance (cambios futuros, mismo ciclo de fases)

- Gestión de usuarios (paso 13, ban/unban/mute UI — paso 14)
- Borrar/fijar mensajes (paso 15)
- Abrir/cerrar chat (paso 16)
- Aprobar/rechazar solicitudes (paso 17)
- Vista de logs real (paso 18)

## Enfoque técnico

- **Estructura de carpetas** según la guía de frontend
  (`frontend-react-skill.md` §1): `app/`, `pages/`, `features/`
  (auth → groups → dashboard), `components/`, `lib/`, `types/`.
- **Auth**: Context `AuthProvider` + hook `useAuth()` (guía §4). El
  access token vive en memoria (nunca localStorage, spec §17.1). El
  `api-client` recibe el token vía contexto y hace refresh transparente
  con retry único.
- **API**: `lib/api-client.ts` se extiende para inyectar
  `Authorization: Bearer` y manejar `401` → refresh → retry. Tipos de
  dominio en `features/*/types.ts` derivados de los DTOs del backend
  (guía §2, §3).
- **Dependencias nuevas propuestas** (guía §3, §5, §7):
  - `@tanstack/react-query` — data fetching (la única dependencia
    "extra" que la guía justifica explícitamente).
  - `react-hook-form` + `zod` — formulario de login (guía §5: forms con
    más de 2 campos; login tiene 2 — se propone incluir igualmente para
    dejar la base del patrón de forms listo, dado que users/ban vendrán
    con formularios reales; alternativa: solo `useState` en login y
    diferir RHF+Zod al próximo cambio. **DECISIÓN PENDIENTE**).
  - UI: sin librería de componentes en este cambio (guía §7 ofrece
    shadcn/ui/Radix como opción; para mínimo acorde al MVP, CSS plano +
    componentes simples propios). **DECISIÓN PENDIENTE**.
- **Tests**: Vitest + RTL. Login (submit → api → contexto), `useAuth`
  (restauración de sesión), `useGroups` con mock de api-client, rutas
  protegidas (RequireAuth redirige). No perseguir cobertura alta (guía
  §10, spec §21).
- **Config**: `VITE_API_BASE_URL` (ya soportada por api-client). Sin
  cambios en Dockerfile ni vite.config (server ya escucha 0.0.0.0).

## Decisiones pendientes (consecuencias de dependencias)

1. **RHF + Zod ahora o después** — la guía lo pide para forms >2 campos;
   el login tiene 2. Propuesta: incluirlos ahora para fundar el patrón
   de forms (ban/mute con motivo vienen en pasos 13-14). Alternativa:
   diferirlos. (Sin son incluidos: +2 deps runtime.)
2. **Librería UI** — propuesta: ninguna en este cambio (CSS plano),
   diferir shadcn/ui/Radix a cuando haya tablas reales (users/logs,
   pasos 13/18). Alternativa: shadcn/ui desde ahora. (Sin UI lib: 0 deps
   nuevas; guarda el límite de 400 líneas por PR.)

## Riesgos

- El límite de 400 líneas por PR (AGENTS §21/E de sdd-phase-common)
  obliga a dividir: PR1 estructura+routing+auth, PR2 dashboard+grupos.
  Sin remote (la chain de PRs sigue en local), se aplican commits
  encadenados como en rest-api.
- En desarrollo, el backend corre en `:8080` y Vite en `:5173` — el
  proxy de dev para `/api` sigue pendiente de configurar en
  vite.config (se incluye en este cambio, no requiere infra extra).

## Plan de entrega tentativo

- PR1: estructura de carpetas + Router completo + RequireAuth + layout
  shell + 404 + LoginPage funcional + api-client con auth + context
  useAuth + tests.
- PR2: DashboardPage con lista de grupos + GroupDetailPage básica +
  placeholders users/requests/logs + hooks TanStack Query + tests.

## Rollback

- Revertir los commits del cambio: el bootstrap anterior queda intacto
  en git; no hay cambios de base de datos ni de backend en este cambio
  (solo frontend, sin tocar infraestructura).