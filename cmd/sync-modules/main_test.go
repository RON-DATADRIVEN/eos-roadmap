package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
)

func TestPublicFeatureStatus(t *testing.T) {
	cases := []struct {
		name       string
		phase      string
		wantStatus string
		wantPct    int
		wantOK     bool
	}{
		{"prototipado", "Prototipado", "En prototipo", 20, true},
		{"desarrollo", "Desarrollo", "En desarrollo", 50, true},
		{"test", "Test", "En pruebas", 75, true},
		{"staging", "Staging", "En validación", 90, true},
		{"deploy", "Deploy", "Liberado", 100, true},
		{"archivado", "Archivado", "Archivado", 100, true},
		{"desconocido", "Ideas", "", 0, false},
		{"vacío", "", "", 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotPct, gotOK := publicFeatureStatus(tc.phase)
			if gotStatus != tc.wantStatus || gotPct != tc.wantPct || gotOK != tc.wantOK {
				t.Errorf(
					"publicFeatureStatus(%q) = (%q, %d, %v); want (%q, %d, %v)",
					tc.phase,
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

func TestPublicPhase(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantPhase string
		wantOK    bool
	}{
		{"prototipado", "Prototipado", "Prototipado", true},
		{"desarrollo minúsculas", "desarrollo", "Desarrollo", true},
		{"test", "Test", "Test", true},
		{"staging", "Staging", "Staging", true},
		{"deploy", "Deploy", "Deploy", true},
		{"archivado", "Archivado", "Archivado", true},
		{"ideas no público", "Ideas", "", false},
		{"planeacion no público", "En planeación", "", false},
		{"vacío no público", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPhase, gotOK := publicPhase(tc.raw)
			if gotPhase != tc.wantPhase || gotOK != tc.wantOK {
				t.Errorf("publicPhase(%q) = (%q, %v); want (%q, %v)", tc.raw, gotPhase, gotOK, tc.wantPhase, tc.wantOK)
			}
		})
	}
}

func TestPublicBugStatus(t *testing.T) {
	cases := []struct {
		name       string
		phase      string
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
		{"archivado", "Archivado", githubv4.IssueState("OPEN"), "Resuelto", 100},
		{"cerrado", "Desarrollo", githubv4.IssueStateClosed, "Resuelto", 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotStatus, gotPct := publicBugStatus(tc.phase, tc.state)
			if gotStatus != tc.wantStatus || gotPct != tc.wantPct {
				t.Errorf(
					"publicBugStatus(%q, %q) = (%q, %d); want (%q, %d)",
					tc.phase,
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

func TestFileContentChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "modules.json")
	content := []byte("[\n  {\n    \"id\": \"1\"\n  }\n]\n")

	changed, err := fileContentChanged(path, content)
	if err != nil {
		t.Fatalf("fileContentChanged missing file returned error: %v", err)
	}
	if !changed {
		t.Fatal("fileContentChanged missing file = false; want true")
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	changed, err = fileContentChanged(path, content)
	if err != nil {
		t.Fatalf("fileContentChanged same content returned error: %v", err)
	}
	if changed {
		t.Fatal("fileContentChanged same content = true; want false")
	}

	changed, err = fileContentChanged(path, []byte("[\n]\n"))
	if err != nil {
		t.Fatalf("fileContentChanged different content returned error: %v", err)
	}
	if !changed {
		t.Fatal("fileContentChanged different content = false; want true")
	}
}

func TestMarshalJSONMatchesGeneratorFormat(t *testing.T) {
	got, err := marshalJSON([]ModuleOut{{ID: "1", Nombre: "Test", Fase: "Test", Estado: "En atención", Porcentaje: 50, Tipo: "bug"}})
	if err != nil {
		t.Fatalf("marshalJSON returned error: %v", err)
	}
	want := "[\n  {\n    \"id\": \"1\",\n    \"nombre\": \"Test\",\n    \"descripcion\": \"\",\n    \"fase\": \"Test\",\n    \"estado\": \"En atención\",\n    \"porcentaje\": 50,\n    \"tipo\": \"bug\"\n  }\n]\n"
	if string(got) != want {
		t.Fatalf("marshalJSON mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteOutputsIfModulesChangedSkipsMetadataWhenModulesUnchanged(t *testing.T) {
	dir := t.TempDir()
	modulesPath := filepath.Join(dir, "modules.json")
	metaPath := filepath.Join(dir, "modules-meta.json")
	modules := []ModuleOut{{ID: "1", Nombre: "Test", Fase: "Test", Estado: "En atención", Porcentaje: 50, Tipo: "bug"}}
	modulesJSON, err := marshalJSON(modules)
	if err != nil {
		t.Fatalf("marshalJSON modules: %v", err)
	}
	if err := os.WriteFile(modulesPath, modulesJSON, 0o644); err != nil {
		t.Fatalf("WriteFile modules: %v", err)
	}

	originalMeta := []byte("{\n  \"generatedAt\": \"2026-01-01T00:00:00Z\",\n  \"source\": \"GitHub Project EOS 2.0\",\n  \"itemCount\": 1\n}\n")
	if err := os.WriteFile(metaPath, originalMeta, 0o644); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}
	originalTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(metaPath, originalTime, originalTime); err != nil {
		t.Fatalf("Chtimes metadata: %v", err)
	}

	changed, err := writeOutputsIfModulesChanged(modulesPath, metaPath, modules, func() time.Time {
		return time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("writeOutputsIfModulesChanged returned error: %v", err)
	}
	if changed {
		t.Fatal("writeOutputsIfModulesChanged changed = true; want false")
	}

	gotMeta, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile metadata: %v", err)
	}
	if string(gotMeta) != string(originalMeta) {
		t.Fatalf("metadata changed on no-op run:\ngot:\n%s\nwant:\n%s", gotMeta, originalMeta)
	}
	gotInfo, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("Stat metadata: %v", err)
	}
	if !gotInfo.ModTime().Equal(originalTime) {
		t.Fatalf("metadata mtime changed on no-op run: got %s want %s", gotInfo.ModTime(), originalTime)
	}
}

func TestWriteOutputsIfModulesChangedWritesMetadataWhenModulesChange(t *testing.T) {
	dir := t.TempDir()
	modulesPath := filepath.Join(dir, "modules.json")
	metaPath := filepath.Join(dir, "modules-meta.json")
	if err := os.WriteFile(modulesPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile modules: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte("{\"generatedAt\":\"old\"}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}

	modules := []ModuleOut{{ID: "1", Nombre: "Test", Fase: "Test", Estado: "En atención", Porcentaje: 50, Tipo: "bug"}}
	fixedTime := time.Date(2026, 6, 25, 12, 34, 56, 0, time.UTC)
	changed, err := writeOutputsIfModulesChanged(modulesPath, metaPath, modules, func() time.Time {
		return fixedTime
	})
	if err != nil {
		t.Fatalf("writeOutputsIfModulesChanged returned error: %v", err)
	}
	if !changed {
		t.Fatal("writeOutputsIfModulesChanged changed = false; want true")
	}

	wantModules, err := marshalJSON(modules)
	if err != nil {
		t.Fatalf("marshalJSON modules: %v", err)
	}
	gotModules, err := os.ReadFile(modulesPath)
	if err != nil {
		t.Fatalf("ReadFile modules: %v", err)
	}
	if string(gotModules) != string(wantModules) {
		t.Fatalf("modules output mismatch:\ngot:\n%s\nwant:\n%s", gotModules, wantModules)
	}

	var gotMeta MetadataOut
	gotMetaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile metadata: %v", err)
	}
	if err := json.Unmarshal(gotMetaBytes, &gotMeta); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	if gotMeta.GeneratedAt != fixedTime.Format(time.RFC3339) {
		t.Fatalf("GeneratedAt = %q; want %q", gotMeta.GeneratedAt, fixedTime.Format(time.RFC3339))
	}
	if gotMeta.Source != defaultMetadataSource {
		t.Fatalf("Source = %q; want %q", gotMeta.Source, defaultMetadataSource)
	}
	if gotMeta.ItemCount != len(modules) {
		t.Fatalf("ItemCount = %d; want %d", gotMeta.ItemCount, len(modules))
	}
}

func TestPublicPhaseMapsPlanningToReported(t *testing.T) {
	phase, ok := publicPhase("En planeación")
	if !ok {
		t.Fatal("expected En planeación to be public")
	}
	if phase != "Reportados" {
		t.Fatalf("expected Reportados, got %q", phase)
	}
}

func TestPublicBugStatusForReported(t *testing.T) {
	status, baseline := publicBugStatus("Reportados", githubv4.IssueStateOpen)
	if status != "Reportado" {
		t.Fatalf("expected Reportado, got %q", status)
	}
	if baseline != 0 {
		t.Fatalf("expected baseline 0, got %d", baseline)
	}
}

func TestPublicPhaseStillExcludesIdeas(t *testing.T) {
	phase, ok := publicPhase("Ideas")
	if ok {
		t.Fatalf("expected Ideas to be private, got phase %q", phase)
	}
}
