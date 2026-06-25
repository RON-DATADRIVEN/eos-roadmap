# EOS – Roadmap público

Repositorio para la página pública del roadmap EOS en GitHub Pages.

La vista pública debe mostrar:

- Bugs conocidos
- Características en desarrollo


## Operación del sync desde GitHub Projects

El workflow `.github/workflows/sync-modules.yml` puede regenerar `docs/modules.json` desde GitHub Projects.

Secretos requeridos:

| Secret           | Uso                                                                        |
| ---------------- | -------------------------------------------------------------------------- |
| `PROJECTS_TOKEN` | Leer GitHub Projects v2, issues y campos del proyecto.                     |
| `SYNC_PR_TOKEN`  | Obligatorio. PAT o token de GitHub App dedicado para publicar datos generados directamente en `main`. |

El JSON generado por el sync debe cumplir `docs/modules.schema.json`.

La vista pública no debe exponer campos internos de aprobación, enlaces de Slack ni identificadores operativos del Project.

`eos-roadmap` opera con un modelo solo-dev. El sync no abre PR automático para datos generados: cuando cambian datos públicos, el workflow hace commit directo a `main` únicamente de `docs/modules.json` y `docs/modules-meta.json`.
La protección de `main` no requiere PR reviews ni required status checks para este repositorio. Como guardrails, la configuración debe seguir bloqueando force push y branch deletion si esas opciones están disponibles.
`SYNC_PR_TOKEN` sigue siendo obligatorio para publicar en `main`. Debe ser un PAT o token de GitHub App dedicado; no hay fallback a `GITHUB_TOKEN` y no debe usarse un token genérico sin control.
El workflow valida antes de hacer commit/push directo: ejecuta `go test ./...`, valida `docs/modules.json` contra `docs/modules.schema.json`, y aplica un allowlist exacto para que solo puedan quedar staged `docs/modules.json` y `docs/modules-meta.json`.



## Reglas públicas

### Bugs conocidos

Se muestran issues clasificados como bug.

### Características en desarrollo

Una característica aparece públicamente si cumple:

```text
Check Luis = Aprobado
AND
Status en Prototipado, Desarrollo, Test, Staging o Deploy
```

## Mapeo público de estados

| Estado interno | Texto público |
|---|---|
| Prototipado | En prototipo |
| Desarrollo | En desarrollo |
| Test | En pruebas |
| Staging | En validación |
| Deploy | Liberado |

## Página pública

<https://ron-datadriven.github.io/eos-roadmap/>
