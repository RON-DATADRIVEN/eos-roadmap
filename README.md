# EOS – Roadmap público

Repositorio para la página pública del roadmap EOS en GitHub Pages.

La vista pública debe mostrar:

- Bugs conocidos
- Características en desarrollo

## Fuente de datos

La página carga `docs/modules.json`. Ese archivo puede editarse manualmente o generarse desde GitHub Projects mediante el workflow `.github/workflows/sync-modules.yml`.

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
