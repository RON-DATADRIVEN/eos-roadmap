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

El sync no abre PR automático para datos generados. Cuando cambian datos públicos, el workflow hace commit directo a `main` únicamente de `docs/modules.json` y `docs/modules-meta.json`.
La protección de `main` debe permitir que el actor de `SYNC_PR_TOKEN` haga bypass controlado del requisito de PR review para este flujo. Si branch protection no permite ese bypass, el workflow fallará en `git push origin HEAD:main`. No hay fallback a `GITHUB_TOKEN` para publicar en `main` y no debe usarse un token genérico sin control.
El workflow ejecuta `go test ./...` y valida `docs/modules.json` contra `docs/modules.schema.json` antes de hacer commit/push directo, porque este flujo no depende de workflows `push` posteriores para validar los datos publicados.



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
