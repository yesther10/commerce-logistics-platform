# Guía de decisiones y aprendizaje

Esta guía explica desde cero cómo está construido `Commerce Logistics Platform`, qué decisiones de arquitectura se tomaron y qué límites tiene la implementación actual. Está dirigida a una persona que está aprendiendo microservicios; por eso cada concepto aparece primero con una intuición y después con su aplicación concreta en este repositorio.

La guía describe el estado real del código. No presenta como implementado aquello que está reservado para una etapa posterior.

## Ruta rápida

Para obtener una visión útil sin leer todo de una vez:

1. Leer [la arquitectura inicial](architecture.md) para conocer los límites de los servicios.
2. Leer [el contrato de eventos](events.md) para entender la integración con RabbitMQ.
3. Levantar la infraestructura local siguiendo [`infrastructure/docker/README.md`](../infrastructure/docker/README.md).
4. Leer la sección [Flujo completo](#8-contrato-de-eventos-y-flujo-completo) de esta guía.
5. Practicar con la [ruta de aprendizaje](#13-cómo-leer-el-repositorio-y-aprender-por-etapas).

## 1. El problema que resuelve el proyecto

### Intuición

Una tienda en línea no solo necesita registrar un pedido. Después de crear un pedido normalmente debe:

1. aprobar o rechazar el pago;
2. solicitar el envío;
3. crear el envío y seguir su estado;
4. avisar al cliente que el envío fue creado.

En un sistema pequeño, todo esto podría vivir en una sola aplicación. Este proyecto usa el mismo problema para estudiar qué sucede cuando cada capacidad pertenece a un servicio independiente y los servicios deben coordinarse sin compartir sus bases de datos.

```text
Cliente
  |
  | HTTP directo al servicio que posee la capacidad
  v
commerce-laravel
  |  OrderCreated
  |  PaymentApproved
  |  ShipmentRequested
  v
RabbitMQ (broker de eventos)
  |
  v
logistics-go
  |  ShipmentCreated
  v
RabbitMQ
  |
  v
notifications-nestjs
  |  NotificationRequested
  |  entrega simulada
```

El proyecto no intenta ser una tienda completa. Su objetivo educativo es hacer visibles los límites, el ownership de datos y los fallos normales de una integración distribuida.

### Conceptos fundamentales

#### Monolito

**Intuición:** un monolito es como una sola oficina que atiende ventas, pagos, envíos y notificaciones. Es sencillo de iniciar y coordinar, pero sus partes comparten el mismo proceso y normalmente la misma base de datos.

**Detalle técnico:** un monolito es una aplicación desplegada como una unidad principal. Puede tener módulos internos muy bien separados; “monolito” no significa necesariamente “código desordenado”. La propiedad importante es que las capacidades se ejecutan y se despliegan juntas.

Este repositorio no implementa un monolito: tiene tres servicios desplegables de forma independiente y una infraestructura compartida solo para sus dependencias locales.

#### Microservicio

**Intuición:** un microservicio es una oficina pequeña que tiene una responsabilidad clara y puede operar sin convertirse en una modificación obligatoria de las otras oficinas.

**Detalle técnico:** un microservicio es un proceso o aplicación autónoma con una frontera de negocio, un ciclo de despliegue y, normalmente, un almacenamiento que controla. Tener tres carpetas no basta: la frontera se vuelve real cuando los servicios no escriben las tablas de los demás y se integran mediante contratos explícitos.

En este proyecto, `commerce-laravel`, `logistics-go` y `notifications-nestjs` son las tres fronteras. Compose puede construir sus contenedores mediante el perfil `app`, aunque los procesos también pueden ejecutarse desde el host o desde entornos de desarrollo separados.

#### Bounded context (contexto delimitado)

**Intuición:** la palabra “pedido” puede significar cosas distintas para comercio y logística. Para comercio es un agregado con productos, cantidades y pago; para logística es una referencia que permite crear un envío.

**Detalle técnico:** un bounded context es el límite dentro del cual un modelo, un vocabulario y unas reglas tienen un significado coherente. Cada servicio de este proyecto modela su propio contexto:

| Contexto | Pregunta que responde | Datos propios |
| --- | --- | --- |
| Comercio | ¿Qué compró el cliente y cuál es el estado del pedido? | productos, clientes, carritos, pedidos, ítems y pago simulado |
| Logística | ¿Qué envío existe y cuál es su estado logístico? | envíos y estado de seguimiento |
| Notificaciones | ¿Qué aviso debe gestionarse y cuál es su estado de entrega? | solicitudes y entrega simulada |

Un `order_id` enviado a logística es una referencia, no convierte al pedido en propiedad de logística.

#### Ownership de datos

**Intuición:** quien es responsable de una hoja de datos también es quien decide sus reglas y cambios. Los demás pueden pedir una transición, pero no editar esa hoja directamente.

**Detalle técnico:** ownership significa que un servicio es la autoridad para crear, modificar y leer sus datos de dominio. En este proyecto cada servicio tiene su PostgreSQL independiente. Ningún servicio lee tablas ajenas ni hace joins entre bases.

La información mínima que viaja en un evento permite procesar una transición, pero no crea una copia completa del agregado remoto.

#### Comunicación síncrona

**Intuición:** una llamada síncrona es una conversación telefónica: quien llama espera una respuesta antes de continuar.

**Detalle técnico:** en una interacción HTTP síncrona, el cliente envía una solicitud y mantiene abierta la operación hasta recibir una respuesta o un error. Aquí se usa para que un cliente llame al servicio que posee una capacidad y para el trabajo interno de una transacción.

El flujo entre servicios no depende de una llamada HTTP síncrona de comercio a logística ni de logística a notificaciones.

#### Comunicación asíncrona

**Intuición:** una comunicación asíncrona es dejar un mensaje en una bandeja. El emisor puede continuar; el receptor lo procesará cuando esté disponible.

**Detalle técnico:** el productor publica un mensaje y el consumidor lo procesa en otro momento. El emisor no obtiene una respuesta de negocio inmediata del consumidor. En este proyecto RabbitMQ transporta los eventos del ciclo de pedido.

Esto desacopla la disponibilidad de logística y notificaciones del request HTTP que crea un pedido.

#### Evento

**Intuición:** un evento dice “algo que ya ocurrió”. No dice “ejecute una función privada de mi código”.

**Detalle técnico:** un evento es un mensaje de integración que describe una transición de negocio ya confirmada dentro del servicio productor. Por ejemplo, `OrderCreated` significa que comercio ya creó el pedido en su base de datos.

Los eventos tienen una envoltura común (`envelope`), versión, identificadores de trazabilidad y un `payload` específico. El productor es el servicio que publica el evento y el consumidor es el servicio o handler que lo procesa; el productor es dueño del significado del evento.

#### Cola

**Intuición:** una cola es una fila de mensajes pendientes para un consumidor.

**Detalle técnico:** una cola conserva mensajes hasta que un consumidor los procesa y confirma. Una cola permite que el productor y el consumidor tengan ritmos diferentes. En la implementación existen colas durables como `commerce.order.created`, `logistics.shipment.requested` y las colas de notificaciones.

#### Broker

**Intuición:** un broker es el centro postal: recibe mensajes, decide a qué bandejas entregarlos y los conserva según sus reglas.

**Detalle técnico:** un broker de mensajería acepta publicaciones, aplica routing y gestiona colas, confirmaciones y entregas. RabbitMQ es el broker de este proyecto. El exchange `platform.events` es de tipo `topic`; las routing keys conectan cada evento con las colas interesadas.

#### Consistencia eventual

**Intuición:** después de crear un pedido, la base de comercio puede mostrar “pedido creado” mientras logística todavía no muestra un envío. Ambas vistas convergerán cuando los eventos pendientes se procesen.

**Detalle técnico:** la consistencia eventual acepta que los datos de distintos servicios no se actualicen en el mismo instante. No hay una transacción distribuida que confirme las tres bases juntas. Cada servicio confirma su propia transacción y el siguiente cambio llega mediante un evento.

El sistema debe diseñarse para estados intermedios y para observar los eventos pendientes; no debe asumir que todas las bases cambian al mismo tiempo.

#### Idempotencia

**Intuición:** repetir la misma instrucción no debe crear un segundo envío ni entregar dos veces la misma transición.

**Detalle técnico:** una operación es idempotente cuando repetirla con la misma identidad produce el mismo efecto final que ejecutarla una vez. RabbitMQ trabaja con entrega *at-least-once* (al menos una vez), por lo que una entrega duplicada es posible.

Los consumidores guardan el `event_id` en `processed_events` dentro de la misma frontera de durabilidad que el cambio de negocio. Si ya existe, omiten el trabajo y confirman el mensaje duplicado.

#### Outbox

**Intuición:** el outbox es una bandeja de salida dentro de la misma oficina. Primero se registra el cambio de negocio y el mensaje pendiente en una sola operación local; después otro proceso lo entrega al broker.

**Detalle técnico:** el patrón outbox escribe el estado de negocio y el evento en la misma transacción de la base del productor. Un publicador consulta los eventos pendientes, los publica y los marca como publicados solo después de una confirmación de RabbitMQ.

Así se evita el fallo clásico “la base confirmó el pedido, pero el proceso se cayó antes de publicar el evento”. El evento queda pendiente para un intento posterior.

#### Inbox

**Intuición:** el inbox es la lista de mensajes que una oficina ya procesó. Sirve para que una repetición no se convierta en trabajo duplicado.

**Detalle técnico:** el patrón inbox persiste la identidad del mensaje entrante antes o junto con el cambio de negocio, dentro de la transacción local. En este proyecto no hay una tabla llamada `inbox`: la tabla `processed_events` cumple esa responsabilidad en cada servicio consumidor.

La clave de deduplicación es `event_id`, no `order_id`, porque un pedido puede producir varios eventos legítimos.

#### ACK manual

**Intuición:** un ACK es decirle al broker “ya terminé con este mensaje”. El ACK manual impide decirlo antes de tiempo.

**Detalle técnico:** los consumidores usan `noAck: false` o el equivalente de la biblioteca. Solo confirman el mensaje después de que la transacción local termina con éxito. Para un duplicado conocido también se envía ACK, porque el trabajo ya había sido completado.

En el código los consumidores tienen además `prefetch=1`, por lo que reciben como máximo un mensaje no confirmado a la vez.

#### Publisher confirm

**Intuición:** publicar un mensaje no es lo mismo que saber que RabbitMQ lo aceptó. La confirmación es el recibo del broker.

**Detalle técnico:** un canal de confirmación permite que el productor espere el acuse de RabbitMQ antes de marcar el registro outbox como publicado. Si el proceso se cae después del confirm y antes de actualizar la base, puede repetir el mismo `event_id`; esa ventana se acepta y se compensa con consumidores idempotentes.

#### DLQ

**Intuición:** una dead-letter queue (DLQ, cola de mensajes muertos) es un área de cuarentena para mensajes que no se pudieron procesar.

**Detalle técnico:** el consumidor rechaza un mensaje sin reencolarlo y RabbitMQ lo enruta mediante `platform.events.dlx` a la DLQ específica del consumidor. La DLQ conserva el envelope y los metadatos necesarios para inspección o replay controlado.

La DLQ no significa que el mensaje se haya borrado ni que ya exista una recuperación automática.

#### Healthcheck

**Intuición:** un healthcheck es una pregunta automática: “¿este proceso o dependencia está suficientemente disponible para considerarse saludable?”.

**Detalle técnico:** Docker Compose ejecuta `rabbitmq-diagnostics -q ping` para RabbitMQ y `pg_isready` para cada PostgreSQL. Las APIs exponen `/up` en Laravel y `/healthz` en Go y NestJS. Un healthcheck informa disponibilidad técnica; no demuestra que el flujo completo de negocio haya funcionado.

## 2. Alcance elegido

### Decisión

El alcance inicial es un e-commerce con logística integrada, reducido al ciclo de pedido que permite demostrar límites de servicios y mensajería confiable.

### Qué entra

| Capacidad | Implementación actual |
| --- | --- |
| Catálogo básico | `commerce-laravel` crea y lista productos. |
| Pedido | Comercio crea pedidos, ítems y total a partir de precios almacenados en el servidor. |
| Pago | Transición simulada a `payment_approved`; no existe proveedor real. |
| Solicitud de envío | Comercio publica `ShipmentRequested` solo para un pedido aprobado. |
| Envío | `logistics-go` crea el registro con estado `created`. |
| Notificación | `notifications-nestjs` crea la solicitud, publica `NotificationRequested` y simula el estado `delivered`. |
| Integración | RabbitMQ, outboxes locales, deduplicación por `event_id`, ACK manual, confirms y DLQ durable. |
| Infraestructura local | Un broker RabbitMQ, un PostgreSQL independiente por servicio y un perfil opcional de aplicaciones mediante Docker Compose. |

### Qué queda fuera

| Fuera de alcance | Motivo de la exclusión |
| --- | --- |
| API Gateway | Solo hay tres servicios y todavía se están aprendiendo los límites; agregarlo ahora ocultaría el ownership y sumaría routing, autenticación y observabilidad de borde. |
| Proveedor o servicio de pagos | `PaymentApproved` es una simulación de comercio, no una autorización externa. |
| Carrier, tarifas, rutas y booking | `ShipmentCreated` solo representa el registro local del envío. |
| Proveedor real de email, SMS o push | La entrega de notificaciones es simulada. |
| Base compartida o librería de dominio compartida | Rompería el ejercicio de ownership independiente. |
| HTTP entre servicios para este ciclo | RabbitMQ es la frontera de integración elegida. |
| Retries delayed y reconexión automática | Son trabajo futuro; la limitación actual es explícita y se documenta en [Retries y DLQ](#10-retries-y-dlq-la-limitación-actual). |
| Contenedores de aplicación | El perfil `app` construye y separa APIs, publishers, consumers, workers y el migrador Go; el modo predeterminado conserva solo las dependencias. |
| Autenticación y credenciales de producción | No forman parte de la primera etapa educativa. |

El alcance pequeño es una decisión de aprendizaje: permite observar una cadena completa sin inventar dominios que todavía no tienen reglas ni integraciones reales.

## 3. Mapa de servicios y responsabilidades

```text
                       +----------------------+
HTTP directo --------> | commerce-laravel     |
                       | Laravel / PHP        |
                       | PostgreSQL commerce  |
                       +----------+-----------+
                                  |
                    OrderCreated  | PaymentApproved
                    ShipmentRequested
                                  v
                       +----------------------+
                       | RabbitMQ              |
                       | platform.events       |
                       +----------+-----------+
                                  |
                                  v
                       +----------------------+
                       | logistics-go          |
                       | Go / net/http         |
                       | PostgreSQL logistics  |
                       +----------+-----------+
                                  |
                           ShipmentCreated
                                  v
                       +----------------------+
                       | notifications-nestjs  |
                       | NestJS / Node.js      |
                       | PostgreSQL notif.     |
                       +----------------------+
```

### Resumen

| Servicio | Posee | No posee | Tecnología y frontera |
| --- | --- | --- | --- |
| `commerce-laravel` | productos, clientes, carritos, pedidos, ítems y estado de pago simulado | envíos y solicitudes de notificación | Laravel 13 / PHP; API HTTP, transacciones y comandos Artisan |
| `logistics-go` | envíos, planificación inicial y estado de seguimiento | pedidos, clientes, productos y pagos | Go; `net/http`, dominio separado, `database/sql` con PostgreSQL y worker AMQP |
| `notifications-nestjs` | solicitudes de notificación y estado de entrega simulada | clientes, pedidos, envíos y pagos | NestJS / Node.js; módulos, inyección de dependencias, TypeORM y worker AMQP |

### `commerce-laravel`

**Responsabilidad exacta:** es la autoridad del contexto comercial. Expone:

- `POST /api/v1/products`;
- `GET /api/v1/products`;
- `POST /api/v1/orders`;
- `GET /api/v1/orders/{order}`.

La creación del pedido bloquea los productos seleccionados, calcula los importes en centavos enteros usando precios del servidor y persiste pedido e ítems en una transacción local. En esa misma transacción registra `OrderCreated` en `outbox_events`.

Su consumidor `php artisan events:consume-order-created --once` procesa `OrderCreated`, registra la deduplicación, cambia el pedido a `payment_approved` y escribe `PaymentApproved` y `ShipmentRequested` en el outbox. El pago es una transición simulada dentro del propio servicio; no se debe interpretar como autorización real.

**Por qué Laravel/PHP:** el repositorio usa Laravel para mostrar una API de negocio convencional con validación, Form Requests, Resources, ORM y transacciones. La asignación también permite comparar un framework completo con las implementaciones más explícitas de Go y NestJS.

**Alternativa descartada o posterior:** no se creó un `payment-service` ni se integró un PSP. Una integración real se dejaría para una decisión de dominio posterior, cuando existan estados, compensaciones y seguridad suficientes.

**Costo aceptado:** el servicio concentra inicialmente catálogo, pedidos y pago simulado. Eso es deliberado para evitar crear un cuarto servicio sin una responsabilidad real.

Más detalle en [`services/commerce-laravel/README.md`](../services/commerce-laravel/README.md), [`routes/api.php`](../services/commerce-laravel/routes/api.php) y [`composer.json`](../services/commerce-laravel/composer.json).

### `logistics-go`

**Responsabilidad exacta:** posee el registro de envío y su estado logístico. Expone:

- `GET /healthz`;
- `POST /shipments`;
- `GET /shipments/{id}`.

El endpoint `POST /shipments` valida `order_id`, genera el UUID del envío en este servicio y comienza con estado `created`. El worker consume `ShipmentRequested`, registra `event_id`, crea el envío y el `ShipmentCreated` outbox en una única transacción PostgreSQL.

**Por qué Go:** el trabajo inicial busca una implementación pequeña y explícita. El estándar `net/http` cubre servidor, routing y testing HTTP sin agregar un framework antes de demostrar la frontera. La separación entre handler, dominio y repositorio permite probar HTTP con fakes deterministas.

**Alternativa descartada o posterior:** no se agregó un framework web Go. Puede evaluarse cuando aumenten las necesidades transversales, pero ahora añadiría abstracción sin una necesidad demostrada.

**Costo aceptado:** hay más código explícito para configuración, HTTP, migraciones y mensajería. Ese costo es útil pedagógicamente porque hace visibles las responsabilidades.

Más detalle en [`services/logistics-go/README.md`](../services/logistics-go/README.md) y en [`cmd/worker/main.go`](../services/logistics-go/cmd/worker/main.go).

### `notifications-nestjs`

**Responsabilidad exacta:** posee las solicitudes de notificación y la transición local de `requested` a `delivered` simulada. Expone:

- `GET /healthz`;
- `POST /notifications`;
- `GET /notifications/{notification_id}`.

NestJS procesa `ShipmentCreated`, crea la solicitud local y registra `NotificationRequested` en el outbox. Después consume `NotificationRequested` y cambia la solicitud a `delivered` dentro de una transacción. No resuelve destinatarios ni llama a email, SMS o push.

**Por qué NestJS/Node.js:** NestJS muestra composición por módulos e inyección de dependencias, mientras que Node.js ofrece un runtime natural para HTTP y AMQP. TypeORM queda detrás de contratos de aplicación y los tests pueden reemplazar el almacenamiento por un proveedor en memoria.

**Alternativa descartada o posterior:** no se agregó un framework de mensajería adicional ni un proveedor de notificaciones real. El worker usa `amqplib` directamente para que el contrato AMQP permanezca visible.

**Costo aceptado:** el servicio tiene más piezas de composición que una aplicación HTTP mínima. Ese costo se acepta para estudiar módulos, adaptadores, migraciones y worker en un segundo stack.

Más detalle en [`services/notifications-nestjs/README.md`](../services/notifications-nestjs/README.md) y [`package.json`](../services/notifications-nestjs/package.json).

## 4. Por qué usamos monorepo

### Intuición

Un monorepo es un repositorio que contiene varios servicios relacionados. No significa que los servicios compartan código de dominio o una base de datos.

### Decisión

Este proyecto usa un monorepo con esta forma:

```text
README.md
docs/
infrastructure/docker/
docker-compose.yml
services/
├── commerce-laravel/
├── logistics-go/
└── notifications-nestjs/
```

### Motivo

- El contrato [`docs/events.md`](events.md) puede revisarse junto con los productores y consumidores.
- La infraestructura local, las instrucciones y los tres servicios quedan en una misma vista.
- Un estudiante puede ejecutar una sola configuración Compose y comparar PHP, Go y Node.js.
- Los cambios educativos de una misma rebanada de integración se pueden inspeccionar juntos.
- No se necesita publicar todavía paquetes de contrato ni automatizar repositorios separados.

### Alternativa: multirepo

Con multirepo habría un repositorio para cada servicio y posiblemente otro para infraestructura y contratos. Eso puede ser apropiado para equipos autónomos, permisos distintos, ciclos de release muy diferentes o repositorios de menor tamaño.

### Costo de la decisión

El monorepo puede permitir acoplamiento accidental: un cambio de contrato puede editar todos los servicios en el mismo lugar sin respetar sus ciclos de despliegue. También requiere disciplina para conservar fronteras y pipelines independientes. Si el proyecto creciera, se debería decidir explícitamente si el contrato se publica como artefacto versionado o si se separan los repositorios.

## 5. Por qué no hay API Gateway en la primera etapa

### Intuición

Un API Gateway es una puerta de entrada que puede enrutar, autenticar, limitar tráfico y combinar respuestas de varios servicios. Una puerta puede ser útil, pero construirla antes de saber qué debe proteger o combinar solo agrega otra sala que mantener.

### Decisión

Los clientes llaman directamente al servicio que posee la capacidad. El flujo entre servicios usa RabbitMQ. No hay gateway en Compose ni en las aplicaciones.

### Motivo

- Solo existen tres servicios y sus fronteras todavía son parte del aprendizaje.
- No hay una necesidad acordada de agregación de respuestas.
- Añadiría routing, propagación de autenticación, despliegue y observabilidad de borde.
- El acceso directo hace visible qué servicio es dueño de cada operación.

### Alternativa y costo

La alternativa es introducir un gateway cuando exista una API pública estable, políticas de seguridad centralizadas, rate limiting o una vista compuesta que lo justifique. Mientras tanto, el cliente debe conocer el servicio correcto y no existe un punto único para autenticación o métricas de borde. Ese costo es temporal y aceptado; no se debe esconder un ownership confuso detrás de un gateway.

## 6. Por qué cada servicio tiene PostgreSQL independiente

### Decisión

Compose inicia tres instancias independientes:

| Servicio | Contenedor | Puerto del host | Base inicial |
| --- | --- | ---: | --- |
| Comercio | `postgres-commerce` | `55432` | `commerce_dev` |
| Logística | `postgres-logistics` | `55433` | `logistics_dev` |
| Notificaciones | `postgres-notifications` | `55434` | `notifications_dev` |

### Motivo

- Hace visible quién posee cada dato.
- Impide accidentalmente crear tablas ajenas o hacer joins entre contextos.
- Permite que cada servicio controle sus migraciones y su ciclo de persistencia.
- Obliga a modelar la integración como contrato de eventos, no como acceso directo.

### Qué sí puede cruzar la frontera

Un evento puede transportar `order_id` y, cuando corresponde, `shipment_id`. Esos identificadores son referencias. Logística no crea una tabla de pedidos; notificaciones no crea una tabla de envíos.

### Alternativa y costo

Una base compartida sería más sencilla al principio: una transacción y un join resolverían varias pantallas. Pero produciría acoplamiento de esquemas, escrituras cruzadas y releases coordinados. La separación cuesta más infraestructura local y obliga a manejar consistencia eventual, pero es el ejercicio central del proyecto.

## 7. Por qué RabbitMQ y no Kafka inicialmente

### Intuición

RabbitMQ se parece a una central de colas: es natural para entregar un mensaje a consumidores y confirmar su procesamiento. Kafka se parece más a un registro distribuido particionado: es excelente cuando el historial, el throughput y el replay son parte central del diseño.

### Decisión

Se usa RabbitMQ con:

- exchange durable `platform.events` de tipo `topic`;
- routing keys como `commerce.order.created`;
- colas durables por consumidor;
- ACK manual;
- publisher confirms;
- exchange de dead letters `platform.events.dlx`.

### Motivo

El primer flujo es pequeño y orientado a entregar transiciones a consumidores concretos. RabbitMQ permite estudiar routing, ACK, confirmaciones y DLQ sin introducir desde el inicio el modelo de particiones, offsets y operación de un log distribuido.

### Alternativa: Kafka

Kafka sería interesante si el proyecto necesitara retención prolongada como fuente de eventos, muchos consumidores independientes, throughput elevado, particionamiento explícito o replay frecuente. No se afirma que RabbitMQ sea superior en general: se elige porque encaja mejor con el primer objetivo educativo y con colas de trabajo.

### Costo aceptado

La estrategia actual no ofrece el mismo modelo de historial y replay nativo que un log de Kafka. El replay de esta etapa es controlado desde DLQ y preserva el `event_id`; no existe una plataforma de replay automática.

## 8. Contrato de eventos y flujo completo

El contrato normativo está en [`docs/events.md`](events.md). Lo siguiente lo explica con contexto y con la topología que existe en las aplicaciones.

### Convenciones del broker

| Elemento | Contrato de versión 1 |
| --- | --- |
| Exchange | `platform.events` |
| Tipo | `topic` |
| Event type | Nombre PascalCase en el envelope, por ejemplo `OrderCreated` |
| Routing key | Nombre en minúsculas separado por puntos |
| Versionado | `event_version` entero; empieza en `1` |
| Entrega | At-least-once; puede haber duplicados |
| Orden | No existe orden global; solo se usa la causalidad de la cadena |

Las colas reales se declaran por los workers, no por Docker Compose. En los `.env.example` actuales aparecen:

| Consumidor | Cola | DLQ |
| --- | --- | --- |
| Commerce | `commerce.order.created` | `commerce.order.created.dlq` |
| Logistics | `logistics.shipment.requested` | `logistics.shipment.requested.dlq` |
| Notifications: `ShipmentCreated` | `notifications.logistics.shipment.created` | `notifications.logistics.shipment.created.dlq` |
| Notifications: `NotificationRequested` | `notifications.requested` | `notifications.requested.dlq` |

### Envelope JSON

Todos los eventos tienen esta forma:

```json
{
  "event_id": "7a4d5c90-0d41-4f88-bf4c-111111111111",
  "event_type": "ShipmentCreated",
  "event_version": 1,
  "occurred_at": "2026-01-01T12:00:04Z",
  "correlation_id": "0b6f7b55-7a9f-4b1c-9f1e-222222222222",
  "causation_id": "4f2a5e61-0e72-4bb8-a2e1-333333333333",
  "payload": {
    "order_id": "order-uuid",
    "shipment_id": "shipment-uuid",
    "shipment_status": "created"
  }
}
```

| Campo | Significado | Regla |
| --- | --- | --- |
| `event_id` | Identidad única de esa publicación | Es la clave primaria de deduplicación. Una republicación accidental conserva el mismo valor. |
| `event_type` | Nombre del evento | Debe coincidir con el evento documentado. |
| `event_version` | Versión del esquema y significado | Es obligatorio; cambios incompatibles requieren nueva versión o evento. |
| `occurred_at` | Momento en que el productor generó el evento | Timestamp UTC ISO 8601. |
| `correlation_id` | Identidad del flujo completo | El primer productor lo crea; los eventos derivados lo conservan. |
| `causation_id` | Evento inmediatamente anterior que causó esta transición | Es `null` para `OrderCreated`; luego contiene el `event_id` del padre. |
| `payload` | Datos propios del evento | Solo contiene los campos mínimos del contrato. Campos nuevos deben ser compatibles; los consumidores deben ignorar campos desconocidos. |

`event_id` identifica un mensaje; `correlation_id` identifica la historia completa. No deben confundirse.

### Catálogo completo de eventos

| Evento | Productor | Consumidor | Routing key |
| --- | --- | --- | --- |
| `OrderCreated` | `commerce-laravel` | simulador de pago dentro de Commerce | `commerce.order.created` |
| `PaymentApproved` | `commerce-laravel` | siguiente transición lógica dentro de Commerce; no tiene una cola separada | `commerce.payment.approved` |
| `ShipmentRequested` | `commerce-laravel` | `logistics-go` | `commerce.shipment.requested` |
| `ShipmentCreated` | `logistics-go` | `notifications-nestjs` | `logistics.shipment.created` |
| `NotificationRequested` | `notifications-nestjs` | handler de entrega dentro de Notifications | `notifications.requested` |

Payload mínimo de cada evento:

```json
// OrderCreated — versión 1
{
  "order_id": "order-uuid"
}
```

```json
// PaymentApproved — versión 1
{
  "order_id": "order-uuid",
  "payment_status": "approved",
  "simulated": true
}
```

```json
// ShipmentRequested — versión 1
{
  "order_id": "order-uuid"
}
```

```json
// ShipmentCreated — versión 1
{
  "order_id": "order-uuid",
  "shipment_id": "shipment-uuid",
  "shipment_status": "created"
}
```

```json
// NotificationRequested — versión 1
{
  "order_id": "order-uuid",
  "shipment_id": "shipment-uuid",
  "notification_type": "shipment_created"
}
```

El contrato no incluye dirección, carrier, tarifa, dimensiones, estimación, destinatario ni canal de notificación. Agregar esos campos requiere una revisión de compatibilidad. En la implementación actual `PaymentApproved` no dispara un consumidor RabbitMQ separado: el mismo procesamiento de `OrderCreated` registra `PaymentApproved` y `ShipmentRequested` de forma causal en el outbox.

### Flujo paso a paso

```text
1. OrderCreated
       commerce-laravel
          |
2. PaymentApproved (simulado)
   ShipmentRequested
          |
          v
3. logistics-go
          |
4. ShipmentCreated
          |
          v
5. notifications-nestjs
          |
6. NotificationRequested
          |
7. Entrega simulada -> delivered
```

#### Paso 1: `OrderCreated`

1. El cliente llama `POST /api/v1/orders`.
2. Commerce valida ítems y cantidades.
3. Obtiene los precios desde su propia base; no acepta el precio calculado por el cliente.
4. En una transacción crea pedido, ítems y evento `OrderCreated` en `outbox_events`.
5. El comando `php artisan events:publish` publica el evento al exchange `platform.events` y espera publisher confirm.

El evento solo se publica después de que el pedido existe de manera durable en Commerce.

#### Paso 2: `PaymentApproved` y `ShipmentRequested`

1. `php artisan events:consume-order-created --once` consume `OrderCreated` con ACK manual.
2. Commerce registra el `event_id` en `processed_events`.
3. Cambia el pedido a `payment_approved` como simulación local.
4. Crea `PaymentApproved` con `causation_id = event_id(OrderCreated)`.
5. Crea `ShipmentRequested` con `causation_id = event_id(PaymentApproved)`.
6. Guarda ambos eventos en el outbox en la misma transacción del cambio de pedido.
7. El consumidor envía ACK solo después del commit.

Aunque los dos eventos se registran en una transacción, siguen siendo mensajes independientes. Los publicadores los entregan al broker y los consumidores deben tolerar duplicados.

#### Paso 3: `ShipmentRequested`

1. `logistics-go` consume `commerce.shipment.requested`.
2. Verifica tipo, versión y payload.
3. Inserta el `event_id` en su `processed_events`.
4. Crea el envío en PostgreSQL de logística con un `shipment_id` generado por logística.
5. Registra `ShipmentCreated` en el outbox con el mismo `correlation_id` y como causación el `event_id` de `ShipmentRequested`.
6. Hace commit y recién después envía ACK.

Logística guarda `order_id` como referencia. No necesita copiar el pedido completo para crear su envío inicial.

#### Paso 4: `ShipmentCreated`

1. El publisher de Go obtiene el outbox pendiente.
2. Publica `ShipmentCreated` con confirmación de RabbitMQ.
3. Marca la fila como publicada solo después del confirm.
4. `notifications-nestjs` consume `logistics.shipment.created`.

#### Paso 5: `NotificationRequested`

1. Notifications verifica el envelope de `ShipmentCreated` y deduplica por `event_id`.
2. Crea en su base una solicitud con estado `requested`.
3. Crea `NotificationRequested` en su outbox dentro de la misma transacción.
4. Confirma el mensaje entrante después del commit.
5. El publisher de NestJS publica el nuevo evento con un confirm channel.

#### Paso 6: `delivered`

1. El mismo servicio consume `notifications.requested`.
2. Deduplica `NotificationRequested` en su propia frontera.
3. Cambia la solicitud local de `requested` a `delivered` dentro de una transacción.
4. La “entrega” termina allí porque no hay un proveedor externo.

`delivered` significa que la simulación local procesó la solicitud. No significa que un email o SMS real haya llegado a una persona.

### Cadena de causalidad

Todos los eventos de un pedido conservan el mismo `correlation_id`:

```text
OrderCreated
  event_id = A, causation_id = null, correlation_id = C
PaymentApproved
  event_id = B, causation_id = A,    correlation_id = C
ShipmentRequested
  event_id = D, causation_id = B,    correlation_id = C
ShipmentCreated
  event_id = E, causation_id = D,    correlation_id = C
NotificationRequested
  event_id = F, causation_id = E,    correlation_id = C
```

Esto permite agrupar una historia distribuida sin inventar un orden global de todos los pedidos.

## 9. Outbox, inbox, ACK y fallos

### Por qué se usan juntos

Cada mecanismo resuelve una ventana de fallo distinta:

| Mecanismo | Protege contra |
| --- | --- |
| Outbox | Guardar el cambio local y perder el evento antes de publicarlo. |
| Publisher confirm | Marcar publicado sin que RabbitMQ haya aceptado el mensaje. |
| Inbox (`processed_events`) | Aplicar dos veces el mismo evento redeliverado. |
| ACK manual | Confirmar el mensaje antes del commit local. |
| Idempotencia de negocio | Crear dos veces una transición por un duplicado. |
| DLQ | Perder silenciosamente mensajes que requieren diagnóstico. |

### Escenarios de fallo

| Escenario | Comportamiento actual |
| --- | --- |
| Falla la base antes del commit local | No queda negocio ni outbox confirmado. El consumidor no debería ACKear; la implementación actual rechaza el fallo hacia la DLQ porque todavía no hay delayed retry. |
| El proceso cae antes de publicar el outbox | La fila queda pendiente. El publisher puede volver a intentarlo: Artisan al ejecutar otra vez, o el polling del worker Go/NestJS. |
| RabbitMQ no confirma la publicación | El outbox permanece pendiente y el publisher informa el error. No se marca como publicado. |
| RabbitMQ confirma y el proceso cae antes de marcar la fila | La misma fila se publica otra vez. Conserva el `event_id`; el consumidor reconoce el duplicado y no repite el cambio. |
| El consumidor hace commit y cae antes de ACK | RabbitMQ redelivera el mensaje. `processed_events` indica que ya se procesó; el consumidor hace ACK sin repetir negocio. |
| Payload inválido, versión no soportada o regla permanente inválida | Se rechaza sin reencolar y se envía a la DLQ del consumidor. |
| RabbitMQ se reinicia mientras el worker está conectado | La implementación actual no ofrece reconexión automática; se debe diagnosticar y reiniciar el proceso cuando corresponda. |

El diseño evita prometer “exactly once” de extremo a extremo. La propiedad real es entrega al menos una vez más consumidores idempotentes y cambios locales transaccionales.

## 10. Retries y DLQ: la limitación actual

### Contrato deseado

El contrato de eventos describe una política futura de hasta tres reintentos después del intento inicial, cuatro intentos totales, con retrasos crecientes y un contador en headers o mecanismo equivalente. Los errores permanentes deben ir directamente a DLQ.

### Implementación actual

Los consumidores sí declaran:

- exchange durable `platform.events`;
- exchange durable `platform.events.dlx`;
- colas y DLQ durables por consumidor;
- rechazo sin reencolado de errores de procesamiento;
- conservación del envelope y metadatos de broker para inspección.

Todavía no implementan delayed retries automáticos. Por eso un fallo no pasa actualmente por una secuencia automática de backoff creciente; se dirige a la DLQ para inspección o replay controlado. Tampoco existe una política de reconexión automática completa para RabbitMQ.

### Por qué es una limitación consciente

La primera etapa prioriza demostrar ownership, transacciones, outbox, inbox, ACK y confirms. Implementar retries reales exige resolver también:

- clasificación confiable de errores transitorios y permanentes;
- headers y conteo de intentos;
- TTL y colas de retry o una estrategia equivalente;
- backoff creciente sin busy-loop;
- límites de concurrencia y orden causal;
- reconexión de canales de consumo y publicación;
- observabilidad de cada intento;
- replay seguro desde DLQ.

Añadir solo un `nack` con reencolado podría producir un loop rápido y ocultar el problema. Por eso la versión actual prefiere una DLQ visible y controlada antes que fingir una política de retry completa.

### Qué se haría después

1. Clasificar errores y definir el contador de intentos.
2. Agregar colas de retry con retrasos crecientes o una topología equivalente.
3. Mantener el `event_id` original y los metadatos de causalidad.
4. Agregar reconexión y recreación de canales.
5. Añadir métricas, logs estructurados y una herramienta de replay con límites.
6. Probar duplicados, reinicios y agotamiento de retries en un smoke test automatizado.

## 11. Docker y la infraestructura local

### Por qué se agregan ahora los contenedores de aplicación

Las primeras etapas validaron cada servicio y el flujo RabbitMQ ejecutando dependencias en Docker y procesos desde el host. Eso dejó una limitación concreta: el PHP 8.2 del host no satisface Laravel 13, y una verificación completa exigía instalar PHP, Go y Node.js con versiones compatibles. Ahora que las fronteras, los entrypoints y las variables reales están comprobados, el siguiente work unit empaqueta esos procesos sin reescribir la arquitectura.

La decisión es deliberadamente local: se agregan Dockerfiles reproducibles y un perfil Compose, no una plataforma de despliegue. Así se mejora la repetibilidad del aprendizaje sin presentar este entorno como producción.

### Dos modos de Compose

Desde la raíz:

```bash
cp .env.example .env
docker compose --env-file .env.example config --quiet
docker compose --env-file .env.example --profile app config --quiet

# Solo infraestructura; es el comportamiento predeterminado.
docker compose up -d

# Infraestructura más todas las aplicaciones.
docker compose --profile app up -d --build
docker compose --profile app ps
```

Los servicios de aplicación tienen el perfil `app`; por eso `docker compose up -d` no cambia el flujo anterior. El perfil explícito evita que una verificación de infraestructura inicie automáticamente workers, migraciones o APIs.

### Contenedores y procesos

Compose levanta cuatro contenedores de dependencia:

1. `rabbitmq`, imagen `rabbitmq:3.13-management-alpine`;
2. `postgres-commerce`, imagen `postgres:16-alpine`;
3. `postgres-logistics`, imagen `postgres:16-alpine`;
4. `postgres-notifications`, imagen `postgres:16-alpine`.

Con `--profile app` agrega estos procesos separados:

| Proceso | Entry point real | Responsabilidad |
| --- | --- | --- |
| `commerce-api` | `php artisan serve ...` | API Laravel, migraciones de inicio y `/up`. |
| `commerce-publisher` | `php artisan events:publish --loop=1` | Publica el outbox pendiente cada segundo. |
| `commerce-consumer` | `php artisan events:consume-order-created` | Consumer long-running de `OrderCreated`, ACK y DLQ. |
| `logistics-migrator` | `cmd/migrate` | Aplica una vez las migraciones embebidas de Go. |
| `logistics-api` | `cmd/server` | API Go y `/healthz`. |
| `logistics-worker` | `cmd/worker` | Consumer de `ShipmentRequested` y publisher con polling interno. |
| `notifications-api` | `node dist/main.js` | API NestJS y `/healthz`, con RabbitMQ deshabilitado. |
| `notifications-worker` | `node dist/worker.js` | Consumers y timer de outbox con RabbitMQ habilitado. |

La separación es importante: una API HTTP no se convierte accidentalmente en un consumer, y los publishers/outboxes se observan como procesos propios. Commerce necesitaba un modo periódico porque su comando publisher existente era one-shot; se agregó la opción `--loop=1` al mismo comando, en vez de crear un loop shell frágil. Go y NestJS ya tenían workers long-running y conservan sus loops internos.

### Puertos

| Contenedor | Host | Interior del network |
| --- | --- | --- |
| RabbitMQ AMQP | `127.0.0.1:5673` | `rabbitmq:5672` |
| RabbitMQ Management UI | `http://127.0.0.1:15673` | `rabbitmq:15672` |
| PostgreSQL Commerce | `127.0.0.1:55432` | `postgres-commerce:5432` |
| PostgreSQL Logistics | `127.0.0.1:55433` | `postgres-logistics:5432` |
| PostgreSQL Notifications | `127.0.0.1:55434` | `postgres-notifications:5432` |
| Commerce API | `127.0.0.1:8000` | `commerce-api:8000` |
| Logistics API | `127.0.0.1:8080` | `logistics-api:8080` |
| Notifications API | `127.0.0.1:3000` | `notifications-api:3000` |

Los puertos del host no son los predeterminados para reducir conflictos con instalaciones locales. Entre contenedores se deben usar los endpoints internos; un proceso ejecutado en el host usa los puertos publicados. Las aplicaciones reciben `postgres-commerce`, `postgres-logistics`, `postgres-notifications` y `rabbitmq` como nombres DNS internos; nunca deben usar `127.0.0.1` para hablar con otro contenedor.

Para observar APIs, publishers, outboxes y consumers:

```bash
curl http://127.0.0.1:8000/up
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:3000/healthz
docker compose --profile app logs -f commerce-publisher commerce-consumer
docker compose --profile app logs -f logistics-worker notifications-worker
```

### Volúmenes

Los volúmenes nombrados preservan datos cuando se ejecuta `docker compose down`:

- `commerce-logistics-platform-rabbitmq-data`;
- `commerce-logistics-platform-commerce-postgres-data`;
- `commerce-logistics-platform-logistics-postgres-data`;
- `commerce-logistics-platform-notifications-postgres-data`.

Para borrar los datos intencionalmente:

```bash
docker compose down -v
```

Este comando no es un “stop” inocuo: elimina los volúmenes del proyecto.

### Healthchecks y dependencias

RabbitMQ usa:

```text
rabbitmq-diagnostics -q ping
```

Cada PostgreSQL usa `pg_isready` con sus credenciales y base configuradas. Los checks se ejecutan cada cinco segundos, con doce reintentos y un período inicial apropiado para el servicio. Para inspeccionarlos:

```bash
for service in rabbitmq postgres-commerce postgres-logistics postgres-notifications; do
  container_id="$(docker compose ps -q "$service")"
  docker inspect --format '{{.Name}}: {{.State.Health.Status}}' "$container_id"
done
```

Las APIs tienen healthchecks HTTP. Los workers esperan dependencias saludables antes de iniciar; `logistics-migrator` es un job one-shot y su salida `Exited (0)` es esperada. Estas dependencias ordenan el entorno local, pero no son un orquestador de producción.

### Red y `internal: true`

Todos los servicios se conectan a `platform-internal`. La red conserva la comunicación interna por nombres de servicio. No tiene `internal: true`.

La razón es específica del desarrollo local en Linux: una red Docker marcada como interna impedía que el host alcanzara los puertos publicados, aunque Compose mostrara los bindings configurados. Quitar ese flag fue el cambio mínimo para que procesos ejecutados en el host puedan conectarse a `127.0.0.1:55432`, `55433`, `55434` y `5673`.

**Trade-off:** la red ya no es una frontera de aislamiento de producción. Esto es aceptable para el entorno educativo local, pero no debe confundirse con una política de seguridad productiva. En producción se necesitarían redes, firewall, credenciales, TLS y reglas de exposición diseñadas para ese entorno.

Más detalle en [`infrastructure/docker/README.md`](../infrastructure/docker/README.md) y [`docker-compose.yml`](../docker-compose.yml).

## 12. Estrategia de pruebas y verificación

La estrategia separa pruebas rápidas de comportamiento, pruebas de integración con dependencias y un smoke test distribuido.

### Capas

| Capa | Qué verifica | Dependencias |
| --- | --- | --- |
| Unitarias | Envelopes, dominio, validaciones y workflows aislados | Fakes o memoria |
| HTTP | Status codes, rutas, validación y respuestas | Servidor de pruebas; no necesariamente PostgreSQL |
| Migraciones | Que el esquema se pueda crear y que el runner sea repetible | PostgreSQL real |
| Integración de mensajería | Topología, outbox, confirm, ACK y deduplicación | RabbitMQ y PostgreSQL |
| Smoke E2E | `OrderCreated` hasta `delivered` atravesando los tres servicios | Todos los servicios, RabbitMQ y tres PostgreSQL |

### Comandos por servicio

#### Laravel

La suite usa SQLite en memoria por `phpunit.xml`, por lo que no necesita PostgreSQL ni RabbitMQ:

```bash
docker run --rm \
  -u "$(id -u):$(id -g)" \
  -v "$PWD/services/commerce-laravel:/app" \
  -w /app \
  --entrypoint php composer:2 artisan test
```

Para la base real, el servicio documenta:

```bash
cd services/commerce-laravel
php artisan migrate
php artisan events:publish
php artisan events:consume-order-created --once
```

La publicación AMQP necesita la extensión `sockets`. La conexión PostgreSQL del runtime necesita `pdo_pgsql`.

#### Go

```bash
cd services/logistics-go
go test ./...
go vet ./...
go run ./cmd/migrate
go run ./cmd/server
go run ./cmd/worker
```

Los tests unitarios y HTTP usan fakes y no necesitan PostgreSQL ni RabbitMQ. El servidor, migrador y worker sí necesitan las dependencias reales.

#### NestJS

```bash
cd services/notifications-nestjs
npm install
npm run format:check
npm run lint
npm test
npm run build
npm run migration:run
npm run start:dev
```

El worker puede ejecutarse después de `npm run build` con `npm run start:worker`. RabbitMQ se habilita con `RABBITMQ_ENABLED=true`; por defecto está deshabilitado.

### Qué se verificó

En la verificación de este estado del repositorio se comprobó:

- `docker compose config --quiet` sin errores.
- Los cuatro contenedores Compose en estado `healthy`, con los puertos publicados esperados.
- `go test ./...` y `go vet ./...` correctos.
- Migraciones Go aplicadas correctamente con el `DATABASE_URL` de `services/logistics-go/.env.example`.
- `npm run format:check`, `npm run lint`, `npm test` y `npm run build` correctos en NestJS: cuatro suites, nueve tests.
- Migraciones TypeORM sin migraciones pendientes en PostgreSQL de notificaciones.
- Suite Laravel ejecutada dentro del runtime Docker compatible: doce tests y 56 assertions correctas.
- Smoke test distribuido contra RabbitMQ y PostgreSQL: la cadena `OrderCreated -> PaymentApproved -> ShipmentRequested -> ShipmentCreated -> NotificationRequested -> delivered` produjo los outbox confirmados y la transición de notificación simulada. También se verificó el comportamiento de entrega duplicada por `event_id`.
- `git diff --check` sin errores de whitespace.

No existe todavía un script único de smoke test versionado; la verificación distribuida requiere iniciar los procesos y observar las bases y colas. Esa es una deuda de automatización, no una funcionalidad del dominio.

### Limitaciones del entorno

- El PHP del host es 8.2, mientras Laravel 13 requiere PHP `>=8.3`; por eso las pruebas Laravel se ejecutan con la imagen Docker documentada.
- Para probar el runtime real de Laravel se necesitan `pdo_pgsql` y `sockets`; la suite SQLite con mocks no prueba esas extensiones.
- Los tests de Go y NestJS con fakes no demuestran por sí solos conectividad real con RabbitMQ.
- Las migraciones y el smoke E2E necesitan PostgreSQL y RabbitMQ activos.
- Un healthcheck verde no prueba el contrato completo ni la idempotencia.
- El perfil `app` coordina el arranque local, pero no ofrece despliegue rolling, escalado, límites de recursos ni migraciones de producción.

## 13. Cómo leer el repositorio y aprender por etapas

### Etapa 1: alcance y vocabulario

Leer:

1. [`README.md`](../README.md), para la intención general y el estado.
2. [`docs/architecture.md`](architecture.md), para ownership y límites.
3. Esta guía hasta la sección de servicios.

Objetivo: poder responder qué servicio posee cada dato sin mencionar todavía RabbitMQ.

### Etapa 2: un servicio HTTP local

Comenzar por `logistics-go`:

1. Leer [`services/logistics-go/README.md`](../services/logistics-go/README.md).
2. Levantar `postgres-logistics`.
3. Ejecutar `go run ./cmd/migrate`.
4. Iniciar `go run ./cmd/server`.
5. Probar `/healthz`, `POST /shipments` y `GET /shipments/{id}`.

Objetivo: entender handler, dominio, repositorio y migraciones sin mensajería.

### Etapa 3: API con framework y persistencia aislada

Continuar con `notifications-nestjs`:

1. Leer su README y `package.json`.
2. Ejecutar `npm install` y `npm test`.
3. Ejecutar la migración contra `postgres-notifications`.
4. Probar `/healthz`, `POST /notifications` y `GET /notifications/{id}`.

Objetivo: comparar módulos, inyección de dependencias, DTOs y TypeORM con la implementación explícita de Go.

### Etapa 4: transacciones comerciales

Estudiar `commerce-laravel`:

1. Revisar `routes/api.php`.
2. Leer `CreateOrderAction`, modelos, migraciones y tests.
3. Observar que el total se calcula con precios del servidor.
4. Ejecutar la suite en Docker, no con el PHP 8.2 del host.

Objetivo: entender una transacción local antes de conectarla al broker.

### Etapa 5: contrato y confiabilidad

Leer [`docs/events.md`](events.md) junto con:

- `services/commerce-laravel/app/Events/EventEnvelope.php`;
- `services/commerce-laravel/app/Events/ProcessOrderCreated.php`;
- `services/logistics-go/internal/events/envelope.go`;
- `services/logistics-go/internal/application/shipment_requested.go`;
- `services/notifications-nestjs/src/messaging/event-envelope.ts`;
- `services/notifications-nestjs/src/notifications/application/notification-workflow.ts`.

Objetivo: seguir un `event_id`, un `correlation_id` y un `causation_id` a través de la cadena.

### Etapa 6: operación y fallos

Practicar:

- detener un consumidor antes del ACK;
- republicar un evento con el mismo `event_id`;
- observar que `processed_events` evita el trabajo duplicado;
- revisar una DLQ;
- simular una caída del broker y comprobar que el outbox no se marca como publicado sin confirm.

Objetivo: comprender que “funciona en el happy path” no equivale a “es confiable”.

## 14. Preguntas típicas de entrevista y respuestas sugeridas

### ¿Por qué tres microservicios y no un monolito?

Porque el objetivo del proyecto es aprender fronteras de dominio y comunicación distribuida. Comercio, logística y notificaciones tienen ownership distinto. No se afirma que tres servicios sean siempre mejores; para un producto pequeño un monolito modular podría costar menos.

### ¿Cómo evita el proyecto una base de datos compartida?

Cada servicio tiene una instancia PostgreSQL independiente y solo guarda sus propios datos. Los otros servicios reciben eventos y conservan referencias como `order_id`; no hacen joins ni escrituras cross-service.

### ¿Qué ocurre si RabbitMQ está caído después de crear un pedido?

El pedido y `OrderCreated` ya están en la transacción local. El outbox permanece pendiente y el publisher puede intentarlo después. La implementación actual no tiene reconexión automática completa; el proceso debe volver a conectarse al reiniciarse.

### ¿El sistema garantiza exactly-once?

No. RabbitMQ entrega al menos una vez y existe una ventana entre el publisher confirm y marcar el outbox. La solución es conservar el mismo `event_id`, registrar `processed_events` y hacer idempotentes los consumidores.

### ¿Por qué ACK manual?

Para confirmar solo después del commit local. Si el proceso falla antes del ACK, el broker puede redeliverar; el inbox local detecta que el negocio ya fue aplicado.

### ¿Qué representa `PaymentApproved`?

Una transición simulada y propiedad de Commerce. No representa autorización de un proveedor externo ni justifica crear un servicio de pagos en esta etapa.

### ¿Qué representa `ShipmentCreated`?

Que `logistics-go` creó un registro local de envío y publicó el evento. No significa carrier booking, tarifa, ruta ni entrega completada.

### ¿Qué representa `delivered`?

Que el handler de notificaciones completó la transición local simulada. No prueba la llegada de un email o SMS real.

### ¿Por qué RabbitMQ y no Kafka?

Porque el flujo inicial necesita routing por eventos, colas, ACK, confirms y DLQ con una topología pequeña. Kafka sería una alternativa válida si el requisito principal fuera retención de log, particiones, muchos consumidores o replay frecuente.

### ¿Por qué no agregar un API Gateway desde el comienzo?

Porque todavía no hay necesidad de agregación, políticas de borde o autenticación centralizada, y el acceso directo hace visible el ownership. Se reconsideraría si aparecen requisitos concretos.

### ¿Cómo se versiona un evento?

Con `event_version`. La versión 1 es obligatoria en el envelope. Cambios incompatibles requieren una nueva versión o un nuevo event type; cambios aditivos deben ser tolerados por consumidores que ignoren campos desconocidos.

### ¿Cómo se manejan los errores permanentes?

Se rechazan sin reencolar y se envían a la DLQ del consumidor para inspección y replay controlado. La política de delayed retries para errores transitorios todavía no está implementada.

### ¿Qué cambiaría en producción?

Como mínimo: autenticación, secretos reales, TLS, observabilidad, alertas, backups, despliegue, política de retries con backoff, reconexión, replay seguro y un API Gateway si una necesidad concreta lo justifica. Los contenedores locales ya existen, pero no resuelven esos requisitos de producción.

## 15. Glosario final

| Término | Definición breve |
| --- | --- |
| ACK | Confirmación de que un consumidor terminó un mensaje. |
| API Gateway | Punto de entrada que puede enrutar y centralizar políticas para varias APIs. |
| At-least-once | Semántica de entrega en la que un mensaje puede llegar una o más veces, pero no se debe perder silenciosamente. |
| Bounded context | Límite donde un modelo y su vocabulario tienen un significado consistente. |
| Broker | Sistema que recibe, enruta y entrega mensajes. |
| Causation ID | Identificador del evento inmediatamente anterior que causó otro evento. |
| Consistencia eventual | Los datos distribuidos pueden diferir temporalmente y converger después. |
| Correlation ID | Identificador compartido por los eventos de un mismo flujo. |
| DLQ | Cola de dead letters para mensajes rechazados o agotados. |
| Evento | Mensaje que describe una transición ocurrida. |
| Exchange | Entrada de RabbitMQ que enruta publicaciones hacia colas. |
| Healthcheck | Comprobación automática de disponibilidad técnica. |
| Idempotencia | Propiedad de repetir una operación sin repetir su efecto de negocio. |
| Inbox | Registro local de mensajes entrantes ya procesados. |
| Microservicio | Servicio autónomo con frontera de negocio, almacenamiento y ciclo de despliegue propios. |
| Monolito | Aplicación principal desplegada como una unidad. |
| Outbox | Registro transaccional de eventos pendientes de publicar. |
| Payload | Datos específicos que transporta un evento. |
| Publisher confirm | Confirmación del broker de que aceptó una publicación. |
| Queue / cola | Estructura que conserva mensajes pendientes para un consumidor. |
| RabbitMQ | Broker de mensajería usado por esta plataforma. |
| Routing key | Clave que usa un exchange `topic` para seleccionar bindings y colas. |
| Síncrono | Comunicación que espera una respuesta antes de continuar. |
| Asíncrono | Comunicación que permite continuar mientras el receptor procesa después. |
| Ownership | Autoridad de un servicio sobre sus datos y reglas. |
| Retry | Nuevo intento de procesar un mensaje fallido. |

## 16. Decisiones futuras pendientes

Estas decisiones no están implementadas en la etapa actual:

1. **Operación de aplicaciones:** endurecer las imágenes, separar migraciones de despliegue, definir escalado y operar los procesos fuera del Compose educativo.
2. **Autenticación:** definir identidad, autorización, propagación de credenciales y protección de las APIs públicas.
3. **Observabilidad:** agregar logs estructurados, métricas, trazas distribuidas, correlation IDs visibles y alertas.
4. **API Gateway eventual:** introducirlo solo si una necesidad concreta de API pública, políticas de borde o agregación lo justifica.
5. **Retries con backoff:** implementar delayed retries, clasificación de errores, conteo de intentos y replay seguro.
6. **Reconexión:** agregar recuperación automática de conexiones y canales RabbitMQ, con pruebas de reinicio y backpressure.

Cada punto debe convertirse en una decisión revisable con alcance, contrato, costo y pruebas. No debe agregarse solo porque sea una pieza habitual de una arquitectura de microservicios.
