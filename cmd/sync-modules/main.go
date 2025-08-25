package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Label struct{ Name string `json:"name"` }
type Milestone struct{ DueOn *time.Time `json:"due_on"` }
type User struct{ Login string `json:"login"` }

type Issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	CreatedAt   time.Time `json:"created_at"`
	Milestone   *Milestone  `json:"milestone"`
	Labels      []Label     `json:"labels"`
	Assignees   []User      `json:"assignees"`
	PullRequest *struct{}   `json:"pull_request,omitempty"`
}

type Module struct {
	ID          string `json:"id"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Estado      string `json:"estado"`
	Porcentaje  int    `json:"porcentaje"`
	Propietario string `json:"propietario"`
	Inicio      string `json:"inicio"`
	ETA         string `json:"eta"`
	Enlaces     []Link `json:"enlaces"`
}

type Link struct{ Label string `json:"label"`; URL string `json:"url"` }

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" { fatal("GITHUB_TOKEN no definido") }

	repoSlug := os.Getenv("REPO_SLUG")
	if repoSlug == "" { repoSlug = os.Getenv("GITHUB_REPOSITORY") }
	if repoSlug == "" || !strings.Contains(repoSlug, "/") { fatal("REPO_SLUG debe ser owner/repo") }
	owner := strings.Split(repoSlug, "/")[0]
	repo := strings.Split(repoSlug, "/")[1]

	out := os.Getenv("OUTPUT"); if out == "" { out = "docs/modules.json" }
	moduleLabel := os.Getenv("MODULE_LABEL"); if moduleLabel == "" { moduleLabel = "module" }

	issues, err := fetchIssues(token, owner, repo)
	if err != nil { fatal(err.Error()) }

	var modules []Module
	for _, is := range issues {
		if is.PullRequest != nil { continue }
		if !hasLabel(is.Labels, moduleLabel) { continue }

		estado := mapEstado(is)
		porc := calcProgress(is)
		desc := firstParagraph(clean(is.Body))
		eta := ""
		if is.Milestone != nil && is.Milestone.DueOn != nil { eta = is.Milestone.DueOn.UTC().Format("2006-01-02") }
		prop := joinAssignees(is.Assignees)

		modules = append(modules, Module{
			ID:          fmt.Sprintf("issue-%d", is.Number),
			Nombre:      is.Title,
			Descripcion: desc,
			Estado:      estado,
			Porcentaje:  clamp(porc, 0, 100),
			Propietario: prop,
			Inicio:      is.CreatedAt.UTC().Format("2006-01-02"),
			ETA:         eta,
			Enlaces:     []Link{{Label: "Issue", URL: is.HTMLURL}},
		})
	}

	if err := writeJSON(out, modules); err != nil { fatal(err.Error()) }
	fmt.Printf("✔ Generado %s con %d módulos\n", out, len(modules))
}

func fetchIssues(token, owner, repo string) ([]Issue, error) {
	client := &http.Client{ Timeout: 30 * time.Second }
	perPage := 100
	page := 1
	var all []Issue
	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=all&per_page=%d&page=%d", owner, repo, perPage, page)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		resp, err := client.Do(req)
		if err != nil { return nil, err }
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(b))
		}
		var batch []Issue
		if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil { return nil, err }
		all = append(all, batch...)
		if len(batch) < perPage { break }
		page++
	}
	return all, nil
}

func hasLabel(labels []Label, name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, l := range labels { if strings.ToLower(l.Name) == name { return true } }
	return false
}

func mapEstado(is Issue) string {
	lower := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	var names []string
	for _, l := range is.Labels { names = append(names, lower(l.Name)) }
	if contains(names, "status:en-curso") || contains(names, "status:en curso") || contains(names, "in-progress") { return "En curso" }
	if contains(names, "status:hecho") || is.State == "closed" { return "Hecho" }
	if contains(names, "status:planificado") { return "Planificado" }
	if is.State == "open" { return "Planificado" }
	return "Hecho"
}

func calcProgress(is Issue) int {
	re := regexp.MustCompile(`(?i)progress:\s*(\d{1,3})%`)
	if m := re.FindStringSubmatch(is.Body); len(m) == 2 { return atoiSafe(m[1]) }
	reDone := regexp.MustCompile(`(?m)^\s*-\s*\[x\]\s+`)
	done := len(reDone.FindAllStringIndex(is.Body, -1))
	reTodo := regexp.MustCompile(`(?m)^\s*-\s*\[ \]\s+`)
	todo := len(reTodo.FindAllStringIndex(is.Body, -1))
	if done+todo > 0 { return int(float64(done)/float64(done+todo)*100.0 + 0.5) }
	if is.State == "closed" { return 100 }
	return 0
}

func firstParagraph(s string) string {
	parts := strings.Split(s, "\n\n")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			if len(p) > 300 { p = p[:300] + "…" }
			return p
		}
	}
	return ""
}

func clean(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	re := regexp.MustCompile(`(?m)^[#>*` + "`" + `\-]+\s*`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func joinAssignees(as []User) string {
	if len(as) == 0 { return "—" }
	var v []string
	for _, a := range as { v = append(v, a.Login) }
	return strings.Join(v, ", ")
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil { return err }
	if err := os.MkdirAll(dir(path), 0o755); err != nil { return err }
	return os.WriteFile(path, b, 0o644)
}

func dir(p string) string { if i := strings.LastIndex(p, "/"); i >= 0 { return p[:i] }; return "." }
func clamp(x, a, b int) int { if x < a { return a }; if x > b { return b }; return x }
func atoiSafe(s string) int { n := 0; for _, ch := range s { if ch >= '0' && ch <= '9' { n = n*10 + int(ch-'0') } }; return n }
func contains(arr []string, needle string) bool { for _, v := range arr { if v == needle { return true } }; return false }
func fatal(msg string) { fmt.Fprintln(os.Stderr, "ERROR:", msg); os.Exit(1) }

