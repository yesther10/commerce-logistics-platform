# Guía para contribuir

Esta guía define cómo se mueve un cambio desde una issue aprobada hasta `develop` y, después, hasta `main`. La regla principal es mantener cada unidad de trabajo pequeña, verificable y reversible.

## Ruta rápida

1. Crea o toma una issue de trabajo y espera la etiqueta `status:approved`.
2. Crea una rama desde `develop` con el prefijo adecuado.
3. Implementa una sola unidad de trabajo con commits Conventional Commits.
4. Ejecuta los comandos locales de este documento.
5. Abre la PR con `Closes #N`, exactamente una etiqueta `type:*`, la cadena y el límite de rollback.
6. Espera CI verde y revisión antes de integrar.

## Git Flow elegido

`main` es la rama estable. `develop` es la rama de integración. Las ramas de trabajo nacen de `develop` y las PRs normales se integran primero allí.

| Rama | Propósito | Base habitual | Destino habitual |
| --- | --- | --- | --- |
| `main` | Estado estable y versionado | — | — |
| `develop` | Integración de trabajo terminado | `main` al iniciar una línea de desarrollo | `main` mediante `release/*` |
| `feat/*` | Nueva capacidad o mejora funcional | `develop` o el tracker de su cadena | `develop` o la rama padre de la cadena |
| `fix/*` | Corrección de un defecto no urgente | `develop` | `develop` |
| `ci/*` | Automatización y validaciones de CI | `develop` | `develop` |
| `docs/*` | Documentación independiente | `develop` | `develop` |
| `release/*` | Preparación de una versión estable | `develop` | `main` y sincronización posterior a `develop` |
| `hotfix/*` | Corrección urgente de una versión estable | `main` | `main` y sincronización posterior a `develop` |

Las ramas se nombran en minúsculas: `feat/order-history`, `ci/full-integration`, `docs/git-workflow` o `hotfix/payment-timeout`.

### Feature-branch chain

Cuando una iniciativa supera aproximadamente 400 líneas cambiadas, se usa una cadena de ramas, no una PR gigante:

```text
main (estable)
  ↑ PR de release/*
develop (integración)
  ↑ PR tracker, draft y sin merge directo
feat/platform-foundation (tracker)
  ↑ PR #1
feat/platform-foundation-01-ci
  ↑ PR #2
feat/platform-foundation-02-integration
```

La rama tracker se crea desde `develop` y su PR apunta a `develop`. Cada rama hija apunta a su padre inmediato. La PR tracker se integra solo después de que las hijas hayan sido revisadas e integradas en orden. Cada PR debe declarar inicio, fin, dependencia, siguiente paso, alcance y límite de rollback.

## Reglas de Pull Request

Una PR solo puede integrarse cuando cumple todo lo siguiente:

- enlaza una issue aprobada con `Closes #N`, `Fixes #N` o `Resolves #N`;
- tiene exactamente una etiqueta `type:*` (`type:bug`, `type:feature`, `type:docs`, `type:refactor`, `type:chore` o `type:breaking-change`);
- tiene CI verde y los comandos locales relevantes ejecutados;
- tiene revisión de otra persona;
- mantiene un único work unit y un diff limpio respecto de su rama base;
- usa squash cuando la unidad debe llegar como un commit coherente; usa rebase para actualizar la rama sin crear merges accidentales. La estrategia concreta debe respetar la protección configurada en GitHub;
- no contiene commits de IA ni trailers `Co-Authored-By`.

No se permite fusionar una PR directamente a `main` para trabajo normal. `main` recibe releases y hotfixes.

## Conventional Commits

El mensaje tiene esta forma:

```text
<type>(<scope>): <descripción>
```

Tipos aceptados: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `style` y `test`. El scope es opcional, debe estar en minúsculas y la descripción debe explicar el resultado, no enumerar archivos.

Ejemplos:

```text
ci(workflows): add full compose integration check
fix(logistics): handle duplicate shipment events
```

Los commits deben ser humanos, revisables y sin `Co-Authored-By`.

## Work units y tamaño de PR

Un work unit es una entrega que tiene un solo propósito claro: un comportamiento, una corrección, una migración o una pieza de documentación que se puede verificar y revertir sin retirar trabajo no relacionado.

- Mantén el código con sus pruebas.
- Mantén la documentación con el cambio visible que explica.
- Registra el comando de prueba y su resultado.
- Define qué archivos o comportamiento se pueden revertir de forma independiente.
- No dividas por tipo de archivo (`models`, después `services`, después `tests`) si eso deja el repositorio incompleto en cada paso.

Cuenta adiciones más eliminaciones. Si el diff se aproxima a 400 líneas, detén la PR y revisa el límite. Por encima de 400 líneas se divide en work units independientes y se usa la feature-branch chain. Solo un artefacto generado o una migración que no pueda separarse limpiamente justificaría una excepción explícita revisada por una persona mantenedora.

## Release y hotfix

### Release normal

1. Verifica que `develop` contiene las unidades aprobadas y que CI está verde.
2. Crea `release/vX.Y.Z` desde `develop`.
3. Ajusta únicamente metadatos de release y correcciones de preparación.
4. Abre la PR hacia `main`, ejecuta la checklist de release y obtiene revisión.
5. Integra la release en `main`, crea el tag `vX.Y.Z` y sincroniza `main` de vuelta a `develop`.
6. El workflow de release publica imágenes en GHCR. Publicar artefactos **no despliega a producción**.

### Hotfix

1. Crea `hotfix/descripcion` desde el último estado estable de `main`.
2. Incluye la corrección mínima, sus pruebas y una issue aprobada.
3. Abre la PR hacia `main` con CI y revisión.
4. Integra, crea el siguiente tag patch y sincroniza la corrección en `develop`.

El rollback de una versión consiste en detener la promoción del tag nuevo y volver al tag estable anterior. En una PR no integrada, se cierra la rama. En una PR integrada, se revierte el merge dentro de la misma rama de integración; no se reescribe la historia compartida.

## Comandos locales exactos

Ejecuta desde la raíz del repositorio.

### Compose

```bash
cp .env.example .env
docker compose --env-file .env.example config --quiet
docker compose --env-file .env.example --profile app config --quiet
docker compose up -d
docker compose --profile app up -d --build
docker compose --profile app ps
docker compose --profile app down
```

Usa `docker compose down -v` solo para un reset local deliberado porque elimina las bases de datos y el estado de RabbitMQ.

### Laravel

```bash
cd services/commerce-laravel
composer install --no-interaction --prefer-dist --no-progress
composer validate --strict
vendor/bin/pint --test
php artisan test
cd ../..
```

### Go

```bash
cd services/logistics-go
test -z "$(gofmt -l cmd internal)"
go test ./...
go vet ./...
cd ../..
```

### NestJS

```bash
cd services/notifications-nestjs
npm ci
npm run format:check
npm run lint
npm test
npm run build
cd ../..
```

La misma secuencia se ejecuta automáticamente en `.github/workflows/ci.yml`. Los valores de CI provienen de `.env.example`; no se agregan credenciales de aplicación ni secretos del entorno local a los workflows.
