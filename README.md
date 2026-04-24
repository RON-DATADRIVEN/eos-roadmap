# EOS – Roadmap público

Repositorio para **planificación visible** del proyecto EOS y su **página pública** (GitHub Pages) con módulos, estado y próximos hitos.

## ¿Cómo actualizar el roadmap visible?

1. Edita `docs/modules.json` (agrega/actualiza módulos, estado y progreso) o utiliza el flujo de GitHub Projects. El workflow `.github/workflows/sync-modules.yml` sincroniza el archivo automáticamente cada 5 minutos usando el secreto `PROJECTS_TOKEN`.

- **Atención (Rotación de Token):** `PROJECTS_TOKEN` es un Personal Access Token. Debido a las políticas de seguridad, tiene fecha de caducidad. Debe configurarse una alarma en calendario para rotarlo antes de que expire, de lo contrario, la sincronización fallará silenciosamente.
1. Commit manual a `main` si no usas Projects. GitHub Pages publica automáticamente.
2. Comparte el enlace con stakeholders.

### Tipos de módulo permitidos

- `epic`: iniciativas grandes que agrupan varias tareas o bugs relacionados.
- `bug`: incidencias detectadas en producción o QA que requieren seguimiento específico.

Mantén estos valores al agregar o actualizar el campo `tipo` en `docs/modules.json` para que el generador de datos y la vista pública permanezcan sincronizados.

## Plantillas

- Issues: Feature, Bug, Change Request.
- Pull Request template + CODEOWNERS.

## 🌐 Página pública
<https://ron-datadriven.github.io/eos-roadmap/>

## 🔧 New Issues
<https://github.com/RON-DATADRIVEN/eos-roadmap/issues/new/choose>

## ☁️ Servicio `create-issue`

El comando `cmd/create-issue` expone un endpoint HTTP pensado para Cloud Run/Functions. Recibe el template seleccionado desde el modal, crea el issue en GitHub y lo agrega automáticamente al Project EOS 2.0 mediante GraphQL.

### Variables de entorno

- `GITHUB_TOKEN`: token con permisos `repo` y `project` sobre `RON-DATADRIVEN/eos-roadmap`.
- `GITHUB_PROJECT_ID`: identificador del ProjectV2 (por ejemplo, el ID de EOS 2.0).
- `ALLOWED_ORIGIN`: dominio HTTPS permitido para CORS (ej. `https://ron-datadriven.github.io`). Si la variable llega vacía el servicio
  añadirá automáticamente `https://ron-datadriven.github.io` (o la lista definida en `-ldflags "-X main.buildDefaultAllowedOrigins=..."`)
  para evitar bloqueos, pero se recomienda actualizarla siempre que cambie el dominio público.
- `PORT`: opcional, puerto de escucha cuando se ejecuta localmente.
- `LOGGING_PROJECT_ID`: opcional. Si deseas Cloud Logging indica aquí el proyecto de Google Cloud. Cuando se omite se registra todo en stdout para que GitHub Actions, Codespaces o cualquier servidor simple puedan almacenar los eventos.
- `LOGGING_LOG_ID`: opcional, nombre del log dentro de Cloud Logging. Si no se define se usa `create-issue-requests`.
- `GOOGLE_APPLICATION_CREDENTIALS`: ruta al archivo JSON del servicio con permisos `roles/logging.logWriter` para ejecuciones locales (solo necesaria si decides usar Google Cloud Logging).

### Despliegue en Cloud Run
>
> **Nota sobre el empaquetado:** Actualmente el repositorio no cuenta con un `Dockerfile`. Para despliegues en Google Cloud, el comando `gcloud builds submit` utiliza *Cloud Buildpacks* de forma transparente. Si se requiere construir la imagen localmente o en GitHub Packages, se recomienda instalar [pack](https://buildpacks.io/) y ejecutar `pack build ghcr.io/<org>/create-issue:latest --builder gcr.io/buildpacks/builder:v1`, o en su defecto, crear un `Dockerfile` estándar para Go.

1. Habilita los servicios necesarios (solo la primera vez): `gcloud services enable logging.googleapis.com run.googleapis.com`.
2. Construye la imagen: `gcloud builds submit --tag gcr.io/<project-id>/create-issue cmd/create-issue`.
3. Antes de desplegar, confirma que `ALLOWED_ORIGIN` coincida con el dominio público vigente (por ejemplo, `https://ron-datadriven.github.io`).
4. Despliega: `gcloud run deploy create-issue --image gcr.io/<project-id>/create-issue --region <region> --allow-unauthenticated --set-env-vars ALLOWED_ORIGIN=https://ron-datadriven.github.io,GITHUB_PROJECT_ID=<project-id>,LOGGING_PROJECT_ID=<gcp-project>,LOGGING_LOG_ID=create-issue-requests --set-secrets GITHUB_TOKEN=github-token:latest`.
5. Define el secreto `github-token` en Secret Manager (rotación automática recomendada) antes del despliegue y asigna al servicio de Cloud Run una cuenta con el rol `roles/logging.logWriter` para permitir el envío a Cloud Logging.

### Integración con el modal

- Define la URL del servicio en GitHub Pages usando el atributo `data-issue-service-url` del elemento `<html>` o asignando `window.ISSUE_SERVICE_URL` antes de cargar el script.
- El modal nunca expone el token; solo envía título y campos normalizados al backend.
- Los mensajes del modal reflejan el estado (envío, reintentos, advertencias si el Project no se actualiza).

### Operación sin servicios de Google

Si prefieres mantener toda la infraestructura dentro del ecosistema de GitHub, consulta `docs/operacion-solo-github.md`. Allí se detalla cómo ejecutar el backend en Actions, Codespaces o servidores propios usando únicamente GitHub Pages y Projects para la gestión del roadmap.

## 🛡️ Gobernanza y Protección de Ramas

El repositorio define reglas de protección para la rama principal en el archivo `.github/protection.json`. Estas reglas se aplican mediante herramientas externas (ej. Maintainer MCP) ya que GitHub no procesa nativamente archivos `.json` para protección de ramas.
