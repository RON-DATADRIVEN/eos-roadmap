# EOS – Roadmap público

Repositorio para la página pública del roadmap EOS en GitHub Pages.

La vista pública debe mostrar:

- Bugs conocidos
- Características en desarrollo

## Fuente de datos

## Operación del sync desde GitHub Projects

El workflow `.github/workflows/sync-modules.yml` puede regenerar `docs/modules.json` desde GitHub Projects.

Secretos requeridos:

| Secret           | Uso                                                                        |
| ---------------- | -------------------------------------------------------------------------- |
| `PROJECTS_TOKEN` | Leer GitHub Projects v2, issues y campos del proyecto.                     |
| `SYNC_PR_TOKEN`  | Crear o actualizar el PR automático con el `docs/modules.json` regenerado. |

El JSON generado por el sync debe cumplir `docs/modules.schema.json`.

La vista pública no debe exponer campos internos de aprobación, enlaces de Slack ni identificadores operativos del Project.



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
