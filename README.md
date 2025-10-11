# EOS – Roadmap público

Repositorio para **planificación visible** del proyecto EOS y su **página pública** (GitHub Pages) con módulos, estado y próximos hitos.

## ¿Cómo actualizar el roadmap visible?
1. Edita `docs/modules.json` (agrega/actualiza módulos, estado y progreso).
2. Commit a `main`. GitHub Pages publica automáticamente.
3. Comparte el enlace con stakeholders.

## Plantillas
- Issues: Feature, Bug, Change Request.
- Pull Request template + CODEOWNERS.

## 🌐 Página pública
https://ron-datadriven.github.io/eos-roadmap/

## 🔧 New Issues
https://github.com/RON-DATADRIVEN/eos-roadmap/issues/new/choose

## ☁️ Servicio `create-issue`
El comando `cmd/create-issue` expone un endpoint HTTP pensado para Cloud Run/Functions. Recibe el template seleccionado desde el modal, crea el issue en GitHub y lo agrega automáticamente al Project EOS 2.0 mediante GraphQL.

### Variables de entorno
- `GITHUB_TOKEN`: token con permisos `repo` y `project` sobre `RON-DATADRIVEN/eos-roadmap`.
- `GITHUB_PROJECT_ID`: identificador del ProjectV2 (por ejemplo, el ID de EOS 2.0).
- `ALLOWED_ORIGIN`: dominio HTTPS permitido para CORS (ej. `https://ron-datadriven.github.io`).
- `PORT`: opcional, puerto de escucha cuando se ejecuta localmente.

### Despliegue en Cloud Run
1. Construye la imagen: `gcloud builds submit --tag gcr.io/<project-id>/create-issue cmd/create-issue`.
2. Despliega: `gcloud run deploy create-issue --image gcr.io/<project-id>/create-issue --region <region> --allow-unauthenticated --set-env-vars ALLOWED_ORIGIN=https://ron-datadriven.github.io,GITHUB_PROJECT_ID=<project-id> --set-secrets GITHUB_TOKEN=github-token:latest`.
3. Define el secreto `github-token` en Secret Manager (rotación automática recomendada) antes del despliegue.

### Integración con el modal
- Define la URL del servicio en GitHub Pages usando el atributo `data-issue-service-url` del elemento `<html>` o asignando `window.ISSUE_SERVICE_URL` antes de cargar el script.
- El modal nunca expone el token; solo envía título y campos normalizados al backend.
- Los mensajes del modal reflejan el estado (envío, reintentos, advertencias si el Project no se actualiza).

