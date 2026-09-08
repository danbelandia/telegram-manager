# Multi-Bot Registry Specification

## Purpose

Registro de N bots (uno por tenant) con un poller por tenant. El modo
sigue siendo global (`TELEGRAM_MODE`, Q2-global+polling); cada tenant
corre su propio poller con su token descifrado, aislado de los demás.

## Requirements

### Requirement: Un poller por tenant al boot

El sistema MUST, al arrancar, levantar un poller por cada tenant con
token registrado, usando su adapter propio. Un tenant sin token válido
MUST NOT impedir el arranque de los demás pollers.

#### Scenario: Boot con varios tenants

- GIVEN tenants A y B con tokens válidos registrados
- WHEN el backend arranca
- THEN hay un poller activo por tenant, cada uno consumiendo eventos
  de su propio bot

### Requirement: Poller en caliente tras signup

El sistema MUST levantar el poller del nuevo tenant al completarse un
signup, sin reiniciar el backend ni interrumpir los pollers existentes.

#### Scenario: Signup activa polling

- GIVEN un signup 201 para el tenant C con pollers A y B corriendo
- THEN el poller de C queda activo y los de A y B no se interrumpen

### Requirement: Token revocado con backoff degradado

Si Telegram rechaza el token de un tenant (revocado), su poller MUST
entrar en backoff con reintentos acotados y estado `degraded`, MUST
NOT tumbar a los otros pollers ni al backend, y MUST loggear el evento
sin exponer el token.

#### Scenario: Revocación aislada

- GIVEN pollers A y B corriendo, y el token de A es revocado
- WHEN Telegram rechaza las llamadas de A
- THEN el poller de A entra en backoff degradado y el de B sigue
  procesando con normalidad

### Requirement: Rate limiter por instancia

Cada instancia del adapter MUST aplicar un rate limiter token bucket de
~25 req/seg global, reintentando ante `429` según `retry_after` con un
máximo de 3 reintentos. El resto del backend MUST NOT preocuparse de
límites: solo invoca la interfaz.

#### Scenario: Ráfaga contenida

- GIVEN una ráfaga de acciones que supera 25 req/seg en un adapter
- WHEN se ejecutan
- THEN las llamadas se espacian sin superar el límite y sin errores
  visibles salvo `429` persistente tras 3 reintentos
