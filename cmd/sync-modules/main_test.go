package main

import (
	"testing"
)

func TestNormalizeStatus(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantStatus string
		wantPct    int
	}{
		{"vacío", "", "Planificado", 0},
		{"planificado exacto", "Planificado", "Planificado", 0},
		{"planificado min", "planificación", "Planificado", 0},
		{"curso exacto", "en curso", "En curso", 50},
		{"curso ingles", "in progress", "En curso", 50},
		{"curso raiz", "está en progreso ahora", "En curso", 50},
		{"hecho exacto", "Hecho", "Hecho", 100},
		{"hecho ingles", "done", "Hecho", 100},
		{"hecho deploy", "deployment", "Hecho", 100},
		{"desconocido", "algo raro", "Planificado", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotPct := normalizeStatus(tc.raw)
			if gotStatus != tc.wantStatus || gotPct != tc.wantPct {
				t.Errorf("normalizeStatus(%q) = (%q, %d); want (%q, %d)", tc.raw, gotStatus, gotPct, tc.wantStatus, tc.wantPct)
			}
		})
	}
}

func TestCalculatePercentage(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		baseline int
		want     int
	}{
		{"sin directiva ni checklist", "solo texto", 50, 50},
		{"directiva manual", "texto\nProgress: 75%\nmas texto", 50, 75},
		{"directiva manual min/max", "Progress: 150%", 50, 100},
		{"directiva manual min/max neg", "Progress: -10%", 50, 0},
		{"checklist 0/2", "- [ ] Tarea 1\n- [ ] Tarea 2", 10, 0},
		{"checklist 1/2", "- [ ] Tarea 1\n- [x] Tarea 2", 10, 50},
		{"checklist 2/2", "- [X] Tarea 1\n- [x] Tarea 2", 10, 100},
		{"checklist con espacios raros", "-  [ ] Tarea 1\n- [ x ] Tarea 2", 10, 50},
		{"ambos, directiva gana", "- [ ] T1\nProgress: 80%", 10, 80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := calculatePercentage(tc.body, tc.baseline)
			if got != tc.want {
				t.Errorf("calculatePercentage() = %d; want %d", got, tc.want)
			}
		})
	}
}

func TestDetectTipo(t *testing.T) {
	cases := []struct {
		name          string
		title         string
		labels        []string
		projectFields map[string]string
		want          string
	}{
		{"por project field epic", "algo", nil, map[string]string{"Tipo": "epic"}, "epic"},
		{"por project field bug", "algo", nil, map[string]string{"Tipo": "bug"}, "bug"},
		{"por label epic", "algo", []string{"tipo:epic"}, nil, "epic"},
		{"por label bug", "algo", []string{"tipo:bug"}, nil, "bug"},
		{"por titulo epic", "[ÉPICA] Gran feature", nil, nil, "epic"},
		{"por titulo epic ingles", "[EPIC] Huge feature", nil, nil, "epic"},
		{"por titulo bug", "[BUG] Error 500", nil, nil, "bug"},
		{"nada", "Tarea normal", []string{"enhancement"}, nil, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectTipo(tc.title, tc.labels, tc.projectFields)
			if got != tc.want {
				t.Errorf("detectTipo() = %q; want %q", got, tc.want)
			}
		})
	}
}
