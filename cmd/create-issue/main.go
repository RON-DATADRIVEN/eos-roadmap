package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type fieldType string

const (
	fieldTypeMarkdown fieldType = "markdown"
	fieldTypeTextarea fieldType = "textarea"
	fieldTypeInput    fieldType = "input"
)

type templateField struct {
	ID       string
	Label    string
	Type     fieldType
	Required bool
	Value    string
}

type issueTemplate struct {
	ID     string
	Title  string
	Labels []string
	Body   []templateField
}

var templates = map[string]issueTemplate{
	"blank": {
		ID:    "blank",
		Title: "[ISSUE] Título",
		// Mantenemos las etiquetas exactamente como existen en GitHub para
		// evitar rechazos por diferencias mínimas (poka-yoke: prevenir errores
		// antes de que sucedan al confiar en textos iguales a los del tablero).
		Labels: []string{
			"Status: Ideas",
			"Tipo :Blank Issue",
		},
		Body: []templateField{
			{
				ID:    "descripcion",
				Label: "Descripción",
				Type:  fieldTypeTextarea,
				Value: "**Contexto**\n-\n\n**Detalles**\n-\n\n**Criterio de aceptación**\n-",
			},
		},
	},
	"bug": {
		ID:    "bug",
		Title: "fix: <resumen>",
		Labels: []string{
			"Tipo: Bug",
			"Status :En planeación",
		},
		Body: []templateField{
			{ID: "summary", Label: "Resumen", Type: fieldTypeInput, Required: true},
			{ID: "steps", Label: "Pasos para reproducir", Type: fieldTypeTextarea, Required: true},
			{ID: "expected", Label: "Comportamiento esperado", Type: fieldTypeTextarea, Required: true},
			{ID: "actual", Label: "Comportamiento actual", Type: fieldTypeTextarea, Required: true},
			{ID: "env", Label: "Entorno", Type: fieldTypeTextarea},
			{ID: "logs", Label: "Logs/evidencia", Type: fieldTypeTextarea},
		},
	},
	"change_request": {
		ID:    "change_request",
		Title: "chore: change-request <resumen>",
		Labels: []string{
			"Tipo: Change Request",
			"Status: Ideas",
		},
		Body: []templateField{
			{
				ID:    "intro",
				Label: "",
				Type:  fieldTypeMarkdown,
				Value: "Describe el cambio propuesto y el impacto (tiempo, costo, riesgo). Será evaluado.",
			},
			{ID: "description", Label: "Descripción del cambio", Type: fieldTypeTextarea, Required: true},
			{ID: "impact", Label: "Impacto (alcance/tiempo/costo/riesgo)", Type: fieldTypeTextarea, Required: true},
			{ID: "requester", Label: "Solicitante", Type: fieldTypeInput, Required: true},
		},
	},
	"feature": {
		ID:    "feature",
		Title: "[FEAT] Título de la feature",
		Labels: []string{
			"Tipo: Feature",
			"Status: Ideas",
		},
		Body: []templateField{
			{ID: "descripcion", Label: "Descripción", Type: fieldTypeTextarea, Required: true},
			{ID: "criterio", Label: "Criterio de aceptación (resumen)", Type: fieldTypeInput, Required: true},
		},
	},
}

type issueRequest struct {
	TemplateID string            `json:"templateId"`
	Title      string            `json:"title"`
	Fields     map[string]string `json:"fields"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type issueResponse struct {
	IssueURL string    `json:"issueUrl,omitempty"`
	Error    *apiError `json:"error,omitempty"`
	DebugID  string    `json:"debugId,omitempty"`
}

type githubIssueResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	NodeID  string `json:"node_id"`
}

const (
	githubRepoOwner = "RON-DATADRIVEN"
	githubRepoName  = "eos-roadmap"
	userAgent       = "eos-roadmap-create-issue/1.0"
)

const defaultAllowedOrigin = "https://ron-datadriven.github.io"

// defaultLogID define un nombre reconocible para el stream de Cloud Logging
// cuando no se especifica uno mediante variables de entorno. El nombre deja
// claro qué servicio genera los eventos para facilitar búsquedas en la
// consola de operaciones.
const defaultLogID = "create-issue-requests"

type originEntry struct {
	raw        string
	normalized string
}

var (
	githubToken   = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	projectID     = strings.TrimSpace(os.Getenv("GITHUB_PROJECT_ID"))
	allowedOrigin = strings.TrimSpace(os.Getenv("ALLOWED_ORIGIN"))
	logProjectID  = strings.TrimSpace(os.Getenv("LOGGING_PROJECT_ID"))
	logID         = strings.TrimSpace(os.Getenv("LOGGING_LOG_ID"))

	// buildDefaultAllowedOrigins permite definir, mediante flags de compilación,
	// una lista base de dominios que deben aceptarse incluso si la variable
	// ALLOWED_ORIGIN llega vacía o con valores erróneos. Al mantener el valor
	// predeterminado del sitio público, evitamos errores humanos durante un
	// despliegue apresurado.
	buildDefaultAllowedOrigins = defaultAllowedOrigin

	allowAnyOrigin       bool
	allowedOriginEntries            = configureAllowedOrigins(allowedOrigin, buildDefaultAllowedOrigins)
	requestLogBackend    logBackend = &noopLogBackend{}
)

// issueCreator y projectAdder son funciones intercambiables para facilitar el
// reemplazo en pruebas. Gracias a esto podemos simular respuestas de GitHub sin
// depender de la red, evitando sorpresas durante la automatización.
var (
	issueCreator = createIssue
	projectAdder = addToProject
)

// logBackend describe el sistema externo al que enviamos cada registro. Nos
// permite sustituir la implementación por una versión en memoria durante las
// pruebas, evitando depender de servicios remotos y reduciendo la posibilidad
// de errores humanos al ejecutar la suite.
type logBackend interface {
	Log(ctx context.Context, entry logEntry) error
	Close() error
}

// logSeverity estandariza los valores de severidad para que sean fáciles de
// convertir al formato que exige Cloud Logging.
type logSeverity string

const (
	severityInfo  logSeverity = "INFO"
	severityError logSeverity = "ERROR"
)

// logEntry resume la información mínima que necesitamos guardar por cada
// solicitud. Se serializa a JSON antes de enviarse al backend, de modo que un
// analista pueda buscar fácilmente por ID, método, plantilla o código de error.
type logEntry struct {
	Timestamp      time.Time   `json:"timestamp"`
	RequestID      string      `json:"requestId"`
	Stage          string      `json:"stage"`
	Severity       logSeverity `json:"severity"`
	Method         string      `json:"method"`
	Path           string      `json:"path"`
	Origin         string      `json:"origin"`
	TemplateID     string      `json:"templateId,omitempty"`
	Status         int         `json:"status"`
	ErrorCode      string      `json:"errorCode,omitempty"`
	Message        string      `json:"message,omitempty"`
	DurationMillis int64       `json:"durationMillis,omitempty"`
}

// noopLogBackend actúa como un respaldo seguro cuando todavía no hemos
// inicializado el cliente real. Así evitamos pánicos por punteros nulos y
// conservamos la estructura del código incluso en pruebas unitarias.
type noopLogBackend struct{}

func (n *noopLogBackend) Log(context.Context, logEntry) error { return nil }

func (n *noopLogBackend) Close() error { return nil }

// requestLogger concentra toda la información relevante de la petición en
// curso. Lleva el control del estado HTTP, la plantilla y el tiempo empleado,
// lo que nos permite detectar cuellos de botella o fallos específicos sin
// revisar manualmente los logs crudos del servidor.
type requestLogger struct {
	backend    logBackend
	requestID  string
	method     string
	path       string
	origin     string
	templateID string
	status     int
	errorCode  string
	startedAt  time.Time
}

// requestLoggerKey es la clave privada que usamos para guardar el logger en el
// contexto. Al encapsularla evitamos colisiones con otras claves y seguimos la
// práctica recomendada por Go.
type requestLoggerKey struct{}

// loggingResponseWriter envuelve al ResponseWriter original para recordar el
// último código de estado escrito. Así registramos resultados correctos o
// fallidos aunque el handler no llame explícitamente a writeResponse.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

// generateRequestID produce un identificador pseudoaleatorio siguiendo el
// formato de un UUID v4 para ayudar a la correlación entre backend y frontend.
func generateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	hexValue := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:],
	)
}

// newRequestLogger crea un identificador único para la petición, guarda los
// metadatos básicos y genera una entrada "start" en el backend para señalar el
// comienzo del procesamiento.
func newRequestLogger(ctx context.Context, backend logBackend, r *http.Request) *requestLogger {
	requestID := generateRequestID()
	logger := &requestLogger{
		backend:   backend,
		requestID: requestID,
		method:    r.Method,
		path:      r.URL.Path,
		origin:    strings.TrimSpace(r.Header.Get("Origin")),
		startedAt: time.Now().UTC(),
	}

	logger.log(ctx, "start", severityInfo, "inicio de procesamiento")
	return logger
}

// Attach guarda el logger dentro del contexto para que funciones auxiliares lo
// consulten sin necesidad de parámetros adicionales. Esto reduce errores al
// propagar manualmente referencias entre capas.
func (rl *requestLogger) Attach(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestLoggerKey{}, rl)
}

// ID expone el identificador único para que el frontend pueda mostrarlo cuando
// se comunique un error genérico.
func (rl *requestLogger) ID() string {
	return rl.requestID
}

// SetTemplate almacena la plantilla solicitada, permitiendo correlacionar
// errores con un formulario específico.
func (rl *requestLogger) SetTemplate(templateID string) {
	rl.templateID = strings.TrimSpace(templateID)
}

// RecordStatus memoriza el código HTTP que enviaremos al cliente. Preferimos
// llevarlo aquí para que la salida "finish" del log tenga el dato incluso si el
// flujo termina en varios puntos diferentes.
func (rl *requestLogger) RecordStatus(status int) {
	rl.status = status
}

// RecordError guarda el código lógico del error, facilitando el filtrado en
// paneles o alertas.
func (rl *requestLogger) RecordError(code string) {
	rl.errorCode = strings.TrimSpace(code)
}

// LogError envía una entrada adicional con severidad alta cuando una operación
// relevante falla (por ejemplo, CORS, GitHub REST o GraphQL). Incluimos el
// mensaje original y el error concreto para reducir la investigación manual.
func (rl *requestLogger) LogError(ctx context.Context, code, message string, err error) {
	rl.RecordError(code)
	errorMessage := message
	if err != nil {
		errorMessage = fmt.Sprintf("%s: %v", message, err)
	}
	if rl.status == 0 {
		rl.status = http.StatusInternalServerError
	}
	rl.log(ctx, "error", severityError, errorMessage)
}

// Finish debe llamarse al cerrar la petición. Calcula la duración total y
// envía un último registro con el estado final, lo que simplifica detectar si
// un error ya fue devuelto al cliente.
func (rl *requestLogger) Finish(ctx context.Context) {
	duration := time.Since(rl.startedAt)
	entry := logEntry{
		DurationMillis: duration.Milliseconds(),
	}
	rl.logWithEntry(ctx, "finish", severityInfo, "fin de procesamiento", entry)
}

// log es un envoltorio que arma la estructura común para cada evento antes de
// delegar en el backend.
func (rl *requestLogger) log(ctx context.Context, stage string, severity logSeverity, message string) {
	rl.logWithEntry(ctx, stage, severity, message, logEntry{})
}

func (rl *requestLogger) logWithEntry(ctx context.Context, stage string, severity logSeverity, message string, entry logEntry) {
	if rl.backend == nil {
		return
	}

	entry.Timestamp = time.Now().UTC()
	entry.RequestID = rl.requestID
	entry.Stage = stage
	entry.Severity = severity
	entry.Method = rl.method
	entry.Path = rl.path
	entry.Origin = rl.origin
	entry.TemplateID = rl.templateID
	entry.Status = rl.status
	entry.ErrorCode = rl.errorCode
	entry.Message = message

	if err := rl.backend.Log(ctx, entry); err != nil {
		log.Printf("no se pudo registrar en el backend de logs: %v", err)
	}
}

// loggerFromContext recupera el requestLogger asociado a la petición actual.
func loggerFromContext(ctx context.Context) *requestLogger {
	if ctx == nil {
		return nil
	}
	rl, _ := ctx.Value(requestLoggerKey{}).(*requestLogger)
	return rl
}

// cloudLoggingBackend envía cada registro mediante la API REST de Cloud
// Logging. Implementamos la autenticación manual para evitar dependencias
// pesadas y mantener el control sobre los errores que reportamos al operador.
type cloudLoggingBackend struct {
	projectID string
	logName   string
	client    *http.Client

	tokenMu sync.Mutex
	token   string
	expiry  time.Time
}

const loggingEndpoint = "https://logging.googleapis.com/v2/entries:write"
const metadataTokenURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

// newCloudLoggingBackend inicializa la estructura y valida los parámetros. Al
// fallar devolvemos un error explícito para que el operador corrija credenciales
// o permisos antes de iniciar el servicio.
func newCloudLoggingBackend(ctx context.Context, projectID, logName string) (logBackend, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("projectID vacío para logging")
	}
	if strings.TrimSpace(logName) == "" {
		logName = defaultLogID
	}

	escapedLogID := url.PathEscape(logName)
	fullLogName := fmt.Sprintf("projects/%s/logs/%s", projectID, escapedLogID)

	return &cloudLoggingBackend{
		projectID: projectID,
		logName:   fullLogName,
		client:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *cloudLoggingBackend) Log(ctx context.Context, entry logEntry) error {
	token, err := c.ensureToken(ctx)
	if err != nil {
		return fmt.Errorf("no se pudo obtener token para logging: %w", err)
	}

	payload := map[string]any{
		"logName": c.logName,
		"resource": map[string]any{
			"type": "global",
		},
		"entries": []map[string]any{
			{
				"jsonPayload": entry,
				"severity":    string(entry.Severity),
				"timestamp":   entry.Timestamp.Format(time.RFC3339Nano),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("no se pudo serializar entrada de logging: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loggingEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("no se pudo crear solicitud de logging: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("error al llamar a Cloud Logging: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Cloud Logging devolvió %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	return nil
}

func (c *cloudLoggingBackend) ensureToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Until(c.expiry) > time.Minute {
		return c.token, nil
	}

	token, expiry, err := fetchToken(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	c.expiry = expiry
	return c.token, nil
}

func (c *cloudLoggingBackend) Close() error { return nil }

// fetchToken intenta primero obtener un token mediante metadata y, si falla,
// recurre a las credenciales locales definidas por el operador.
func fetchToken(ctx context.Context) (string, time.Time, error) {
	if token, expiry, err := fetchTokenFromMetadata(ctx); err == nil {
		return token, expiry, nil
	}
	log.Printf("no se pudo obtener token de metadata: %v", err)

	credentialsPath := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if credentialsPath == "" {
		return "", time.Time{}, errors.New("GOOGLE_APPLICATION_CREDENTIALS no definido y metadata inaccesible")
	}

	return fetchTokenFromCredentials(ctx, credentialsPath)
}

// fetchTokenFromMetadata utiliza el servidor de metadata disponible en Cloud
// Run/Compute Engine para generar un token delegando en la cuenta de servicio.
func fetchTokenFromMetadata(ctx context.Context) (string, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataTokenURL, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	metadataClient := &http.Client{Timeout: 2 * time.Second}
	resp, err := metadataClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", time.Time{}, fmt.Errorf("metadata status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", time.Time{}, errors.New("metadata devolvió token vacío")
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return tokenResp.AccessToken, expiry, nil
}

// fetchTokenFromCredentials lee un archivo JSON de cuenta de servicio y obtiene
// un token OAuth2 válido para escribir en Cloud Logging.
func fetchTokenFromCredentials(ctx context.Context, path string) (string, time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("no se pudo leer credenciales: %w", err)
	}

	var creds struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", time.Time{}, fmt.Errorf("formato de credenciales inválido: %w", err)
	}

	if strings.TrimSpace(creds.ClientEmail) == "" || strings.TrimSpace(creds.PrivateKey) == "" {
		return "", time.Time{}, errors.New("credenciales sin client_email o private_key")
	}

	tokenURI := strings.TrimSpace(creds.TokenURI)
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	block, _ := pem.Decode([]byte(creds.PrivateKey))
	if block == nil {
		return "", time.Time{}, errors.New("no se pudo decodificar la clave privada")
	}

	var parsedKey any
	parsedKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("clave privada con formato no soportado: %w", err)
		}
	}

	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return "", time.Time{}, errors.New("la clave privada no es RSA")
	}

	now := time.Now()
	claims := map[string]any{
		"iss":   creds.ClientEmail,
		"scope": "https://www.googleapis.com/auth/logging.write",
		"aud":   tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	header := map[string]string{"alg": "RS256", "typ": "JWT"}

	encode := func(value any) (string, error) {
		buf, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(buf), nil
	}

	encodedHeader, err := encode(header)
	if err != nil {
		return "", time.Time{}, err
	}
	encodedClaims, err := encode(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	signingInput := encodedHeader + "." + encodedClaims
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("no se pudo firmar el JWT: %w", err)
	}

	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("error al solicitar token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", time.Time{}, fmt.Errorf("token_uri devolvió %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, err
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", time.Time{}, errors.New("respuesta sin access_token")
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return tokenResp.AccessToken, expiry, nil
}

func main() {
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN no configurado")
	}
	if projectID == "" {
		log.Fatal("GITHUB_PROJECT_ID no configurado")
	}
	if logProjectID == "" {
		log.Fatal("LOGGING_PROJECT_ID no configurado")
	}
	if strings.TrimSpace(logID) == "" {
		logID = defaultLogID
	}

	ctx := context.Background()
	backend, err := newCloudLoggingBackend(ctx, logProjectID, logID)
	if err != nil {
		log.Fatalf("no se pudo inicializar Cloud Logging: %v", err)
	}
	requestLogBackend = backend
	defer func() {
		if err := backend.Close(); err != nil {
			log.Printf("error al cerrar el cliente de logging: %v", err)
		}
	}()

	if allowAnyOrigin {
		log.Print("CORS abierto: se permiten todos los orígenes (ALLOWED_ORIGIN=*)")
	} else if len(allowedOriginEntries) == 0 {
		log.Print("ADVERTENCIA: ALLOWED_ORIGIN vacío o sin valores válidos, se rechazarán solicitudes con origen")
	} else {
		log.Printf("Orígenes permitidos: %s", allowedOrigin)
	}

	http.HandleFunc("/", handleRequest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Escuchando en :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("error al iniciar servidor: %v", err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	lrw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
	ctx := r.Context()
	logger := newRequestLogger(ctx, requestLogBackend, r)
	ctx = logger.Attach(ctx)
	r = r.WithContext(ctx)

	defer func() {
		if lrw.status != 0 {
			logger.RecordStatus(lrw.status)
		}
		logger.Finish(ctx)
	}()

	if !handleCORS(ctx, lrw, r) {
		return
	}

	switch r.Method {
	case http.MethodOptions:
		logger.RecordStatus(http.StatusNoContent)
		lrw.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		handlePost(ctx, lrw, r)
	default:
		writeError(ctx, lrw, http.StatusMethodNotAllowed, "method_not_allowed", "método no permitido", nil)
	}
}

func handleCORS(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	if !isOriginAllowed(origin) {
		denyOrigin(ctx, w, origin)
		return false
	}

	if allowAnyOrigin {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "3600")
	return true
}

func denyOrigin(ctx context.Context, w http.ResponseWriter, origin string) {
	message := fmt.Sprintf("Origen no permitido: %s", origin)
	writeError(ctx, w, http.StatusForbidden, "forbidden_origin", message, nil)
}

func isOriginAllowed(origin string) bool {
	if allowAnyOrigin {
		return true
	}

	if len(allowedOriginEntries) == 0 {
		return false
	}

	normalizedOrigin, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}

	for _, entry := range allowedOriginEntries {
		if entry.normalized == normalizedOrigin {
			return true
		}
	}

	return false
}

func configureAllowedOrigins(current, fallback string) []originEntry {
	seen := map[string]struct{}{}
	var entries []originEntry

	addOrigin := func(value string, source string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}

		if value == "*" {
			allowAnyOrigin = true
			return
		}

		normalized, err := normalizeOrigin(value)
		if err != nil {
			log.Printf("origen permitido inválido ignorado (%s): %q", source, value)
			return
		}

		if _, ok := seen[normalized]; ok {
			return
		}

		entries = append(entries, originEntry{raw: value, normalized: normalized})
		seen[normalized] = struct{}{}
	}

	// Interpretamos la lista de orígenes de respaldo permitiendo separar por
	// comas o saltos de línea. Así evitamos que un error de formato deje al
	// servicio sin valores mínimos.
	fallbackCandidates := splitOriginCandidates(fallback)
	if len(fallbackCandidates) == 0 {
		// Si el operador no definió una lista personalizada, recurrimos al
		// dominio público por defecto para mantener la puerta abierta a la
		// aplicación web existente.
		fallbackCandidates = splitOriginCandidates(defaultAllowedOrigin)
	}

	for _, candidate := range fallbackCandidates {
		addOrigin(candidate, "predeterminado")
		if allowAnyOrigin {
			break
		}
	}

	if allowAnyOrigin {
		allowedOrigin = "*"
		return nil
	}

	// Procesamos las entradas suministradas en la variable de entorno, sabiendo que
	// cualquier error humano quedará registrado en el log pero no eliminará los
	// dominios seguros que ya añadimos.
	candidates := splitOriginCandidates(current)
	for _, candidate := range candidates {
		addOrigin(candidate, "ALLOWED_ORIGIN")
		if allowAnyOrigin {
			break
		}
	}

	if allowAnyOrigin {
		allowedOrigin = "*"
		return nil
	}

	if len(entries) == 0 {
		// Como última defensa, añadimos explícitamente el dominio público
		// conocido. Esto evita que un error al construir la lista de respaldo
		// deje fuera al frontend que publica las peticiones.
		forcedFallback := splitOriginCandidates(defaultAllowedOrigin)
		for _, candidate := range forcedFallback {
			addOrigin(candidate, "predeterminado forzado")
			if allowAnyOrigin {
				break
			}
		}
	}

	if allowAnyOrigin {
		allowedOrigin = "*"
		return nil
	}

	if len(entries) == 0 {
		allowedOrigin = ""
		return nil
	}

	rawOrigins := make([]string, 0, len(entries))
	for _, entry := range entries {
		rawOrigins = append(rawOrigins, entry.raw)
	}
	allowedOrigin = strings.Join(rawOrigins, ",")

	return entries
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("origen %q incompleto", value)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())

	port := parsed.Port()
	if port != "" {
		if !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
			host = fmt.Sprintf("%s:%s", host, port)
		}
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

func splitOriginCandidates(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ';':
			return true
		default:
			return false
		}
	})

	cleaned := make([]string, 0, len(fields))
	for _, candidate := range fields {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		cleaned = append(cleaned, candidate)
	}

	return cleaned
}

func handlePost(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid_request", "JSON inválido", err)
		return
	}

	if logger := loggerFromContext(ctx); logger != nil {
		logger.SetTemplate(req.TemplateID)
	}

	tmpl, ok := templates[req.TemplateID]
	if !ok {
		writeError(ctx, w, http.StatusBadRequest, "invalid_template", "Plantilla no válida", nil)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(ctx, w, http.StatusBadRequest, "invalid_request", "El título es obligatorio", nil)
		return
	}

	fields := map[string]string{}
	for k, v := range req.Fields {
		fields[k] = strings.TrimSpace(v)
	}

	body, err := buildBody(tmpl, fields)
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "invalid_request", err.Error(), err)
		return
	}

	issue, err := issueCreator(ctx, title, tmpl.Labels, body)
	if err != nil {
		if logger := loggerFromContext(ctx); logger != nil {
			logger.LogError(ctx, "github_issue_error", "error al crear issue en GitHub", err)
		}
		writeError(ctx, w, http.StatusBadGateway, "github_issue_error", "No se pudo crear el issue en GitHub", err)
		return
	}

	err = projectAdder(ctx, issue.NodeID)
	if err != nil {
		if logger := loggerFromContext(ctx); logger != nil {
			logger.LogError(ctx, "github_project_error", fmt.Sprintf("issue #%d creado pero no se pudo agregar al proyecto", issue.Number), err)
		}
		writeResponse(ctx, w, http.StatusOK, issueResponse{
			IssueURL: issue.HTMLURL,
			Error: &apiError{
				Code:    "github_project_error",
				Message: "Issue creado pero no se pudo agregar al proyecto",
			},
		})
		return
	}

	writeResponse(ctx, w, http.StatusOK, issueResponse{IssueURL: issue.HTMLURL})
}

func buildBody(tmpl issueTemplate, fields map[string]string) (string, error) {
	var sections []string

	for _, field := range tmpl.Body {
		switch field.Type {
		case fieldTypeMarkdown:
			if strings.TrimSpace(field.Value) != "" {
				sections = append(sections, field.Value)
			}
		case fieldTypeTextarea, fieldTypeInput:
			value := strings.TrimSpace(fields[field.ID])
			if value == "" {
				if field.Required {
					return "", fmt.Errorf("El campo '%s' es obligatorio", displayLabel(field))
				}
				continue
			}
			sections = append(sections, fmt.Sprintf("### %s\n%s", displayLabel(field), value))
		default:
			return "", fmt.Errorf("Tipo de campo desconocido: %s", field.Type)
		}
	}

	return strings.TrimSpace(strings.Join(sections, "\n\n")), nil
}

func displayLabel(field templateField) string {
	if strings.TrimSpace(field.Label) == "" {
		return field.ID
	}
	return field.Label
}

func createIssue(ctx context.Context, title string, labels []string, body string) (*githubIssueResponse, error) {
	buf, err := buildIssuePayload(title, labels, body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", githubRepoOwner, githubRepoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var apiResp map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, fmt.Errorf("estado inesperado %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("estado inesperado %d: %v", resp.StatusCode, apiResp)
	}

	var issue githubIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}
	if issue.NodeID == "" {
		return nil, errors.New("respuesta sin node_id")
	}
	return &issue, nil
}

// buildIssuePayload centraliza la construcción del JSON que enviamos a GitHub, de modo
// que podamos validarlo en pruebas y evitar errores de tipeo o cambios silenciosos en
// las etiquetas.
func buildIssuePayload(title string, labels []string, body string) ([]byte, error) {
	payload := map[string]any{
		"title":  title,
		"body":   body,
		"labels": labels,
	}
	return json.Marshal(payload)
}

func addToProject(ctx context.Context, nodeID string) error {
	if strings.TrimSpace(nodeID) == "" {
		return errors.New("node_id vacío")
	}

	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	httpClient := oauth2.NewClient(ctx, src)
	gqlClient := githubv4.NewClient(httpClient)

	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: githubv4.ID(projectID),
		ContentID: githubv4.ID(nodeID),
	}

	var mutation struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID githubv4.ID
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	return gqlClient.Mutate(ctx, &mutation, input, nil)
}

func writeError(ctx context.Context, w http.ResponseWriter, status int, code, message string, cause error) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.RecordStatus(status)
		logger.LogError(ctx, code, message, cause)
	}
	writeResponse(ctx, w, status, issueResponse{Error: &apiError{Code: code, Message: message}})
}

func writeResponse(ctx context.Context, w http.ResponseWriter, status int, resp issueResponse) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.RecordStatus(status)
		if resp.Error != nil {
			logger.RecordError(resp.Error.Code)
		}
		if strings.TrimSpace(resp.DebugID) == "" {
			resp.DebugID = logger.ID()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logErrorWithFallback(ctx, "write_response_error", "error al escribir respuesta", err)
	}
}

// logErrorWithFallback logs an error using the logger from context if available, otherwise falls back to log.Printf.
func logErrorWithFallback(ctx context.Context, code, message string, err error) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.LogError(ctx, code, message, err)
	} else {
		log.Printf("%s: %s: %v", code, message, err)
	}
}
