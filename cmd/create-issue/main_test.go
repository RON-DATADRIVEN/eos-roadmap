package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestHandleCORSForbiddenOrigin(t *testing.T) {
	restore := preserveOriginGlobals(t)
	defer restore()

	allowAnyOrigin = false
	allowedOriginEntries = configureAllowedOrigins("https://allowed.example", defaultAllowedOrigin)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://not-allowed.example")

	allowed := handleCORS(rr, req)
	if allowed {
		t.Fatalf("expected handleCORS to reject origin")
	}

	if rr.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: got %d", rr.Code)
	}

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://not-allowed.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, "https://not-allowed.example")
	}

	var resp issueResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if resp.Error == nil || resp.Error.Code != "forbidden_origin" {
		t.Fatalf("expected forbidden_origin error, got %+v", resp.Error)
	}

	if resp.Error.Message == "" {
		t.Fatal("expected error message to be populated")
	}
}
