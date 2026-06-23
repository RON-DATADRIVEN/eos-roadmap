package main

import (
	"testing"

	"github.com/shurcooL/githubv4"
)

func TestPublicFeatureStatus(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantStatus string
		wantPct    int
		wantOK     bool
	}{
		{"prototipado", "Prototipado", "En prototipo", 20, true},
		{"desarrollo", "Desarrollo", "En desarrollo", 50, true},
		{"test", "Test", "En pruebas", 75, true},
		{"staging", "Staging", "En validación", 90, true},
		{"deploy", "Deploy", "Liberado", 100, true},
		{"minúsculas", "desarrollo", "En desarrollo", 50, true},
		{"desconocido", "Ideas", "", 0, false},
		{"vacío", "", "", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotPct, gotOK := publicFeatureStatus(tc.raw)
			if gotStatus != tc.wantStatus || gotPct != tc.wantPct || gotOK != tc.wantOK {
				t.Errorf(
					"publicFeatureStatus(%q) = (%q, %d, %v); want (%q, %d, %v)",
					tc.raw,
					gotStatus,
					gotPct,
					gotOK,
					tc.wantStatus,
					tc.wantPct,
					tc.wantOK,
				)
			}
		})
	}
}

func TestPublicBugStatus(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		state      githubv4.IssueState
		wantStatus string
		wantPct    int
	}{
		{"abierto sin status", "", githubv4.IssueState("OPEN"), "Reportado", 0},
		{"abierto en prototipado", "Prototipado", githubv4.IssueState("OPEN"), "En atención", 50},
		{"abierto en desarrollo", "Desarrollo", githubv4.IssueState("OPEN"), "En atención", 50},
		{"abierto en test", "Test", githubv4.IssueState("OPEN"), "En atención", 50},
		{"abierto en staging", "Staging", githubv4.IssueState("OPEN"), "En atención", 50},
		{"deploy", "Deploy", githubv4.IssueState("OPEN"), "Resuelto", 100},
		{"cerrado", "Desarrollo", githubv4.IssueStateClosed, "Resuelto", 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotPct := publicBugStatus(tc.raw, tc.state)
			if gotStatus != tc.wantStatus || gotPct != tc.wantPct {
				t.Errorf(
					"publicBugStatus(%q, %q) = (%q, %d); want (%q, %d)",
					tc.raw,
					tc.state,
					gotStatus,
					gotPct,
					tc.wantStatus,
					tc.wantPct,
				)
			}
		})
	}
}

func TestIsBug(t *testing.T) {
	cases := []struct {
		name        string
		labels      []string
		projectTipo string
		want        bool
	}{
		{"Project Tipo bug", nil, "bug", true},
		{"Project Tipo tipo:bug", nil, "tipo:bug", true},
		{"Label bug", []string{"bug"}, "", true},
		{"Label type:bug", []string{"type:bug"}, "", true},
		{"Label tipo:bug", []string{"tipo:bug"}, "", true},
		{"Label BUG mayúsculas", []string{"BUG"}, "", true},
		{"Label buggy feature no cuenta", []string{"buggy feature"}, "", false},
		{"Label epically hard no cuenta", []string{"epically hard"}, "", false},
		{"Sin coincidencia", []string{"enhancement"}, "", false},
		{"Vacío", nil, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBug(tc.labels, tc.projectTipo)
			if got != tc.want {
				t.Errorf("isBug(%v, %q) = %v; want %v", tc.labels, tc.projectTipo, got, tc.want)
			}
		})
	}
}

func TestIsFeature(t *testing.T) {
	cases := []struct {
		name        string
		labels      []string
		projectTipo string
		want        bool
	}{
		{"Project Tipo feature", nil, "feature", true},
		{"Project Tipo tipo:feature", nil, "tipo:feature", true},
		{"Project Tipo Feature mayúscula", nil, "Feature", true},
		{"Label feature", []string{"feature"}, "", true},
		{"Label tipo feature", []string{"Tipo: Feature"}, "", true},
		{"Change Request no cuenta", nil, "Change Request", false},
		{"Blank Issue no cuenta", nil, "Blank Issue", false},
		{"Epic no cuenta", nil, "epic", false},
		{"Bug no cuenta", []string{"bug"}, "bug", false},
		{"Vacío", nil, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isFeature(tc.labels, tc.projectTipo)
			if got != tc.want {
				t.Errorf("isFeature(%v, %q) = %v; want %v", tc.labels, tc.projectTipo, got, tc.want)
			}
		})
	}
}

func TestIsLuisApproved(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"aprobado exacto", "Aprobado", true},
		{"aprobado minúsculas", "aprobado", true},
		{"aprobado con espacios", "  Aprobado  ", true},
		{"pendiente", "Pendiente", false},
		{"contexto", "Contexto", false},
		{"rechazado", "Rechazado", false},
		{"vacío", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLuisApproved(tc.raw)
			if got != tc.want {
				t.Errorf("isLuisApproved(%q) = %v; want %v", tc.raw, got, tc.want)
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
		{"directiva manual max", "Progress: 150%", 50, 100},
		{"directiva manual min", "Progress: -10%", 50, 0},
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
				t.Errorf("calculatePercentage(%q, %d) = %d; want %d", tc.body, tc.baseline, got, tc.want)
			}
		})
	}
}
