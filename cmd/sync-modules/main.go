package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
)

// ---------- Flex date that accepts "YYYY-MM-DD" or RFC3339 ----------
type GHFlexDate struct {
	time.Time
	Raw string
}

func (fd *GHFlexDate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		fd.Time = time.Time{}
		fd.Raw = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	fd.Raw = s
	if s == "" {
		fd.Time = time.Time{}
		return nil
	}
	// DateTime (RFC3339 / RFC3339Nano)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		fd.Time = t
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		fd.Time = t
		return nil
	}
	// Date only (YYYY-MM-DD) -> parse in UTC
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		fd.Time = t
		return nil
	}
	return fmt.Errorf("GHFlexDate: formato no reconocido: %q", s)
}

func (fd GHFlexDate) IsZero() bool { return fd.Time.IsZero() }
func (fd GHFlexDate) ISODate() string {
	if fd.IsZero() {
		return ""
	}
	return fd.Time.UTC().Format("2006-01-02")
}
func (fd GHFlexDate) ISODateTime() string {
	if fd.IsZero() {
		return ""
	}
	return fd.Time.UTC().Format(time.RFC3339)
}

// Helper para imprimir YYYY-MM-DD
func toISO(d GHFlexDate) string { return d.ISODate() }

// ---------- GraphQL types ----------
type Item struct {
	Content struct {
		Issue struct {
			Number    int
			Title     string
			URL       githubv4.URI
			Body      string
			Assignees struct {
				Nodes []assigneeNode
			} `graphql:"assignees(first: 10)"`
		} `graphql:"... on Issue"`
	} `graphql:"content"`

	Area struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"area: fieldValueByName(name:\"Area\")"`

	Status struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"status: fieldValueByName(name:\"Status\")"`

	Prioridad struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"prioridad: fieldValueByName(name:\"Prioridad\")"`

	Size struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"size: fieldValueByName(name:\"Size\")"`

	Iter struct {
		Typename  githubv4.String `graphql:"__typename"`
		Iteration struct {
			Title     githubv4.String
			StartDate GHFlexDate // <- acepta Date o DateTime
			Duration  int
		} `graphql:"... on ProjectV2ItemFieldIterationValue"`
	} `graphql:"iter: fieldValueByName(name:\"Iteration\")"`

	Start struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date GHFlexDate // <- acepta Date o DateTime
		} `graphql:"... on ProjectV2ItemFieldDateValue"`
	} `graphql:"start: fieldValueByName(name:\"Start date\")"`

	ETA struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date GHFlexDate // <- acepta Date o DateTime
		} `graphql:"... on ProjectV2ItemFieldDateValue"`
	} `graphql:"eta: fieldValueByName(name:\"ETA\")"`
}

type page struct {
	Nodes    []Item
	PageInfo struct {
		HasNextPage bool
		EndCursor   githubv4.String
	}
}

type Query struct {
	Org struct {
		Project struct {
			Items page `graphql:"items(first: $first, after: $after)"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $org)"`
}

// ---------- Output JSON ----------
type assigneeNode struct {
	Login string
}

type ModuleOut struct {
	ID          string    `json:"id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Estado      string    `json:"estado"`
	Porcentaje  int       `json:"porcentaje"`
	Propietario string    `json:"propietario"`
	Inicio      string    `json:"inicio,omitempty"`
	ETA         string    `json:"eta,omitempty"`
	Enlaces     []LinkOut `json:"enlaces,omitempty"`
}

type LinkOut struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func singleName(typename githubv4.String, name githubv4.String) string {
	if typename == "ProjectV2ItemFieldSingleSelectValue" {
		return string(name)
	}
	return ""
}

func normalizeStatus(raw string) (string, int) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "Planificado", 0
	}
	switch s {
	case "hecho", "done", "completado", "completo", "finalizado", "cerrado", "closed":
		return "Hecho", 100
	case "en curso", "curso", "en ejecución", "en ejecucion", "desarrollo", "en desarrollo", "in progress", "progress", "bloqueado", "bloqueada":
		return "En curso", 50
	case "planificado", "planificada", "planificación", "planificacion", "en planeación", "en planeacion", "planeado", "planeada", "por hacer", "pendiente", "backlog":
		return "Planificado", 0
	}
	if strings.Contains(s, "hech") || strings.Contains(s, "done") || strings.Contains(s, "final") {
		return "Hecho", 100
	}
	if strings.Contains(s, "curso") || strings.Contains(s, "desarr") || strings.Contains(s, "progres") || strings.Contains(s, "bloq") {
		return "En curso", 50
	}
	return "Planificado", 0
}

func buildDescription(body, title string) string {
	cleaned := strings.ReplaceAll(body, "\r", "\n")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return fmt.Sprintf("Seguimiento del issue \"%s\".", title)
	}
	parts := strings.Split(cleaned, "\n\n")
	candidate := strings.TrimSpace(parts[0])
	if candidate == "" {
		candidate = cleaned
	}
	candidate = collapseSpaces(candidate)
	return truncateRunes(candidate, 280)
}

func collapseSpaces(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func buildOwner(nodes []assigneeNode) string {
	owners := make([]string, 0, len(nodes))
	for _, n := range nodes {
		login := strings.TrimSpace(n.Login)
		if login != "" {
			owners = append(owners, login)
		}
	}
	if len(owners) == 0 {
		return "Sin asignar"
	}
	return strings.Join(owners, ", ")
}

func buildLinks(url string) []LinkOut {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	return []LinkOut{{
		Label: "GitHub",
		URL:   url,
	}}
}

// ---------- Main ----------
func main() {
	log.SetFlags(0)

	org := os.Getenv("ORG")
	if org == "" {
		org = "RON-DATADRIVEN"
	}
	projectStr := os.Getenv("PROJECT_NUMBER")
	if projectStr == "" {
		projectStr = "3"
	}
	projectNum, err := strconv.Atoi(projectStr)
	if err != nil {
		log.Fatalf("PROJECT_NUMBER inválido: %v", err)
	}
	outPath := os.Getenv("OUTPUT")
	if outPath == "" {
		outPath = "docs/modules.json"
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN no está definido")
	}

	httpClient := &http.Client{
		Transport: roundTripperWithToken{token: token},
		Timeout:   30 * time.Second,
	}
	cli := githubv4.NewClient(httpClient)

	first := githubv4.Int(100)
	var after *githubv4.String
	var all []ModuleOut

	for {
		var q Query
		vars := map[string]interface{}{
			"org":           githubv4.String(org),
			"projectNumber": githubv4.Int(projectNum),
			"first":         first,
			"after":         after,
		}
		if err := cli.Query(context.Background(), &q, vars); err != nil {
			log.Fatalf("GraphQL: %v", err)
		}

		for _, it := range q.Org.Project.Items.Nodes {
			iss := it.Content.Issue
			if iss.Number == 0 {
				continue
			}
			rawStatus := singleName(it.Status.Typename, it.Status.Single.Name)
			estado, porcentaje := normalizeStatus(rawStatus)
			m := ModuleOut{
				ID:          strconv.Itoa(iss.Number),
				Nombre:      iss.Title,
				Descripcion: buildDescription(iss.Body, iss.Title),
				Estado:      estado,
				Porcentaje:  porcentaje,
				Propietario: buildOwner(iss.Assignees.Nodes),
				Inicio:      toISO(it.Start.DateVal.Date),
				ETA:         toISO(it.ETA.DateVal.Date),
				Enlaces:     buildLinks(iss.URL.String()),
			}
			all = append(all, m)
		}

		if !q.Org.Project.Items.PageInfo.HasNextPage {
			break
		}
		after = &q.Org.Project.Items.PageInfo.EndCursor
	}

	// Crear carpeta si no existe y escribir JSON
	if err := os.MkdirAll(dirOf(outPath), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("crear %s: %v", outPath, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(all); err != nil {
		log.Fatalf("json: %v", err)
	}
	log.Printf("OK: escrito %s con %d elementos", outPath, len(all))
}

// ---------- Utils ----------
type roundTripperWithToken struct{ token string }

func (rt roundTripperWithToken) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+rt.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	return http.DefaultTransport.RoundTrip(req)
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}
