# Payment Links

Plataforma para que una empresa genere referencias de pago, las comparta con su cliente y reciba la confirmación del proveedor financiero mediante un webhook.

- **Backend**: Go 1.25, PostgreSQL 17, API REST.
- **Frontend**: Next.js 15 (App Router) + TypeScript.

---

## Cómo correrlo

### Con Docker (recomendado)

```bash
git clone <repo> && cd dashboard
docker compose up --build
```

Eso es todo. El `docker-compose.yml` levanta en orden: PostgreSQL → migraciones → seed de empresas → API → dashboard. Cada paso espera al anterior con `depends_on` y condiciones reales (`service_healthy`, `service_completed_successfully`), no con esperas fijas.

| Servicio | URL |
|---|---|
| Dashboard | http://localhost:3000 |
| API | http://localhost:8080/health |
| PostgreSQL | `localhost:5432` (usuario/clave/base: `payment_links`) |

No hace falta `.env`: todas las variables tienen un valor por defecto que funciona. Copia `.env.example` a `.env` solo si necesitas cambiar puertos o activar la firma del webhook.

```bash
docker compose down     # parar (la base sobrevive)
docker compose down -v  # parar y borrar los datos
```

Con `make` disponible: `make up`, `make down`, `make logs`, `make test`, `make webhook-demo`, `make clean`. `make help` los lista.

### En local, sin Docker para la aplicación

Necesitas Go ≥ 1.25, Node ≥ 20 y un PostgreSQL. Lo más cómodo es levantar solo la base con Docker:

```bash
# 1. Base de datos, migraciones y datos de ejemplo
cd backend
make db-up migrate-up seed

# 2. API
cp .env.example .env
make run                       # o: go run ./cmd/api

# 3. Dashboard, en otra terminal
cd ../frontend
cp .env.example .env.local
npm install
npm run dev
```

El backend falla al arrancar si le falta `DATABASE_URL`, y dice exactamente qué variable falta.

### Tests

```bash
cd backend  && go test ./...     # 72 tests: dominio, casos de uso, HTTP e integración
cd frontend && npm run typecheck
```

Los tests de integración del backend levantan un PostgreSQL desechable con **testcontainers-go** y le aplican las migraciones reales. No hay que preparar nada. Si no hay Docker, se saltan explicando por qué y la suite unitaria sigue en verde. Con `TEST_DATABASE_URL` apuntando a una base ya existente la reutilizan, pero **hacen `TRUNCATE` de las tres tablas**: úsala solo con una base desechable.

`go test ./...` no incluye `-race` porque requiere cgo y en Windows sin toolchain de C falla; está en `make test-race`.

---

## Endpoints

Todas las respuestas son JSON. Los errores comparten un único sobre:

```json
{ "error": { "code": "duplicate_external_reference", "message": "a payment reference with that external_reference already exists" } }
```

El `code` es estable y pensado para que el cliente lo interprete; el `message` no lo es. Toda respuesta lleva la cabecera `X-Request-ID`, que aparece también en los logs del servidor.

### `POST /payment-references`

```bash
curl -X POST localhost:8080/payment-references \
  -H 'Content-Type: application/json' \
  -d '{
    "business_id": 1,
    "external_reference": "INV-10023",
    "amount": 125000.50,
    "description": "Factura 10023",
    "expires_at": "2026-10-01T23:59:59Z"
  }'
```

`201 Created`, con cabecera `Location: /payment-references/1`:

```json
{
  "id": 1,
  "business_id": 1,
  "external_reference": "INV-10023",
  "amount": 125000.50,
  "description": "Factura 10023",
  "status": "pending",
  "expires_at": "2026-10-01T23:59:59Z",
  "created_at": "2026-09-30T17:46:01.609836Z",
  "updated_at": "2026-09-30T17:46:01.609836Z",
  "paid_at": null,
  "cancelled_at": null,
  "requires_review": false
}
```

`amount` se acepta como número o como string (`"125000.50"`); el dashboard envía string para que el valor no pase nunca por un float en el navegador.

| Código | Cuándo |
|---|---|
| `400` | JSON malformado, campo desconocido, cuerpo vacío |
| `422` | `business_not_found`, `invalid_amount`, `amount_scale_exceeded`, `invalid_expires_at`, `invalid_external_reference` |
| `409` | `duplicate_external_reference` |

### `GET /payment-references`

Filtros opcionales: `business_id` (alias `business`), `status`, `external_reference` (coincidencia exacta). Paginación con `limit` (por defecto 20, máximo 100) y `offset`. Orden por `created_at` descendente.

```bash
curl 'localhost:8080/payment-references?status=paid&business_id=1&limit=20'
```

```json
{
  "items": [ { "id": 1, "external_reference": "INV-10023", "status": "paid", "...": "..." } ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

`total` ignora la paginación, que es lo que el frontend necesita para paginar. También cuando la página sale vacía: un `offset` pasado del final devuelve `items: []` con el total real, para que un cliente pueda distinguir «no hay coincidencias» de «pediste una página que no existe». El filtro `status` se aplica al **estado efectivo**: `?status=expired` devuelve también las referencias que siguen guardadas como `pending` pero cuya fecha ya pasó.

### `GET /payment-references/{id}`

`200` con el recurso, `404 payment_reference_not_found` si no existe, `400 invalid_id` si el id no es un entero positivo.

### `POST /payment-references/{id}/cancel`

```bash
curl -X POST localhost:8080/payment-references/2/cancel
```

`200` con la referencia cancelada. Cancelar una ya cancelada devuelve `200` sin cambiar nada (es idempotente, ver supuestos).

| Código | Cuándo |
|---|---|
| `409` | `cannot_cancel_paid_reference`, `cannot_cancel_expired_reference` |
| `404` | no existe |

### `POST /webhooks/payments`

```bash
curl -X POST localhost:8080/webhooks/payments \
  -H 'Content-Type: application/json' \
  -d '{
    "event_id": "evt_983829",
    "external_reference": "INV-10023",
    "amount": 125000.50,
    "transaction_id": "tx_883721",
    "paid_at": "2026-09-28T20:15:00Z"
  }'
```

```json
{
  "event_id": "evt_983829",
  "outcome": "applied",
  "applied": true,
  "duplicate": false,
  "payment_reference_id": 1,
  "reference_status": "paid",
  "message": "payment applied, the reference is now paid"
}
```

**El código de estado responde a una sola pregunta: ¿registramos la entrega de forma durable?**

- `200` cuando sí, **incluso si el pago se rechazó por negocio**. El proveedor reintenta ante cualquier cosa que no sea 2xx, y esos rechazos son deterministas: reintentarlos produciría la misma respuesta para siempre. Eso necesita un humano, no otra entrega. El rechazo queda persistido en `webhook_events` y la referencia marcada para revisión. El campo `outcome` dice qué pasó.
- `404` en un único caso: **no existe esa `external_reference`**. Es el único rechazo que un reenvío puede arreglar, porque la referencia puede crearse un segundo después — el proveedor confirma el pago antes de que la empresa registre el cobro más a menudo de lo que uno querría. Responder 2xx aquí le diría al proveedor que dejara de reintentar un pago que todavía podemos aplicar. La entrega queda registrada igual, y **su resultado deja de ser definitivo**: si el mismo `event_id` vuelve cuando la referencia ya existe, se aplica. Es el único `outcome` que no es terminal.
- `400` si el payload es inservible, `401` si la firma no verifica.
- `500` si no logramos registrarlo, que es justo cuando queremos el reintento.

| `outcome` | Significado |
|---|---|
| `applied` | La referencia pasó a `paid` |
| `duplicate` | Ese `event_id` ya se procesó, o el mismo pago llegó con otro `event_id` |
| `paid_after_expiration` | Se aplicó, pero la referencia ya estaba vencida; marcada para revisión |
| `amount_mismatch` | El monto no coincide; **no** se aplica, marcada para revisión |
| `reference_not_found` | No existe esa `external_reference`. Se registra igual y se responde `404`; si el mismo `event_id` se reenvía cuando la referencia ya existe, se aplica |
| `conflict_cancelled` | La referencia estaba cancelada; no se aplica |
| `conflict_other_transaction` | Ya estaba pagada con otro `transaction_id`; posible doble cobro |

**Firma HMAC (opcional).** Con `WEBHOOK_SECRET` vacío no se verifica nada, que es lo que permite los `curl` de arriba. Con secreto:

```bash
BODY='{"event_id":"evt_1","external_reference":"INV-10023","amount":125000.50,"transaction_id":"tx_1","paid_at":"2026-09-28T20:15:00Z"}'
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -hex | sed 's/^.*= /sha256=/')
curl -X POST localhost:8080/webhooks/payments \
  -H 'Content-Type: application/json' -H "X-Signature: $SIG" -d "$BODY"
```

### `GET /businesses`

Lista las empresas para el selector del formulario. No paginado: hay una fila por empresa cliente.

### `GET /health`

`200` si el proceso puede servir tráfico, `503` si no alcanza la base de datos.

### Verlo funcionando

```bash
./scripts/webhook-demo.sh     # o: make webhook-demo
```

Crea una referencia, la paga por webhook, **reenvía el mismo `event_id`** (y demuestra que no se aplica dos veces), prueba una transacción distinta sobre una referencia ya pagada y un monto que no coincide, y termina enseñando las filas de `webhook_events`.

---

## Arquitectura

### Backend: capas con dependencias hacia dentro

```
backend/
├── cmd/api/main.go                    # único lugar con wiring: config → pool → repos → casos de uso → router
├── internal/
│   ├── domain/                        # ⬅ no importa nada: ni HTTP, ni SQL, ni JSON
│   │   ├── errs/                      # vocabulario de errores tipados (Kind + code estable)
│   │   ├── money/                     # Money exacto sobre decimal; nunca float
│   │   ├── business/                  # entidad Business
│   │   └── paymentref/                # entidad, máquina de estados, evento de webhook, errores
│   ├── application/                   # casos de uso + puertos (interfaces) que necesitan
│   │   ├── paymentref/                # create, get, list, cancel, register_payment
│   │   └── business/                  # list
│   ├── infrastructure/                # implementaciones de esos puertos
│   │   ├── postgres/                  # repositorios, TxManager, traducción de errores del driver
│   │   ├── inmemory/                  # los mismos puertos en memoria, para tests
│   │   └── clock/                     # reloj real y reloj fijo
│   ├── transport/http/                # handlers, DTOs, middlewares, mapeo de errores
│   ├── config/                        # variables de entorno → struct tipado
│   └── arch/                          # test que verifica automáticamente las reglas de abajo
├── migrations/                        # SQL numerado, up/down
└── seeds/                             # empresas de ejemplo
```

**La regla:** `domain` no importa nada. `application` importa solo `domain`. `infrastructure` y `transport` importan `application` y `domain`. Nadie importa `transport`.

No es una convención escrita: `internal/arch/arch_test.go` la comprueba con `go list -deps` sobre el cierre transitivo de cada capa y falla la suite si alguien importa pgx desde el dominio o un repositorio desde un handler.

### Frontend: atomic design para la UI, features para la lógica

```
frontend/src/
├── app/                               # rutas; páginas delgadas que componen templates
│   └── payment-references/{page,new,[id]}
├── components/
│   ├── atoms/                         # Button, Input, Tag, BlueprintFrame… envoltorios tipados del design system
│   ├── molecules/                     # FormField, StatusBadge, Pagination, ConfirmAction…
│   ├── organisms/                     # ReferencesTable, ReferenceForm, ReferenceDetailCard…
│   └── templates/                     # DashboardLayout, ListTemplate, DetailTemplate, DialogTemplate
├── features/payment-references/       # api, actions, hooks, types, schemas, status
├── lib/                               # http-client, format, config
└── styles/                            # tokens.css + design-system.css
```

**La regla:** atoms y molecules son presentacionales (solo props). Los organisms reciben datos y callbacks; nunca hacen fetch. Los datos los trae la página como Server Component, y las mutaciones pasan por Server Actions. De 103 archivos, **solo 11 llevan `'use client'`**.

### Cómo se añadiría un dominio nuevo, por ejemplo "clientes"

Sin tocar una línea del código existente:

**Backend**

1. `internal/domain/customer/` — entidad `Customer`, sus invariantes y sus errores con `errs.Validation(...)` / `errs.NotFound(...)`. No importa nada.
2. `internal/application/customer/` — `ports.go` declara `Repository` con lo que los casos de uso necesiten, y un archivo por caso de uso.
3. `internal/infrastructure/postgres/customer_repository.go` — implementa ese puerto y traduce los errores del driver a errores de dominio. `var _ appcustomer.Repository = (*CustomerRepository)(nil)` lo verifica en compilación.
4. `internal/transport/http/dto/customer.go` y `handler/customer.go` — DTOs y handlers. **El mapeo de errores a HTTP no se toca**: el nuevo dominio devuelve errores con los `Kind` que ya existen y `respond.Error` los traduce.
5. `migrations/000005_create_customers.up.sql`.
6. En `router.go`, un `router.Route("/customers", …)`; en `main.go`, tres líneas de wiring.

Lo único que se modifica son los dos puntos de composición (`router.go` y `main.go`), que existen precisamente para eso.

Si "clientes" necesitara relacionarse con las referencias de pago, el caso de uso de referencias declararía un puerto estrecho — como ya hace `BusinessFinder`, que solo tiene `ExistsByID` — en lugar de importar el paquete del otro dominio. Así los dos dominios siguen sin conocerse.

**Frontend**

1. `features/customers/` — `types.ts`, `api.ts`, `schemas.ts`, `actions.ts`, `hooks.ts`.
2. `components/organisms/CustomersTable/` — recibe filas y callbacks.
3. `app/customers/page.tsx` — compone `ListTemplate` con el organism nuevo.

Atoms, molecules y templates se reutilizan tal cual: `ListTemplate`, `Pagination`, `EmptyState`, `FormField` y los átomos no saben nada de referencias de pago.

---

## Decisiones técnicas

**`NUMERIC(14,2)` y decimal exacto, nunca float.** `125000.50` no es representable en binario. Con float, la comparación del webhook entre el monto notificado y el esperado sería una lotería y las sumas derivarían. El valor viaja como string o número JSON, se parsea desde los bytes crudos con `decimal` (nunca pasa por `float64` en Go) y se compara con igualdad exacta. En el frontend solo se formatea; nunca se opera.

**La idempotencia la impone un índice único, no el código.** `webhook_events.event_id` es `UNIQUE` y el `INSERT` es lo primero que ocurre en la transacción. No hay `SELECT` previo: eso dejaría una ventana donde dos entregas concurrentes pasan la comprobación. Se usa `ON CONFLICT DO NOTHING` en vez de dejar salir la violación 23505 porque una 23505 cruda deja la transacción de PostgreSQL abortada, y el caso de uso necesita leer el resultado original para replicarlo.

**Todo el webhook en una transacción**, con `SELECT … FOR UPDATE` sobre la referencia. Un evento no puede quedar registrado como aplicado sin que la referencia se haya movido, ni al revés. Hay tests de concurrencia contra PostgreSQL real: 8 entregas simultáneas del mismo `event_id` aplican exactamente una vez, y un pago compitiendo con una cancelación nunca deja una fila que parezca pagada y cancelada a la vez.

**Concurrencia optimista sin columna de versión.** Los `UPDATE` comparan el estado esperado dentro de la propia sentencia (`WHERE id = $1 AND status = $2`). Si otra transacción movió la fila, cero filas coinciden y el llamador recibe `concurrent_update`.

**La expiración se deriva en lectura y se materializa al escribir.** Una referencia `pending` cuya fecha pasó se reporta como `expired` desde el primer momento, sin depender de ningún job. Cuando `cancel` toma el lock y ve que está vencida, persiste la expiración y devuelve el 409 correcto, así el dato converge en vez de derivar.

**400 y 422 separados.** 400 es «no te entiendo» (JSON roto, id no numérico, `status=refunded`); 422 es «te entiendo pero no lo acepto» (monto no positivo, fecha pasada, empresa inexistente). Un cliente puede distinguir su bug de su dato.

**Errores tipados con un `code` estable, mapeados a HTTP en un solo sitio.** `respond.Error` es la única traducción de dominio a HTTP del código. Añadir un endpoint no puede inventar un formato de error distinto.

**chi en vez de `net/http` puro.** Por el middleware por grupo: la verificación de firma se aplica a `/webhooks` entero, así que no se puede olvidar en una ruta. Los handlers siguen siendo `http.HandlerFunc`, de modo que sustituirlo tocaría un archivo.

**Sin ORM.** Las consultas que importan aquí (`UPDATE` condicional, `FOR UPDATE`, `ON CONFLICT`, ventana para el total) son exactamente las que un ORM esconde.

**Migraciones como servicio aparte.** La API no necesita permisos de DDL y dos réplicas no pueden migrar a la vez.

**Server Components y Server Actions en el frontend.** Los filtros viven en la URL, así que una vista filtrada es un enlace que se puede compartir y no hay caché de cliente que invalidar. El navegador no habla con el API: la URL del API se queda en el servidor. Sin librería de estado global.

**El loading state vive dentro de la página, no en un `loading.tsx`.** Un boundary de ruta envía la cabecera con 200 antes de que la página resuelva, lo que deja `notFound()` sin poder responder 404. Con `<Suspense>` dentro de la página, la cabecera se pinta al instante, la tabla tiene esqueleto y los códigos siguen siendo honestos.

**El design system se portó como CSS global, no como componentes.** Es CSS plano sobre HTML plano por diseño; sus clases son el contrato. Los atoms son envoltorios tipados sobre ellas y los CSS Modules solo aportan la maquetación que falta.

---

## Supuestos

El enunciado deja varios puntos abiertos. Esto es lo que se decidió y por qué.

| Ambigüedad | Decisión |
|---|---|
| **El webhook solo trae `external_reference`, sin `business_id`. ¿Es única globalmente?** | **Sí, `UNIQUE` global.** Es la única forma de resolver el payload de forma determinista. Si el proveedor enviara también la empresa, se relajaría a `UNIQUE (business_id, external_reference)`. |
| **¿Y si el monto del webhook no coincide?** | **No se marca como pagada.** Se registra el evento con `amount_mismatch`, se marca la referencia para revisión y se responde `200`. Un monto menor **no** se interpreta como pago parcial: los pagos parciales están fuera de alcance. |
| **¿Y si llega un pago para una referencia vencida?** | **Se marca como pagada** y se marca para revisión (`paid_after_expiration`). El dinero entró; rechazarlo dejaría a la empresa con un cobro recibido y sin registro. Un humano decide si honrarlo o devolverlo. |
| **¿Y si llega para una cancelada?** | **Se mantiene cancelada.** Cancelar es una decisión explícita de negocio; aplicarlo en silencio la contradiría. Se registra `conflict_cancelled` y se marca para revisión. |
| **¿Y si el webhook llega antes de que exista la referencia?** | Se registra la entrega y se responde **`404`**, no `200`. Es el único rechazo que un reenvío puede arreglar, así que es el único que debe hacer reintentar al proveedor; con `200` el `event_id` quedaba quemado y el pago se perdía. El resultado `reference_not_found` es el único no terminal: el reenvío se reevalúa contra el estado actual. |
| **¿Cómo se crean las empresas?** | **Seed**, porque el enunciado no define endpoint. Se añadió `GET /businesses` porque el selector del formulario lo necesita. |
| **¿Se autentica el webhook?** | **HMAC-SHA256 opcional** sobre el cuerpo crudo, cabecera `X-Signature`, comparación en tiempo constante. **Desactivado si `WEBHOOK_SECRET` está vacío**, para que se pueda probar con `curl`. En producción sería obligatorio. Se descartó una cabecera `X-Timestamp`: sin incluirla en la firma no aporta nada, e incluirla sería inventar un esquema que el proveedor no ha acordado. La protección contra replay la da la idempotencia. |
| **¿Cancelar una ya cancelada es idempotente o conflicto?** | **Idempotente, `200`.** Cancelar expresa un estado final deseado; el reintento de un cliente con red inestable no debería fallar. Pagada sigue siendo `409`, como pide el enunciado. |
| **Mismo pago con otro `event_id`** | Si el `transaction_id` coincide, `duplicate` y no se cambia nada. Si es distinto, posible doble cobro: `conflict_other_transaction`, se marca para revisión y **no se sobrescribe** el primer pago. |
| **Moneda** | No se menciona: se asume una sola (COP por las magnitudes del enunciado) y no se modela. |
| **`paid_at` del proveedor en el futuro** | Se confía como fecha de negocio del pago y no se valida contra `now()`. Las fechas del enunciado son de 2026. |
| **Paginación** | No se menciona; se añadió (`limit` 20 por defecto, máximo 100). Un listado sin cota no sobrevive a datos reales. |
| **Autenticación del dashboard** | Fuera de alcance: un operador interno único. En real, un middleware inyectaría el `business_id` autenticado y todos los casos de uso lo recibirían. |
| **Zona horaria** | Todo `TIMESTAMPTZ` en UTC, RFC3339 en la API, y la UI lo dice explícitamente en cada fecha. |
| **Búsqueda por `external_reference`** | **Coincidencia exacta**, que es lo que hace el backend. El diseño insinuaba búsqueda parcial; el placeholder del buscador dice «exacta» para no prometer lo que no hay. |
| **Tarjetas de estadísticas del diseño** | El diseño mostraba sumas de dinero. La API no tiene endpoint de agregados y sumar la página actual daría una cifra falsa, así que se muestran **conteos**, que sí son exactos (una llamada por estado con `limit=1` leyendo `total`). |
| **`event_id` y enlace de pago en el detalle** | El diseño los mostraba. `event_id` vive en `webhook_events`, no en la referencia, así que se omite. El enlace de pago no existe en el dominio: se construye desde `NEXT_PUBLIC_PAYMENT_LINK_BASE` y está documentado como maqueta. |
| **Modales del diseño** | El diseño resolvía todo en una pantalla (modal + drawer). Aquí son rutas propias (`/new`, `/[id]`) para que sean enlazables; se conserva la gramática visual pero sin overlay oscuro, porque detrás no hay nada. |

---

## Pendientes y cómo los abordaría

**Patrón outbox o cola para los webhooks.** Hoy el webhook se procesa de forma síncrona dentro de la petición del proveedor. Funciona y es correcto, pero acopla nuestra latencia a la suya y no deja sitio para efectos posteriores (notificar a la empresa, emitir una factura). Lo abordaría en dos pasos: primero, que el handler solo persista el evento y responda `202`, y que un worker consuma `webhook_events` con `FOR UPDATE SKIP LOCKED` — la tabla ya existe y ya tiene el `event_id` único, así que es un cambio de quién procesa, no de qué se guarda. Segundo, una tabla `outbox` escrita en la **misma transacción** que el cambio de estado, para que publicar «referencia pagada» sea atómico con el pago; un relay la lee y publica. Sin eso, cualquier publicación a un bus tiene el problema clásico de dos escrituras que no se pueden confirmar juntas.

**Autenticación de empresas.** Hoy no hay ninguna y el dashboard asume un operador interno. Añadiría autenticación por API key por empresa (hash en base, comparación en tiempo constante) más un middleware que resuelva el `business_id` y lo ponga en el contexto. La parte importante es que **los casos de uso reciban ese `business_id` como parámetro obligatorio** y filtren por él en el repositorio, en vez de confiar en que el handler lo haya validado: así un endpoint nuevo no puede olvidarse de aislar el tenant. El punto de extensión ya está marcado en las firmas.

**Job de expiración.** La expiración ya se deriva en lectura, así que nada muestra un estado incorrecto, pero el dato almacenado solo converge cuando alguien cancela. Falta el proceso periódico: `application/paymentref.ExpireDue` y `Repository.ExpireDue` ya están implementados y probados (con lotes acotados y `SKIP LOCKED`); falta el `time.Ticker` que los llame, tomando `pg_try_advisory_lock` para que varias réplicas no hagan el mismo trabajo. Lo pondría en `cmd/worker` desde el principio, no dentro de la API, para poder escalarlos por separado.

**Observabilidad.** Hoy hay logging estructurado con `slog` y un `X-Request-ID` que correlaciona. Falta lo que se necesita para operar: métricas Prometheus (latencia y códigos por endpoint, `outcome` del webhook por tipo, antigüedad de la referencia pendiente más vieja, filas con `requires_review`), trazas OpenTelemetry propagando el contexto desde el middleware hasta el repositorio, y alertas sobre lo que realmente importa — que crezcan los `amount_mismatch` o los `conflict_other_transaction`, no que suba la CPU.

**Tests E2E.** La cobertura actual llega hasta los handlers con dobles en memoria y hasta PostgreSQL real en integración, pero nada ejercita el navegador. Añadiría Playwright contra el stack de `docker compose`: crear una referencia desde el formulario, comprobar que aparece en el listado, dispararle el webhook por HTTP y verificar que el detalle pasa a «Pagada» sin recargar, y que cancelar una pagada muestra el 409. Son cuatro o cinco recorridos; el valor está en que cruzan las tres capas del sistema a la vez.

**Otros, por orden de importancia:** rate limiting en el webhook; una UI de conciliación para las referencias con `requires_review` (hoy se ven marcadas pero no hay flujo para resolverlas); paginación por cursor cuando el `OFFSET` empiece a doler; y pagos parciales, que el enunciado menciona como dominio futuro y que cambiarían el modelo (una referencia pasaría a tener varios pagos y un saldo).

---
