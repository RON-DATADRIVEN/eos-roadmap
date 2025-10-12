# Plan operativo permanente para `create-issue`

## Objetivo
Garantizar que el modal público de creación de issues funcione de manera estable, segura y observable, sin exponer secretos en el frontend y evitando interrupciones por CORS.

## Resumen ejecutivo
- El repositorio ya contiene el backend `cmd/create-issue`, escrito en Go, que espera ejecutarse como servicio HTTP de larga duración.
- La publicación en GitHub Pages sólo consume el endpoint definido mediante `data-issue-service-url` o `window.ISSUE_SERVICE_URL`.
- Para una operación continua se requiere un entorno permanente (Cloud Run, VPS, contenedor propio, etc.) capaz de mantener el proceso activo y exponer HTTPS.

## Fases del plan

### 1. Elegir y preparar el entorno permanente
- **Decisión de infraestructura**: seleccionar Cloud Run, un servidor administrado por la organización o un clúster/servidor de contenedores propio. Codespaces es solo temporal y se apaga al cerrar la sesión.
- **Acceso y roles**: documentar quién administra el entorno, cómo se accede (CLI, SSH, consola web) y qué permisos son necesarios para desplegar o reiniciar el servicio.

### 2. Gestionar secretos y configuración
- **Variables críticas**: `GITHUB_TOKEN` (permisos `repo` y `project`), `GITHUB_PROJECT_ID` (ID del Project V2), `ALLOWED_ORIGIN` (dominio HTTPS exacto del frontend) y opcionalmente `PORT`.
- **Almacenamiento seguro**: usar Secret Manager del proveedor elegido o el equivalente en la plataforma corporativa. Registrar un procedimiento de rotación periódica del token.
- **Scripts de arranque**: preparar unidades `systemd`, archivos `docker-compose.yml` u órdenes de despliegue (por ejemplo, `gcloud run deploy`) que exporten los secretos antes de iniciar el binario.

### 3. Automatizar construcción y despliegue
- **Pipeline CI**: ampliar GitHub Actions para ejecutar `go test ./...` y, tras pasar, construir la imagen/binario del servicio.
- **Publicación**: subir la imagen a GitHub Container Registry o desencadenar el despliegue en el entorno elegido (por ejemplo, `gcloud run deploy` o `ssh user@host 'docker compose up -d'`).
- **Versionado**: etiquetar los despliegues y llevar un registro de cambios que afecten al backend.

### 4. Configurar el host definitivo
- **Ejecución del servicio**: lanzar el binario o contenedor con las variables definidas. El backend expone únicamente la raíz `/` con métodos `POST` y `OPTIONS`.
- **HTTPS**: colocar un proxy (Nginx, Caddy, Traefik) o usar el TLS que provea la plataforma para entregar el endpoint público.
- **CORS verificado**: ejecutar un preflight (`curl -X OPTIONS -H "Origin: https://ron-datadriven.github.io"`) tras cada despliegue para confirmar que el origen queda aceptado.

### 5. Actualizar el frontend
- **Configuración**: establecer en `docs/index.html` el atributo `data-issue-service-url` con la URL pública definitiva.
- **Publicación**: hacer commit y push para que GitHub Pages regenere el sitio. Documentar el procedimiento para futuras rotaciones del backend.

### 6. Observabilidad y operación continua
- **Logs**: si no se usa Google Cloud Logging, recopilar stdout/stderr (por ejemplo, con journald, Loki o el visor de la plataforma). Los mensajes incluyen el origen, método y código de respuesta.
- **Monitoreo**: programar un chequeo recurrente que realice un preflight `OPTIONS /` con el origen autorizado para detectar fallos de red o CORS.
- **Runbooks**: registrar pasos para reinicios, rotación de tokens y resolución de errores comunes (`forbidden_origin`, `github_issue_error`, `github_project_error`).

## Consideración sobre Google Apps Script

Google Apps Script no puede sustituir al servicio `cmd/create-issue` por las siguientes razones:

1. **Modelo de ejecución**: Apps Script se ejecuta bajo demanda y no mantiene un proceso HTTP escuchando permanentemente; cada invocación crea un contenedor efímero. El modal necesita un endpoint accesible en todo momento.
2. **Restricciones de red y CORS**: los Web Apps de Apps Script aplican su propio dominio (`script.google.com`). Configurar CORS para permitir `https://ron-datadriven.github.io` requeriría cabeceras personalizadas y control fino del preflight, que no está soportado nativamente.
3. **Autenticación con GitHub**: almacenar y usar el `GITHUB_TOKEN` en Apps Script implica dejar el secreto en un servicio administrado por Google con permisos limitados sobre cabeceras y tiempo de espera. Aunque el tiempo de ejecución individual baste para una petición, la plataforma no garantiza la disponibilidad continua ni el control de cabeceras necesarias para GraphQL/REST.
4. **Observabilidad y auditoría**: el backend actual genera logs estructurados y admite integración con Cloud Logging. Apps Script ofrece registros básicos sin integración sencilla con los sistemas de monitoreo existentes.

Conclusión: aunque el tiempo de ejecución de Apps Script sea suficiente para enviar una petición a GitHub, la arquitectura del modal requiere un servicio HTTP controlado y siempre disponible. El plan anterior proporciona una ruta reproducible y segura dentro de infraestructura estándar (Cloud Run, contenedores o servidores administrados) sin exponer secretos al frontend.

## Próximos pasos sugeridos
1. Elegir el entorno permanente y documentar responsables.
2. Crear o rotar el token de GitHub y registrar los secretos en el gestor elegido.
3. Implementar el workflow de CI/CD que construya y despliegue el backend.
4. Ejecutar el primer despliegue siguiendo el plan y verificar el flujo completo desde GitHub Pages.
5. Actualizar la documentación interna con runbooks y procedimientos de monitoreo.
