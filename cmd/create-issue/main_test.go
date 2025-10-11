package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
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
	expectedLabels := []string{"Status: Ideas", "Tipo: Blank Issue"}

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
	denyOrigin(rr, "https://unauthorized.example.com")

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

	if handleCORS(rr, req) {
		t.Fatalf("expected handleCORS to reject origin when configuration is empty")
	}

	resp := rr.Result()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}
