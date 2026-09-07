# Guía de Frontend (React + TypeScript) --- Telegram Group Manager

> Complementa la especificación principal del proyecto (secciones 2, 16,
> 17). Se enfoca en convenciones concretas de código para el panel
> administrativo — no repite el listado de rutas ni el wireframe, que ya
> están en la sección 16 del spec.

------------------------------------------------------------------------

## 1. Estructura de carpetas

```
frontend/src/
├── app/               # bootstrap: router, providers globales
├── pages/             # una carpeta por ruta (Login, Groups, GroupDetail, ...)
├── features/          # lógica de negocio por dominio (groups, users, joinRequests, logs, auth)
│   └── groups/
│       ├── api.ts       # llamadas HTTP a /api/groups
│       ├── types.ts     # tipos del dominio
│       ├── hooks.ts     # hooks de datos (useGroups, useGroup)
│       └── components/  # componentes específicos de este feature
├── components/        # componentes de UI genéricos y reutilizables
├── lib/                # cliente HTTP, utilidades transversales
└── types/              # tipos compartidos entre features (ej. ApiError)
```

- Un componente que solo se usa en un feature vive dentro de ese
  feature, no en `components/` genérico — evita que `components/`
  termine siendo un cajón de sastre.
- Las rutas de la sección 16 del spec (`/groups/:id/users`, etc.) mapean
  1:1 a carpetas en `pages/`.

## 2. Tipado

- `strict: true` en `tsconfig.json`, sin excepciones. No usar `any`
  salvo en los límites con librerías de terceros sin tipos, y ahí
  documentando por qué con un comentario.
- Los tipos de dominio (`Group`, `GroupMember`, `JoinRequest`, `LogEntry`)
  se derivan del contrato del backend, no al revés. Si el backend
  documenta sus DTOs (por ejemplo, con comentarios en los handlers Go o
  un archivo OpenAPI simple), generar o transcribir los tipos desde ahí
  para que no diverjan.
- Preferir `type` sobre `interface` para uniones y tipos de dominio
  simples; `interface` cuando se espera que algo se extienda (props de
  componentes reutilizables).

## 3. Llamadas a la API

- Un único cliente HTTP centralizado en `lib/api-client.ts` que:
  - agrega el JWT a cada request (header `Authorization: Bearer ...`),
  - maneja el refresh de token de forma transparente (interceptor o
    wrapper) según la estrategia de la sección 17.1 del spec,
  - normaliza errores al envelope que devuelve el backend (sección 6
    de la guía de backend: `{data, error}`), para que los componentes
    nunca parseen la respuesta cruda.
- Nunca guardar el access token en `localStorage` — vive en memoria
  (estado de la app), y el refresh token es una cookie httpOnly que el
  navegador maneja solo (consistente con 17.1 del spec de backend). El
  frontend nunca lee ni escribe el refresh token directamente.
- Data fetching con **TanStack Query** (`@tanstack/react-query`) en vez
  de `useEffect` + `fetch` manual: da cache, revalidación y estados de
  loading/error consistentes sin reinventar la rueda en cada feature.
  Es la única dependencia "extra" que vale la pena para un panel con
  tantas vistas de datos remotos. Los hooks por feature viven en
  `features/<dominio>/hooks.ts` (`useGroups`, `useGroup`, ...) con
  `retry: false` — el UI decide cuándo reintentar (botón manual).
- En tests, mockear `fetch` por **URL** (helper `test/helpers.tsx`,
  `mockFetchRoutes`) y no con `mockResolvedValueOnce` encadenados: el
  refresh automático ante 401 consume respuestas destinadas a otras
  llamadas y desalinea los mocks por orden. `test/setup.ts` registra
  `afterEach(cleanup)` porque Vitest corre sin `globals: true` y RTL no
  auto-limpia el DOM entre tests.

## 4. Manejo de estado

- Estado de servidor (grupos, usuarios, logs, solicitudes) vive en
  TanStack Query — no se duplica en Redux ni Context.
- Estado de UI puro (modal abierto, tab activo, filtros de una tabla)
  vive en `useState`/`useReducer` local al componente o feature. No
  introducir una librería de estado global (Redux, Zustand) salvo que
  aparezca una necesidad concreta de compartir estado de UI entre
  partes muy distantes del árbol — igual que la sección 2 del spec
  pide evitar infraestructura sin necesidad concreta, aplica también
  al frontend.
- Sesión de auth (usuario logueado, permisos) en un Context simple con
  un hook `useAuth()` — es el único caso legítimo de Context global en
  este proyecto. Vive en `lib/auth-context.tsx` (`AuthProvider` +
  `useAuth`): expone `{loading, user, login, logout}`; `user === null`
  significa sin sesión.

## 5. Formularios y validación

- **React Hook Form** + **Zod** para cualquier formulario con más de 2
  campos (login, filtros de usuarios, formulario de ban con motivo,
  etc.). El schema de Zod puede reusarse para validar en cliente antes
  de enviar, reduciendo round-trips innecesarios al backend — pero
  nunca reemplaza la validación del backend (sección 4 del spec:
  "validación de todos los parámetros recibidos" es responsabilidad
  del servidor, el cliente es solo UX).
- Mensajes de error de formulario en español, específicos ("El motivo
  del ban es obligatorio"), nunca genéricos ("Campo inválido").

## 6. Acciones destructivas

- Toda acción irreversible o sensible (banear, mutear, cerrar el chat,
  eliminar mensaje) pasa por una confirmación explícita antes de llamar
  a la API — esto traduce a UI el requisito de la sección 4 del spec
  ("confirmación para acciones destructivas importantes"). En el MVP se
  usa `window.confirm` (patrón de `GroupUsersPage`/`GroupDetailPage`),
  no un diálogo custom: es lo más simple y tests lo stubbean directo.
- El botón de confirmar queda deshabilitado mientras la request está en
  vuelo (`mutation.isPending`), y el resultado (éxito o mensaje de
  error §18) se muestra debajo — **no** cerrar de forma optimista.
- Los errores de `mutate()` son asíncronos: **no** envolver la llamada
  en try/catch (no captura nada). Leerlos de `mutation.error` y
  formatearlos con `formatModerationError` (features/moderation/error),
  que usa el mensaje legible del backend (§18) si llega.

## 7. Componentes de UI

- Librería de componentes: shadcn/ui (Radix + Tailwind) es una buena
  opción para este panel — da accesibilidad correcta out of the box
  (diálogos, dropdowns, tablas) sin atarse a un design system pesado.
  Alternativa más liviana: Radix Primitives solos + estilos propios,
  si se prefiere no depender de la CLI de shadcn.
- Tablas de datos (usuarios, logs, solicitudes de ingreso): usar
  **TanStack Table** para sorting/paginación/filtrado del lado
  cliente, en vez de reimplementar esa lógica a mano en cada vista.
- Priorizar funcionalidad sobre diseño visual avanzado, tal como pide
  la sección 16 del spec — no invertir tiempo en animaciones o
  theming custom en el MVP.

## 8. Enrutamiento

- **React Router** (ya definido en el stack, sección 2 del spec), con
  rutas protegidas mediante un wrapper `<RequireAuth>` que redirige a
  `/login` si no hay sesión válida, en vez de repetir esa lógica en
  cada página.
- Layout persistente (sidebar + header, ver wireframe de la sección 16)
  como ruta padre con `<Outlet />`, no duplicado en cada página.

## 9. Variables de entorno del frontend

- Solo variables que empiecen con el prefijo que exija Vite
  (`VITE_...`) llegan al bundle del navegador — cualquier cosa sin ese
  prefijo se queda en el proceso de build y no es accesible desde el
  código cliente. Esto es también una garantía de seguridad: confirma
  que nada sensible (como el token de Telegram) puede terminar
  expuesto en el frontend por error, reforzando la regla 18 de la
  sección 25 del spec.
- `VITE_API_BASE_URL` como única variable indispensable para el MVP.

## 10. Testing

- **Vitest** (se integra nativo con Vite, mismo motor que el build) +
  **React Testing Library** para componentes y hooks.
- Priorizar tests de: lógica de formularios con validación, hooks de
  datos (`useGroups`, etc.) con mocks de la API, y flujos críticos
  (login, confirmación de ban) — no perseguir cobertura alta en
  componentes puramente visuales, consistente con el criterio de
  testing de la sección 21 del spec de backend.
- Patrones establecidos (`features/moderation`):
  - **Mock de fetch por substring de URL** (`src/test/helpers.tsx`):
    `mockFetchRoutes({ '/api/.../users': () => okJson(...) })`. Las
    claves anidadas (`.../users/42/ban`) deben ir **antes** que las que
    son prefijo de ellas (`.../users`), porque matchea en orden de
    inserción.
  - **Confirmación**: `window.confirm = vi.fn(() => true|false)` antes
    de renderizar. Tener cuidado con `vi.stubGlobal('confirm', ...)` y
    `vi.spyOn(window, 'confirm')` — en jsdom no siempre aplican; la
    asignación directa funciona.
  - **Rutas con parámetros** (`/groups/:id`): envolver el componente en
    `<MemoryRouter><Routes><Route path=... /></Routes></MemoryRouter>`
    para que `useParams` reciba el id — renderizar el componente suelto
    deja `useParams` vacío.
  - **Errores de mutación**: asserton `mutation.error` derivado
    (`approve.error ?? reject.error`), no excepciones sincrónicas.
