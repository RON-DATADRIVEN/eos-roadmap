package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func preserveOriginGlobals(t *testing.T) func() {
	t.Helper()
	previousAllowAny := allowAnyOrigin
	previousAllowed := allowedOrigin
	previousEntries := allowedOriginEntries

	return func() {
		allowAnyOrigin = previousAllowAny
		allowedOrigin = previousAllowed
		allowedOriginEntries = previousEntries
	}
}

func preserveRequestLogger(t *testing.T) func() {
	t.Helper()
	previousBackend := requestLogBackend
	previousIssueCreator := issueCreator
	previousProjectAdder := projectAdder

	return func() {
		requestLogBackend = previousBackend
		issueCreator = previousIssueCreator
		projectAdder = previousProjectAdder
	}
}

type memoryLogBackend struct {
	entries []logEntry
}

func (m *memoryLogBackend) Log(_ context.Context, entry logEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *memoryLogBackend) Close() error { return nil }

func (m *memoryLogBackend) Entries() []logEntry {
	out := make([]logEntry, len(m.entries))
	copy(out, m.entries)
	return out
}

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantError bool
	}{
		{name: "https no port", input: "https://ron-datadriven.github.io", want: "https://ron-datadriven.github.io"},
		{name: "https default port", input: "https://ron-datadriven.github.io:443", want: "https://ron-datadriven.github.io"},
		{name: "http default port", input: "http://localhost:80", want: "http://localhost"},
		{name: "custom port", input: "https://example.com:8443", want: "https://example.com:8443"},
		{name: "whitespace", input: "   https://Example.com  ", want: "https://example.com"},
		{name: "invalid", input: "not-a-url", wantError: true},
		{name: "missing host", input: "https://", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOrigin(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("normalizeOrigin(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitOriginCandidates(t *testing.T) {
	input := "https://a.example.com, https://b.example.com\nhttps://c.example.com;https://d.example.com"
	want := []string{
		"https://a.example.com",
		"https://b.example.com",
		"https://c.example.com",
		"https://d.example.com",
	}

	got := splitOriginCandidates(input)
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got %d want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("element %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestSplitOriginCandidatesEmpty(t *testing.T) {
	got := splitOriginCandidates("   \n\t")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d elements", len(got))
	}
}

func TestBlankTemplateSendsExpectedLabels(t *testing.T) {
	// Definimos las etiquetas esperadas tal como deben viajar hasta GitHub,
	// evitando discrepancias entre la interfaz y el backend.
	expectedLabels := []string{"Status: Ideas", "Tipo :Blank Issue"}

	// Validamos primero que la plantilla en memoria coincide con la expectativa.
	tmpl, ok := templates["blank"]
	if !ok {
		t.Fatal("la plantilla 'blank' no existe en el mapa de plantillas")
	}
	if !reflect.DeepEqual(tmpl.Labels, expectedLabels) {
		t.Fatalf("etiquetas configuradas = %v, se esperaba %v", tmpl.Labels, expectedLabels)
	}

	// Construimos el payload mediante la función compartida con createIssue para
	// asegurarnos de que las etiquetas correctas llegan sin modificación.
	payloadBytes, err := buildIssuePayload("[ISSUE] título de prueba", tmpl.Labels, "cuerpo de prueba")
	if err != nil {
		t.Fatalf("no se pudo generar el payload: %v", err)
	}

	var payload struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("no se pudo deserializar el payload: %v", err)
	}

	if !reflect.DeepEqual(payload.Labels, expectedLabels) {
		t.Fatalf("etiquetas enviadas = %v, se esperaba %v", payload.Labels, expectedLabels)
	}
}

// roundTripperFunc permite crear implementaciones mínimas de RoundTripper a
// partir de una función, lo que simplifica capturar solicitudes en pruebas y
// reduce la probabilidad de errores humanos al escribir estructuras completas.
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip delegates the call to the callback to fulfill the interface without
// repeating logic in each test, following the poka-yoke philosophy by centralizing
// the behavior.
func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCreateIssueEnviaEtiquetasDePlantillaEnBlanco(t *testing.T) {
	// Guardamos el transporte global para restaurarlo al final y evitar que
	// otras pruebas fallen por un entorno contaminado.
	previousTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
	})

	// We define the token to prevent the function from using an empty string
	// and ensure we cover the path that adds the header.
	previousToken := githubToken
	githubToken = "token-de-prueba"
	t.Cleanup(func() {
		githubToken = previousToken
	})

	tmpl, ok := templates["blank"]
	if !ok {
		t.Fatal("the 'blank' template does not exist in the templates map")
	}

	var capturedBody []byte

	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// We read the body only once to inspect it later, avoiding consuming the stream again
		// and providing immediate feedback if something fails.
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		capturedBody = body
		if err := req.Body.Close(); err != nil {
			return nil, err
		}

		// Respondemos con un issue válido para que la función complete su
		// flujo sin errores simulando una respuesta real de GitHub.
		responseBody := `{"number": 1, "html_url": "https://example.com/issue/1", "node_id": "MDU6SXNzdWUx"}`
		resp := &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     make(http.Header),
		}
		return resp, nil
	})

	ctx := context.Background()
	if _, err := createIssue(ctx, tmpl.Title, tmpl.Labels, "example body"); err != nil {
		t.Fatalf("createIssue returned an unexpected error: %v", err)
	}

	if len(capturedBody) == 0 {
		t.Fatal("failed to capture the request body to GitHub")
	}

	var payload struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("no se pudo deserializar el payload enviado: %v", err)
	}

	if !reflect.DeepEqual(payload.Labels, tmpl.Labels) {
		t.Fatalf("sent labels = %v, expected %v", payload.Labels, tmpl.Labels)
	}
}

func TestConfigureAllowedOriginsDefaultFallback(t *testing.T) {
	restore := preserveOriginGlobals(t)
	defer restore()

	allowAnyOrigin = false
	allowedOrigin = ""

	entries := configureAllowedOrigins("", "https://ron-datadriven.github.io")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].normalized != "https://ron-datadriven.github.io" {
		t.Fatalf("unexpected normalized origin: %q", entries[0].normalized)
	}
}

func TestConfigureAllowedOriginsWildcard(t *testing.T) {
	restore := preserveOriginGlobals(t)
	defer restore()

	allowAnyOrigin = false
	allowedOrigin = ""

	entries := configureAllowedOrigins("*", "https://fallback.example")

	if !allowAnyOrigin {
		t.Fatal("allowAnyOrigin should be true")
	}

	if entries != nil {
		t.Fatalf("entries should be nil when wildcard is enabled")
	}
}

func TestConfigureAllowedOrigins(t *testing.T) {
	const fallbackOrigin = "https://fallback.example"

	tests := []struct {
		name         string
		envVar       string
		wantOrigins  []string
		wantWildcard bool
	}{
		{
			name:        "env var and fallback",
			envVar:      "https://a.example.com,https://b.example.com",
			wantOrigins: []string{"https://a.example.com", "https://b.example.com", fallbackOrigin},
		},
		{
			name:        "env var with duplicates",
			envVar:      "https://a.example.com, https://a.example.com",
			wantOrigins: []string{"https://a.example.com", fallbackOrigin},
		},
		{
			name:        "env var with invalid and valid",
			envVar:      "invalid-origin, https://a.example.com",
			wantOrigins: []string{"https://a.example.com", fallbackOrigin},
		},
		{
			name:        "env var empty with fallback",
			envVar:      " ",
			wantOrigins: []string{fallbackOrigin},
		},
		{
			name:         "wildcard takes precedence",
			envVar:       "https://a.example.com, *",
			wantOrigins:  nil,
			wantWildcard: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := preserveOriginGlobals(t)
			defer restore()

			allowAnyOrigin = false
			allowedOrigin = ""

			entries := configureAllowedOrigins(tt.envVar, fallbackOrigin)

			if allowAnyOrigin != tt.wantWildcard {
				t.Fatalf("allowAnyOrigin = %v, want %v", allowAnyOrigin, tt.wantWildcard)
			}

			if tt.wantWildcard {
				if len(entries) != 0 {
					t.Fatalf("expected no entries for wildcard, got %d", len(entries))
				}
				return
			}

			gotOrigins := make([]string, len(entries))
			for i, e := range entries {
				gotOrigins[i] = e.normalized
			}

			sort.Strings(gotOrigins)
			sort.Strings(tt.wantOrigins)

			if !reflect.DeepEqual(gotOrigins, tt.wantOrigins) {
				t.Fatalf("allowed origins mismatch:\ngot:  %v\nwant: %v", gotOrigins, tt.wantOrigins)
			}
		})
	}
}

func TestIsOriginAllowed(t *testing.T) {
	restore := preserveOriginGlobals(t)
	defer restore()

	allowedOriginEntries = configureAllowedOrigins("https://a.example.com, https://b.example.com", "https://default.example")

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"allowed custom", "https://a.example.com", true},
		{"allowed default", "https://default.example", true},
		{"denied", "https://c.example.com", false},
		{"subdomain not allowed", "https://sub.a.example.com", false},
		{"empty origin", "", false},
		{"malformed origin", "http//bad", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOriginAllowed(tt.origin); got != tt.want {
				t.Fatalf("isOriginAllowed(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestDenyOrigin(t *testing.T) {
	rr := httptest.NewRecorder()
	denyOrigin(context.Background(), rr, "https://unauthorized.example.com")

	resp := rr.Result()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}

	if h := resp.Header.Get("Access-Control-Allow-Origin"); h != "" {
		t.Errorf("expected empty Access-Control-Allow-Origin, got %q", h)
	}
}

func TestHandleCORSRejectsWhenNoOriginsConfigured(t *testing.T) {
	t.Helper()

	// Explicamos que restauramos los valores globales para no afectar a otras pruebas,
	// igual que haría una persona que ordena su espacio de trabajo antes de comenzar.
	restore := preserveOriginGlobals(t)
	defer restore()

	// Dejamos el sistema sin orígenes permitidos, representando un despliegue con
	// configuración vacía o dañada. Lo hacemos manualmente para imitar el fallo
	// original incluso después de mejorar la lógica de respaldo.
	allowAnyOrigin = false
	allowedOrigin = ""
	allowedOriginEntries = nil

	// Construimos una petición desde el dominio público actual para validar que la
	// respuesta sea de rechazo y así detectar el problema original.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	req.Header.Set("Origin", "https://ron-datadriven.github.io")

	if handleCORS(context.Background(), rr, req) {
		t.Fatalf("expected handleCORS to reject origin when configuration is empty")
	}

	resp := rr.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestHandleRequestCORSPreflightAndPost(t *testing.T) {
	t.Helper()

	restoreOrigins := preserveOriginGlobals(t)
	defer restoreOrigins()

	restoreLogger := preserveRequestLogger(t)
	defer restoreLogger()

	// Configuramos los orígenes permitidos incluyendo el dominio público, evitando
	// depender de variables de entorno implícitas dentro de la prueba.
	allowAnyOrigin = false
	allowedOriginEntries = configureAllowedOrigins(defaultAllowedOrigin, defaultAllowedOrigin)

	// Reemplazamos las funciones externas para observar que handlePost se ejecuta
	// sin invocar servicios reales. Guardamos banderas para detectar la llamada.
	postCalled := false
	projectCalled := false
	issueCreator = func(context.Context, string, []string, string) (*githubIssueResponse, error) {
		postCalled = true
		return &githubIssueResponse{Number: 7, HTMLURL: "https://example.com/issues/7", NodeID: "node-7"}, nil
	}
	projectAdder = func(context.Context, string) error {
		projectCalled = true
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(handleRequest))
	defer server.Close()

	client := server.Client()

	allowedOrigin := defaultAllowedOrigin

	// Simulamos primero la petición de preflight que ejecuta el navegador antes
	// del POST real. Incluimos el encabezado solicitado en minúsculas para
	// replicar lo que observamos en producción.
	preflightReq, err := http.NewRequest(http.MethodOptions, server.URL+"/", nil)
	if err != nil {
		t.Fatalf("no se pudo crear la solicitud OPTIONS: %v", err)
	}
	preflightReq.Header.Set("Origin", allowedOrigin)
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightReq.Header.Set("Access-Control-Request-Headers", "content-type")

	preflightResp, err := client.Do(preflightReq)
	if err != nil {
		t.Fatalf("error ejecutando preflight: %v", err)
	}
	defer preflightResp.Body.Close()

	if preflightResp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, se esperaba %d", preflightResp.StatusCode, http.StatusNoContent)
	}

	if got := preflightResp.Header.Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin preflight = %q, se esperaba %q", got, allowedOrigin)
	}

	if got := preflightResp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods no incluye POST: %q", got)
	}

	if got := preflightResp.Header.Get("Access-Control-Allow-Headers"); !headerListContains(got, "content-type") {
		t.Fatalf("Access-Control-Allow-Headers no contiene content-type: %q", got)
	}

	if postCalled {
		t.Fatalf("handlePost no debe ejecutarse durante el preflight")
	}

	// Repetimos ahora la solicitud real para comprobar que la ruta POST llega a
	// handlePost y que los encabezados de CORS acompañan la respuesta.
	type postRequestBody struct {
		TemplateID string                 `json:"templateId"`
		Title      string                 `json:"title"`
		Fields     map[string]interface{} `json:"fields"`
	}
	reqBody := postRequestBody{
		TemplateID: "blank",
		Title:      "Ejemplo",
		Fields:     map[string]interface{}{"descripcion": "Texto"},
	}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("no se pudo serializar el cuerpo JSON: %v", err)
	}
	body := strings.NewReader(string(jsonBytes))
	postReq, err := http.NewRequest(http.MethodPost, server.URL+"/", body)
	if err != nil {
		t.Fatalf("no se pudo crear la solicitud POST: %v", err)
	}
	postReq.Header.Set("Origin", allowedOrigin)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := client.Do(postReq)
	if err != nil {
		t.Fatalf("error ejecutando POST: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("status del POST = %d, se esperaba %d", postResp.StatusCode, http.StatusOK)
	}

	if got := postResp.Header.Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin en POST = %q, se esperaba %q", got, allowedOrigin)
	}

	if got := postResp.Header.Get("Access-Control-Allow-Headers"); !headerListContains(got, "content-type") {
		t.Fatalf("Access-Control-Allow-Headers en POST no contiene content-type: %q", got)
	}

	if !postCalled {
		t.Fatalf("handlePost no fue invocado tras el POST")
	}

	if !projectCalled {
		t.Fatalf("projectAdder no fue invocado tras el POST")
	}
}

func TestHandleRequestCORSForbiddenOrigin(t *testing.T) {
	t.Helper()

	restoreOrigins := preserveOriginGlobals(t)
	defer restoreOrigins()

	restoreLogger := preserveRequestLogger(t)
	defer restoreLogger()

	allowAnyOrigin = false
	allowedOriginEntries = configureAllowedOrigins(defaultAllowedOrigin, defaultAllowedOrigin)

	postCalled := false
	issueCreator = func(context.Context, string, []string, string) (*githubIssueResponse, error) {
		postCalled = true
		return nil, nil
	}
	projectAdder = func(context.Context, string) error { return nil }

	server := httptest.NewServer(http.HandlerFunc(handleRequest))
	defer server.Close()

	client := server.Client()

	body := strings.NewReader("{\"templateId\":\"blank\",\"title\":\"Ejemplo\",\"fields\":{\"descripcion\":\"Texto\"}}")
	req, err := http.NewRequest(http.MethodPost, server.URL+"/", body)
	if err != nil {
		t.Fatalf("no se pudo crear la solicitud POST: %v", err)
	}
	req.Header.Set("Origin", "https://bloqueado.example")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("error ejecutando POST bloqueado: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status del POST bloqueado = %d, se esperaba %d", resp.StatusCode, http.StatusForbidden)
	}

	var payload issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("no se pudo leer la respuesta JSON: %v", err)
	}

	if payload.Error == nil || payload.Error.Code != "forbidden_origin" {
		t.Fatalf("el JSON de error no coincide: %+v", payload.Error)
	}

	if postCalled {
		t.Fatalf("handlePost no debe ejecutarse cuando el origen está bloqueado")
	}
}

// headerListContains revisa listas de encabezados separadas por comas ignorando el
// uso de mayúsculas, evitando que pequeñas diferencias de formato provoquen falsos
// negativos en las comprobaciones de CORS.
func headerListContains(raw string, target string) bool {
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), target) {
			return true
		}
	}
	return false
}

func TestRequestLoggerCapturesSuccessfulPost(t *testing.T) {
	t.Helper()

	restoreOrigins := preserveOriginGlobals(t)
	defer restoreOrigins()

	restoreLogger := preserveRequestLogger(t)
	defer restoreLogger()

	allowAnyOrigin = true
	allowedOriginEntries = nil

	fakeBackend := &memoryLogBackend{}
	requestLogBackend = fakeBackend

	issueCreator = func(context.Context, string, []string, string) (*githubIssueResponse, error) {
		// Entregamos datos estáticos para que la prueba se enfoque en el logging
		// y no dependa de GitHub.
		return &githubIssueResponse{Number: 1, HTMLURL: "https://example.com/issue/1", NodeID: "node-1"}, nil
	}
	projectAdder = func(context.Context, string) error { return nil }

	body := strings.NewReader("{\"templateId\":\"blank\",\"title\":\"Nuevo módulo\",\"fields\":{\"descripcion\":\"Detalle\"}}")
	req := httptest.NewRequest(http.MethodPost, "http://service.local/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://allowed.example")

	rr := httptest.NewRecorder()
	handleRequest(rr, req)

	resp := rr.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}
	if payload.DebugID == "" {
		t.Fatalf("expected debugId in response payload")
	}

	entries := fakeBackend.Entries()
	if len(entries) < 2 {
		t.Fatalf("expected at least two log entries, got %d", len(entries))
	}

	var finishEntry logEntry
	var finishFound bool
	var startEntry logEntry
	var startFound bool
	for _, entry := range entries {
		switch entry.Stage {
		case "start":
			if !startFound {
				startEntry = entry
				startFound = true
			}
		case "finish":
			if !finishFound {
				finishEntry = entry
				finishFound = true
			}
		}
	}

	if !startFound {
		t.Fatalf("start entry not found in log entries: %+v", entries)
	}
	if startEntry.Timestamp.IsZero() {
		t.Fatalf("start entry should include a timestamp")
	}
	if startEntry.Method != http.MethodPost {
		t.Fatalf("start entry method = %s, want %s", startEntry.Method, http.MethodPost)
	}
	if startEntry.Path != "/" {
		t.Fatalf("start entry path = %s, want /", startEntry.Path)
	}
	if startEntry.Origin != "https://allowed.example" {
		t.Fatalf("start entry origin = %s, want https://allowed.example", startEntry.Origin)
	}

	if !finishFound {
		t.Fatalf("finish entry not found in log entries: %+v", entries)
	}
	if finishEntry.Status != http.StatusOK {
		t.Fatalf("finish status = %d, want %d", finishEntry.Status, http.StatusOK)
	}
	if finishEntry.ErrorCode != "" {
		t.Fatalf("finish entry error code = %q, want empty", finishEntry.ErrorCode)
	}
	if finishEntry.TemplateID != "blank" {
		t.Fatalf("finish entry template = %s, want blank", finishEntry.TemplateID)
	}
	if finishEntry.RequestID != payload.DebugID {
		t.Fatalf("finish entry requestId = %s, want %s", finishEntry.RequestID, payload.DebugID)
	}
	if finishEntry.Timestamp.IsZero() {
		t.Fatalf("finish entry should include timestamp")
	}
}

func TestRequestLoggerCapturesCORSRejection(t *testing.T) {
	t.Helper()

	restoreOrigins := preserveOriginGlobals(t)
	defer restoreOrigins()

	restoreLogger := preserveRequestLogger(t)
	defer restoreLogger()

	allowAnyOrigin = false
	allowedOriginEntries = nil
	allowedOrigin = ""

	fakeBackend := &memoryLogBackend{}
	requestLogBackend = fakeBackend

	body := strings.NewReader("{\"templateId\":\"blank\"}")
	req := httptest.NewRequest(http.MethodPost, "http://service.local/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://blocked.example")

	rr := httptest.NewRecorder()
	handleRequest(rr, req)

	resp := rr.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", resp.StatusCode)
	}

	var payload issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}
	if payload.DebugID == "" {
		t.Fatalf("expected debugId in response payload")
	}

	entries := fakeBackend.Entries()
	if len(entries) < 2 {
		t.Fatalf("expected at least two log entries, got %d", len(entries))
	}

	var errorEntry logEntry
	var errorFound bool
	var finishEntry logEntry
	var finishFound bool
	for _, entry := range entries {
		switch entry.Stage {
		case "error":
			if entry.ErrorCode == "forbidden_origin" && !errorFound {
				errorEntry = entry
				errorFound = true
			}
		case "finish":
			if !finishFound {
				finishEntry = entry
				finishFound = true
			}
		}
	}

	if !errorFound {
		t.Fatalf("error entry with code forbidden_origin not found: %+v", entries)
	}
	if errorEntry.Status != http.StatusForbidden {
		t.Fatalf("error entry status = %d, want %d", errorEntry.Status, http.StatusForbidden)
	}
	if errorEntry.Origin != "https://blocked.example" {
		t.Fatalf("error entry origin = %s, want https://blocked.example", errorEntry.Origin)
	}
	if errorEntry.Method != http.MethodPost {
		t.Fatalf("error entry method = %s, want %s", errorEntry.Method, http.MethodPost)
	}
	if errorEntry.Timestamp.IsZero() {
		t.Fatalf("error entry should include timestamp")
	}

	if !finishFound {
		t.Fatalf("finish entry not found in log entries: %+v", entries)
	}
	if finishEntry.Status != http.StatusForbidden {
		t.Fatalf("finish status = %d, want %d", finishEntry.Status, http.StatusForbidden)
	}
	if finishEntry.ErrorCode != "forbidden_origin" {
		t.Fatalf("finish entry error code = %s, want forbidden_origin", finishEntry.ErrorCode)
	}
	if finishEntry.RequestID != payload.DebugID {
		t.Fatalf("finish entry requestId = %s, want %s", finishEntry.RequestID, payload.DebugID)
	}
	if finishEntry.TemplateID != "" {
		t.Fatalf("finish entry template = %s, want empty", finishEntry.TemplateID)
	}
}
