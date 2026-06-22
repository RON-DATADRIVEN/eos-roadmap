package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
)

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
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		fd.Time = t
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		fd.Time = t
		return nil
	}
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
func toISO(d GHFlexDate) string { return d.ISODate() }

type Item struct {
	Content struct {
		Issue struct {
			Number int
			Title  string
			URL    githubv4.URI
			Body   string
			State  githubv4.IssueState
			Labels struct {
				Nodes []labelNode
			} `graphql:"labels(first: 20)"`
			Assignees struct {
				Nodes []assigneeNode
			} `graphql:"assignees(first: 10)"`
		} `graphql:"... on Issue"`
	} `graphql:"content"`

	Status struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"status: fieldValueByName(name:\"Status\")"`

	CheckLuis struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
	} `graphql:"checkLuis: fieldValueByName(name:\"Check Luis\")"`

	Tipo struct {
		Typename githubv4.String                `graphql:"__typename"`
		Single   struct{ Name githubv4.String } `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
		Text     struct {
			Text githubv4.String `graphql:"text"`
		} `graphql:"... on ProjectV2ItemFieldTextValue"`
	} `graphql:"tipo: fieldValueByName(name:\"Tipo\")"`

	Start struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date GHFlexDate
		} `graphql:"... on ProjectV2ItemFieldDateValue"`
	} `graphql:"start: fieldValueByName(name:\"Start date\")"`

	ETA struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date GHFlexDate
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

type assigneeNode struct{ Login string }
type labelNode struct{ Name string }

type ModuleOut struct {
	ID          string    `json:"id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Estado      string    `json:"estado"`
	Porcentaje  int       `json:"porcentaje"`
	Propietario string    `json:"propietario,omitempty"`
	Inicio      string    `json:"inicio,omitempty"`
	ETA         string    `json:"eta,omitempty"`
	Enlaces     []LinkOut `json:"enlaces,omitempty"`
	Tipo        string    `json:"tipo"`
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

func normalizeText(raw string) string {
	val := strings.TrimSpace(strings.ToLower(raw))
	replacer := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u")
	return replacer.Replace(val)
}

func normalizeForType(raw string) string {
	val := normalizeText(raw)
	for _, prefix := range []string{"tipo :", "tipo:", "type:"} {
		val = strings.TrimPrefix(val, prefix)
	}
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		val = strings.TrimSpace(val[1 : len(val)-1])
	}
	return val
}

func projectValueToString(typename githubv4.String, single string, text string) string {
	switch string(typename) {
	case "ProjectV2ItemFieldSingleSelectValue":
		return strings.TrimSpace(single)
	case "ProjectV2ItemFieldTextValue":
		return strings.TrimSpace(text)
	default:
		return ""
	}
}

func isBug(labels []string, projectTipo string) bool {
	if normalizeForType(projectTipo) == "bug" {
		return true
	}
	for _, label := range labels {
		if normalizeForType(label) == "bug" {
			return true
		}
	}
	return false
}

func isLuisApproved(raw string) bool { return normalizeText(raw) == "aprobado" }

func publicFeatureStatus(raw string) (string, int, bool) {
	switch normalizeText(raw) {
	case "prototipado":
		return "En prototipo", 20, true
	case "desarrollo":
		return "En desarrollo", 50, true
	case "test":
		return "En pruebas", 75, true
	case "staging":
		return "En validación", 90, true
	case "deploy":
		return "Liberado", 100, true
	default:
		return "", 0, false
	}
}

func publicBugStatus(raw string, state githubv4.IssueState) (string, int) {
	if state == githubv4.IssueStateClosed {
		return "Resuelto", 100
	}
	switch normalizeText(raw) {
	case "prototipado", "desarrollo", "test", "staging":
		return "En atención", 50
	case "deploy":
		return "Resuelto", 100
	default:
		return "Reportado", 0
	}
}

var progressRegex = regexp.MustCompile(`(?i)Progress:\s*(-?\d+)%`)
var checklistEmptyRegex = regexp.MustCompile(`(?i)-\s*\[\s*\]`)
var checklistDoneRegex = regexp.MustCompile(`(?i)-\s*\[\s*[xX]\s*\]`)

func calculatePercentage(body string, baseline int) int {
	if match := progressRegex.FindStringSubmatch(body); match != nil {
		if p, err := strconv.Atoi(match[1]); err == nil {
			if p < 0 {
				return 0
			}
			if p > 100 {
				return 100
			}
			return p
		}
	}
	empty := len(checklistEmptyRegex.FindAllStringIndex(body, -1))
	done := len(checklistDoneRegex.FindAllStringIndex(body, -1))
	total := empty + done
	if total > 0 {
		return (done * 100) / total
	}
	return baseline
}

func buildDescription(body, title string) string {
	cleaned := strings.ReplaceAll(body, "\r", "\n")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return fmt.Sprintf("Seguimiento del issue %q.", title)
	}
	parts := strings.Split(cleaned, "\n\n")
	candidate := strings.TrimSpace(parts[0])
	if candidate == "" {
		candidate = cleaned
	}
	candidate = collapseSpaces(candidate)
	return truncateRunes(candidate, 280)
}

func collapseSpaces(s string) string { return strings.Join(strings.Fields(s), " ") }

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
		if login := strings.TrimSpace(n.Login); login != "" {
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
	return []LinkOut{{Label: "GitHub", URL: url}}
}

func labelNames(nodes []labelNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if name := strings.TrimSpace(n.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

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

	httpClient := &http.Client{Transport: roundTripperWithToken{token: token}, Timeout: 30 * time.Second}
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
			labels := labelNames(iss.Labels.Nodes)
			projectTipo := projectValueToString(it.Tipo.Typename, string(it.Tipo.Single.Name), string(it.Tipo.Text.Text))
			rawStatus := singleName(it.Status.Typename, it.Status.Single.Name)
			checkLuis := singleName(it.CheckLuis.Typename, it.CheckLuis.Single.Name)

			tipo := ""
			estado := ""
			porcentajeBase := 0
			if isBug(labels, projectTipo) {
				tipo = "bug"
				estado, porcentajeBase = publicBugStatus(rawStatus, iss.State)
			} else if isLuisApproved(checkLuis) {
				if publicStatus, baseline, ok := publicFeatureStatus(rawStatus); ok {
					tipo = "feature"
					estado = publicStatus
					porcentajeBase = baseline
				}
			}
			if tipo == "" {
				continue
			}

			all = append(all, ModuleOut{
				ID:          strconv.Itoa(iss.Number),
				Nombre:      iss.Title,
				Descripcion: buildDescription(iss.Body, iss.Title),
				Estado:      estado,
				Porcentaje:  calculatePercentage(iss.Body, porcentajeBase),
				Propietario: buildOwner(iss.Assignees.Nodes),
				Inicio:      toISO(it.Start.DateVal.Date),
				ETA:         toISO(it.ETA.DateVal.Date),
				Enlaces:     buildLinks(iss.URL.String()),
				Tipo:        tipo,
			})
		}
		if !q.Org.Project.Items.PageInfo.HasNextPage {
			break
		}
		after = &q.Org.Project.Items.PageInfo.EndCursor
	}

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
	log.Printf("OK: escrito %s con %d elementos públicos", outPath, len(all))
}

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
