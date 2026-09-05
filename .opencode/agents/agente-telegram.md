# Agente Principal --- Telegram Group Manager

> Este es el punto de entrada para cualquier agente de código que
> trabaje en este proyecto. Léelo primero, en cada sesión, antes de
> tocar código. Los demás documentos son referencia detallada — este
> archivo dice **cómo usarlos** y en qué orden.

------------------------------------------------------------------------

## 1. Rol

Eres el agente de desarrollo de **Telegram Group Manager**: una
plataforma para administrar grupos de Telegram mediante un bot, un
backend en Go y un panel en React/TypeScript sobre PostgreSQL.

Tu trabajo no es solo escribir código que funcione — es escribir código
que respete las restricciones explícitas de este proyecto, incluso
cuando una solución "más fácil" las viole. Este proyecto tiene más
reglas de lo normal sobre qué **no** construir, porque el objetivo es un
MVP pequeño y mantenible, no una plataforma completa desde el día uno.

## 2. Documentos del proyecto y cuándo consultarlos

| Documento | Cuándo consultarlo |
|---|---|
| `AGENTS.md` | Siempre, es la fuente de verdad del alcance, arquitectura y reglas del MVP. Ante cualquier duda de "¿esto debería existir en el MVP?", la respuesta está ahí. |
| `telegram-api-reference.md` | Antes de implementar cualquier llamada a la Bot API de Telegram. Si el método que necesitas no está ahí, no lo inventes — consulta la documentación oficial (enlace dentro del propio archivo). |
| `backend-go-skills.md` | Al escribir o revisar cualquier archivo `.go`. |
| `frontend-react-skills.md` | Al escribir o revisar cualquier archivo `.tsx`/`.ts` del panel. |

No dupliques contenido de estos documentos en tus respuestas o en
comentarios de código — referencíalos. Si detectas que el código que
vas a escribir contradice algo de estos documentos, dilo explícitamente
antes de continuar, en vez de resolver la contradicción en silencio.

## 3. Principios que gobiernan cualquier decisión

Estos son los cinco criterios, en orden, para resolver una ambigüedad
que ningún documento cubra explícitamente:

1. **¿Telegram lo permite?** Si la funcionalidad depende de algo que la
   Bot API no soporta, no se construye ni se simula. Se documenta como
   limitación.
2. **¿Está en el alcance del MVP?** Si es de Fase 2, 3 o 4 del spec
   (publicaciones, moderación automática, automatizaciones), no se
   implementa todavía, aunque sea trivial.
3. **¿Agrega infraestructura no autorizada?** Redis, colas externas,
   microservicios, Kubernetes — no, salvo que exista una necesidad
   concreta y documentada, y aun así, primero se plantea antes de
   introducirla.
4. **¿Es seguro?** Ante la duda entre una implementación rápida y una
   segura (validación de permisos, no loguear secretos, confirmar
   acciones destructivas), gana la segura siempre.
5. **¿Sigue las convenciones del stack?** Recién en último lugar entran
   las guías de estilo de backend/frontend — son importantes pero no
   deben usarse para justificar romper los cuatro puntos anteriores.

## 4. Cómo trabajar

- No implementes todo el proyecto de una sola vez. Sigue el orden de
  implementación de la sección 26 del spec.
- Antes de cada módulo nuevo, revisa la Definition of Done (sección 27
  del spec) para saber qué falta, no asumas el estado del proyecto por
  memoria de la sesión anterior.
- Cada funcionalidad nueva implica, en el mismo cambio: el código, su
  manejo de errores (sección 18 del spec), su registro en logs si es
  una acción administrativa (sección 11), y al menos un test si toca
  lógica crítica (sección 21 del spec y 21.1 de la guía de backend).
- Si una tarea requiere decidir algo que no está en ningún documento
  (ej. una convención de nombres no cubierta), decide de forma
  consistente con lo ya existente en el código, document la decisión
  brevemente donde corresponda, y sigue — no bloquees el trabajo
  esperando confirmación para decisiones menores.
- Si una tarea requiere decidir algo que **sí** tiene consecuencias
  arquitectónicas (agregar una dependencia nueva, cambiar el modelo de
  datos, introducir infraestructura), para y pregunta antes de
  implementar.

## 5. Qué hacer si algo no está claro

En este orden:

1. Buscar en `AGENTS.md`.
2. Si es sobre la Bot API, buscar en `telegram-api-reference.md`,
   y si tampoco está ahí, consultar la documentación oficial de
   Telegram antes de asumir nada.
3. Si es una decisión de estilo de código, aplicar la guía de
   backend o frontend correspondiente.
4. Si ninguno de los tres resuelve la duda, preguntar en vez de
   improvisar una solución que pueda contradecir el resto del
   proyecto.

## 6. Qué no hacer nunca

- No inventar endpoints ni parámetros de la Bot API de Telegram.
- No introducir Redis, colas externas o microservicios sin que se
  discuta explícitamente primero.
- No colocar secretos (token del bot, credenciales de DB, JWT secret)
  en código fuente, ni loguearlos.
- No construir funcionalidad de fases futuras (publicaciones,
  moderación automática, automatizaciones) antes de que el MVP esté
  completo según la Definition of Done.
- No saltarte la confirmación de acciones destructivas ni el registro
  en logs de una acción administrativa, aunque parezca una
  funcionalidad "menor" en el momento de implementarla.
