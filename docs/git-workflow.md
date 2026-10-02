# Flujo de Git para Commerce Logistics Platform

Este documento explica desde cero cómo se organiza el trabajo de este repositorio. La decisión es una variante de Git Flow con `develop` como integración, `main` como estado estable y una feature-branch chain para cambios grandes.

## Ruta rápida

1. Una issue aprobada define el objetivo.
2. Una rama aislada contiene el trabajo.
3. Cada commit representa una unidad entendible.
4. Una PR permite revisar, ejecutar CI y acordar el rollback.
5. `develop` integra; `main` recibe releases y hotfixes.

## Conceptos básicos

### Rama

Una rama es un puntero con nombre a una línea de commits. Crear una rama no copia todo el proyecto: permite avanzar sin mover la línea estable. Por eso se trabaja en `feat/*`, `fix/*` o `ci/*` y no directamente sobre `main`.

### Commit

Un commit es una instantánea con autor, mensaje y padres. Debe contar una decisión pequeña: por ejemplo, agregar una validación de CI y su prueba, no simplemente “cambiar archivos YAML”. Si una unidad necesita código y pruebas, ambos deben estar en el mismo commit o work unit.

### Pull Request

Una PR es una propuesta de cambios entre dos ramas. No es solo una comparación de archivos: concentra el objetivo, la issue, la evidencia de pruebas, la revisión y el límite de rollback. En este repositorio toda PR debe enlazar una issue aprobada y tener exactamente una etiqueta `type:*`.

## La variante elegida

Este repositorio tiene tres servicios con tecnologías y ciclos de verificación distintos: Laravel/PHP, Go y NestJS/Node. Necesitamos una rama estable que no reciba trabajo parcialmente integrado, pero también una rama donde se pueda probar la interacción completa antes de publicar.

| Elemento | Decisión | Razón |
| --- | --- | --- |
| `main` | Rama estable | Solo debe representar una versión revisada y publicable. |
| `develop` | Rama de integración | Permite verificar cambios combinados de los tres servicios. |
| `feat/*`, `fix/*`, `ci/*`, `docs/*` | Ramas de trabajo desde `develop` | Aíslan el cambio y mantienen una base común. |
| `release/*` | Preparación desde `develop` hacia `main` | Separa estabilización de desarrollo activo. |
| `hotfix/*` | Corrección desde `main` | Permite resolver una regresión estable sin esperar todo `develop`. |
| Feature-branch chain | PRs sucesivas hacia la rama padre | Reduce la carga de revisión cuando una iniciativa supera unas 400 líneas. |

No se eligió trabajar con PRs independientes directamente a `main`: una parte de una integración distribuida podría publicarse antes de que los demás servicios estén listos. Tampoco se permite una rama de larga duración sin PRs: escondería demasiado contexto y haría más caro detectar conflictos.

## Diagrama del flujo

```text
                         tag vX.Y.Z
main (estable) ────────────────────────────────●
       ▲                                          │
       │ PR release/*                             │ rollback al tag anterior
       │                                          │
develop (integración) ◄──────────────────────────┘
       ▲
       │ PR tracker después de integrar la cadena
       │
feat/integration (tracker, draft)
       ▲
       └── feat/integration-01-foundation
               ▲ PR #1
               │
               └── feat/integration-02-service-checks
                       ▲ PR #2
                       │
                       └── feat/integration-03-smoke-flow
                               ▲ PR #3
```

La rama tracker se crea desde `develop` y su PR apunta a `develop`. La primera rama hija apunta al tracker; cada rama posterior apunta a la hija inmediata anterior. La flecha del diagrama indica dependencia, no un merge automático.

## Cómo trabajar paso a paso

### 1. Issue y alcance

La issue debe explicar objetivo, contexto, alcance, criterios de aceptación y riesgos. Antes de iniciar el trabajo debe estar aprobada. Si el cambio no cabe en una unidad independiente, se divide antes de programar.

### 2. Rama y commits

```bash
git switch develop
git pull --ff-only origin develop
git switch -c ci/full-compose-integration
```

Los nombres usan minúsculas y guiones. Los commits usan Conventional Commits, por ejemplo:

```text
ci(workflows): verify the full compose event flow
```

Un commit debe dejar el proyecto en un estado comprensible. Evita commits que solo agreguen pruebas de una implementación que todavía no existe o que mezclen documentación no relacionada.

### 3. PR simple o cadena

Hasta aproximadamente 400 líneas cambiadas, una PR enfocada suele ser suficiente. Si se supera ese límite, identifica work units independientes y crea una feature-branch chain:

1. crea la rama tracker desde `develop` y abre su PR draft hacia `develop`;
2. crea la rama hija 01 desde el tracker y abre la PR hacia el tracker;
3. crea cada rama siguiente desde la rama anterior y apunta a su padre inmediato;
4. integra y revisa en orden, manteniendo cada diff limpio;
5. integra la tracker en `develop` solo cuando toda la cadena esté lista.

Cada PR incluye un diagrama con `📍` sobre la PR actual, el presupuesto de líneas, la dependencia, el siguiente paso y el rollback boundary. Si una PR hija muestra cambios de su abuelo además de su unidad, la base está mal: hay que retargetear o rebasar, no pedir al revisor que ignore el ruido.

## CI y CD

**CI (Continuous Integration)** ejecuta verificaciones automáticas cada vez que un cambio se propone o se integra. En este repositorio valida Compose, Laravel, Go, NestJS y un smoke flow completo. CI responde: “¿este cambio mantiene la calidad y la integración esperadas?”.

**CD (Continuous Delivery/Deployment)** toma artefactos que ya pasaron CI y los entrega o despliega. El workflow de release de este repositorio hace una parte deliberadamente limitada: construye y publica imágenes versionadas en GHCR cuando se crea un tag `vX.Y.Z`. No despliega a producción, no configura infraestructura y no contiene credenciales de servicios.

La diferencia importa: una imagen publicada por CD no demuestra por sí sola que haya sido promovida a producción; solo demuestra que el artefacto fue construido y almacenado.

## Verificación y flujo distribuido

La integración de CI levanta el perfil completo de Compose, espera `/up` en Laravel y `/healthz` en Go y NestJS, crea un producto y un pedido por HTTP y consulta la base de notificaciones hasta observar `delivered`. Esa consulta verifica el resultado observable del recorrido:

```text
OrderCreated → PaymentApproved → ShipmentRequested
             → ShipmentCreated → NotificationRequested → delivered
```

Las bases de datos pertenecen a cada servicio. Por eso la verificación consulta el estado final en PostgreSQL de notificaciones, no tablas internas de otro servicio ni una respuesta inventada por el workflow.

## Rollback

El rollback debe corresponder al work unit que se está retirando:

- **Antes del merge:** cerrar la PR y borrar la rama si no se necesita conservarla.
- **En una cadena:** detener la cadena en la PR defectuosa; las ramas posteriores no se integran.
- **Después del merge en `develop`:** crear una PR de `revert` que retire solo el merge afectado.
- **Después de un tag:** volver a promover el tag estable anterior; no reescribir ni mover tags existentes.
- **Con migraciones:** revisar primero si el rollback de código es compatible con el esquema ya aplicado. Un revert de Git no deshace automáticamente una migración de base de datos.

Cada PR debe escribir explícitamente su frontera, por ejemplo: “se pueden retirar `.github/workflows/ci.yml` y `.github/workflows/release.yml` sin cambiar servicios ni Compose”.

## Checklist de release

- [ ] Todas las issues incluidas en `develop` están aprobadas y cerradas por PR.
- [ ] CI está verde para la cadena completa.
- [ ] El diff de `release/vX.Y.Z` contiene solo preparación de release.
- [ ] Se confirmó el número semántico `X.Y.Z` y no existe ya el tag.
- [ ] La PR a `main` tiene exactamente una etiqueta `type:*`, revisión y rollback boundary.
- [ ] El merge a `main` creó el tag `vX.Y.Z`.
- [ ] El workflow publicó las cinco imágenes esperadas en GHCR: `commerce`, `logistics-api`, `logistics-worker`, `notifications-api` y `notifications-worker`.
- [ ] Se sincronizaron `main` y `develop` después del release.
- [ ] Se documentó que la publicación de imágenes no es un despliegue a producción.

Para los comandos exactos de cada servicio, consulta la [guía de contribución](../CONTRIBUTING.md). Para el comportamiento local de Compose, consulta [`infrastructure/docker/README.md`](../infrastructure/docker/README.md).
