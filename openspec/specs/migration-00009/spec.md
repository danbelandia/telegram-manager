# Migration 00009 Specification

## Purpose

Migración goose `00009` que introduce multitenancy sin romper deploys
existentes: crea `tenants`, agrega `tenant_id` en todo menos `users`,
backfillea el tenant `default` de forma idempotente y refuerza la
unicidad de grupos por tenant.

## Requirements

### Requirement: Tabla tenants y columnas tenant_id

La migración MUST crear la tabla `tenants` (con `slug` UNIQUE) y
agregar `tenant_id` (FK a `tenants`) a `groups`, `group_members`,
`admins`, `join_requests`, `warnings`, `logs` (y publicaciones cuando
existan). `users` MUST NOT llevar `tenant_id`.

#### Scenario: Esquema tras migrar en fresco

- GIVEN una base vacía
- WHEN se aplican las migraciones goose
- THEN existe `tenants` y cada tabla listada tiene `tenant_id`, salvo
  `users`

### Requirement: Backfill default idempotente

La migración MUST crear el tenant `default` si no existe y asignar su
id a toda fila preexistente con `tenant_id` NULL, de forma idempotente
y re-aplicable: correrla dos veces MUST dejar el mismo estado.

#### Scenario: Migración sobre datos existentes

- GIVEN una base con groups y admins sin `tenant_id`
- WHEN se aplica `00009`
- THEN existe el tenant `default` y esas filas apuntan a él

#### Scenario: Re-aplicación segura

- GIVEN `00009` ya aplicada con tenant `default` existente
- WHEN se re-ejecuta el backfill
- THEN no se duplican tenants ni se modifican filas ya asignadas

### Requirement: Unicidad compuesta en groups

La migración MUST reemplazar la unicidad global de `groups.telegram_id`
por `UNIQUE(tenant_id, telegram_id)`: el mismo grupo de Telegram MAY
existir en dos tenants, pero MUST NOT duplicarse dentro del mismo.

#### Scenario: Mismo grupo en dos tenants

- GIVEN el `telegram_id` T en el tenant A
- WHEN se persiste T para el tenant B
- THEN se crea una fila nueva sin violar unicidad

#### Scenario: Duplicado en el mismo tenant

- GIVEN el `telegram_id` T ya persistido en el tenant A
- WHEN se persiste T de nuevo para el tenant A
- THEN se actualiza la fila (upsert), no se duplica

### Requirement: Orden de aplicación

La migración MUST aplicar los cambios en orden: `tenants` primero,
luego `groups`, luego FKs, luego el resto de tablas, para no violar
referencias en ningún paso intermedio.

#### Scenario: Aplicación en fresco

- GIVEN una base vacía
- WHEN se aplica `00009` de cero
- THEN la migración completa sin errores de FK
