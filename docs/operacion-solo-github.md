# Operación solo con servicios de GitHub

Este documento resume la arquitectura del repositorio y describe cómo operar el
roadmap público utilizando únicamente servicios de GitHub. El objetivo es evitar
cualquier dependencia con Google Cloud (Logging, Run o Functions) sin perder
visibilidad ni automatización.

## 1. Componentes principales del repositorio

| Componente | Descripción | Servicio de GitHub relacionado |
| --- | --- | --- |
| `docs/` | Sitio estático publicado con GitHub Pages. Contiene `modules.json` y los recursos necesarios para renderizar el roadmap. | GitHub Pages |
| `cmd/create-issue/` | Servicio en Go que recibe solicitudes desde el modal público, crea Issues y los añade a un Project. | GitHub Projects v2 y API GraphQL |
| `.github/` (no versionado aquí, pero recomendado) | Lugar ideal para almacenar workflows que automaticen la validación y el despliegue del sitio. | GitHub Actions |
| `third_party/githubv4/` | Cliente GraphQL utilizado para interactuar con GitHub. | GitHub API |

## 2. Dependencias actuales de Google

La versión original del servicio `create-issue` enviaba cada registro a Cloud
Logging, lo que obligaba a configurar:

- Variables `LOGGING_PROJECT_ID`, `LOGGING_LOG_ID` y
  `GOOGLE_APPLICATION_CREDENTIALS`.
- Despliegues en Cloud Run mediante `gcloud`.

Tras la actualización incluida en este cambio, el servicio detecta cuando
`LOGGING_PROJECT_ID` está vacío y, en lugar de fallar, vuelca los registros en
stdout en formato JSON. Esto permite recopilar eventos desde:

- Registros de GitHub Actions (`steps.<id>.outputs`).
- Consola de GitHub Codespaces.
- Cualquier servicio propio (bare metal, VPS) sin proveedores externos.

## 3. Estrategia recomendada usando solo GitHub

### 3.1 Frontend en GitHub Pages
1. Edita `docs/modules.json` para reflejar el estado del roadmap.
2. Ejecuta `npm run build` (si existiera) o simplemente realiza el commit.
3. Empuja los cambios a `main`; GitHub Pages publicará automáticamente el sitio.

### 3.2 Backend en infraestructura propia o auto-gestionada
Aunque GitHub no ofrece un servicio HTTP permanente, existen alternativas que se
mantienen dentro del ecosistema sin recurrir a Google:

- **GitHub Codespaces:** ejecuta `go run ./cmd/create-issue` y expone el puerto
  público desde Codespaces. Ideal para demostraciones o etapas tempranas.
- **Servidor propio o VPS** (puede ser administrado por tu organización):
  - Construye el binario: `go build ./cmd/create-issue`.
  - Define variables de entorno mínimas: `GITHUB_TOKEN`, `GITHUB_PROJECT_ID`,
    `ALLOWED_ORIGIN` y (opcional) `PORT`.
  - Arranca el servicio con `./create-issue` y utiliza un proxy como Nginx o
    Caddy para exponer HTTPS.
- **Contenedor en GitHub Container Registry:**
  - Crea una imagen con `docker build -t ghcr.io/<org>/create-issue:latest cmd/create-issue`.
  - Publica la imagen en GHCR y ejecútala en la infraestructura de tu elección.

En todos los casos los logs quedarán disponibles en stdout y podrán redirigirse
al mecanismo preferido (por ejemplo, archivos en disco, Loki, Fluent Bit, etc.).

### 3.3 Automatización con GitHub Actions
- Crea un workflow `ci.yml` que compile y ejecute pruebas (`go test ./...`).
- Agrega un job opcional que construya la imagen de contenedor y la publique en
  GHCR. Desde allí puedes desplegarla en tu infraestructura privada.
- Usa secretos del repositorio para almacenar `GITHUB_TOKEN` (fine-grained) y el
  ID del proyecto (`GITHUB_PROJECT_ID`).

### 3.4 Gestión del roadmap
- Los Issues creados por el servicio se agregan automáticamente al Project v2.
- Usa Vistas personalizadas en Projects para priorizar, agrupar y comunicar.
- Publica enlaces desde GitHub Pages hacia vistas específicas del Project para
  mantener alineadas a todas las personas interesadas.

## 4. Pasos sugeridos para la migración

1. **Revisar tokens:** genera un token clásico o fine-grained desde GitHub con
   permisos `repo` y `project`.
2. **Actualizar configuraciones existentes:** elimina variables específicas de
   Google Cloud de tus despliegues y asegúrate de establecer `LOGGING_PROJECT_ID`
   vacío para activar el backend de stdout.
3. **Configurar monitoreo básico:** guarda los logs producidos por stdout en el
   sistema que uses (por ejemplo, GitHub Actions artifacts o un agente de
   logging propio).
4. **Documentar el nuevo flujo:** comparte este documento con el equipo para que
   sepan que todo el roadmap se gestiona únicamente con herramientas de GitHub.

Con estos pasos, el proyecto mantiene su transparencia y automatización usando
exclusivamente la plataforma de GitHub y sin depender de servicios de Google.
