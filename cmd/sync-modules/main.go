package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

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

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}
type gqlResponse struct {
	Data   *gqlData       `json:"data"`
	Errors []gqlRespError `json:"errors"`
}
type gqlRespError struct {
	Message string `json:"message"`
}

type gqlData struct {
	Organization struct {
		ProjectV2 struct {
			Items struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []gqlItem `json:"nodes"`
			} `json:"items"`
		} `json:"projectV2"`
	} `json:"organization"`
}

type gqlItem struct {
	ID      string `json:"id"`
	Content struct {
		Typename  string `json:"__typename"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		Body      string `json:"body"`
		State     string `json:"state"`
		CreatedAt string `json:"createdAt"`
		Milestone *struct {
			DueOn *string `json:"dueOn"`
		} `json:"milestone"`
		Assignees struct {
			Nodes []struct {
				Login string `json:"login"`
			} `json:"nodes"`
		} `json:"assignees"`
		Labels struct {
			Nodes []struct {
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
	} `json:"content"`
	FieldValues struct {
		Nodes []gqlFieldValue `json:"nodes"`
	} `json:"fieldValues"`
}

type gqlFieldValue struct {
	Typename string `json:"__typename"`
	Field    struct {
		Name string `json:"name"`
	} `json:"field"`
	Name   string   `json:"name"`
	Number *float64 `json:"number"`
	Date   *string  `json:"date"`
	Text   *string  `json:"text"`
	Users  *struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"users"`
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fatal("GITHUB_TOKEN no definido")
	}

	out := getEnv("OUTPUT", "docs/modules.json")

	org := os.Getenv("PROJECT_ORG")
	projectNumber := getenvInt("PROJECT_NUMBER", 3)
	if org == "" {
		if slug := os.Getenv("REPO_SLUG"); slug != "" && strings.Contains(slug, "/") {
			org = strings.Split(slug, "/")[0]
		} else if slug := os.Getenv("GITHUB_REPOSITORY"); slug != "" && strings.Contains(slug, "/") {
			org = strings.Split(slug, "/")[0]
		}
	}
	if org == "" {
		fatal("PROJECT_ORG no definido y no pude inferirlo de REPO_SLUG/GITHUB_REPOSITORY")
	}

	moduleLabel := strings.ToLower(getEnv("MODULE_LABEL", "module"))

	items, err := fetchProjectItems(token, org, projectNumber)
	if err != nil {
		fatal(err.Error())
	}

	modules := make([]Module, 0, len(items))
	for _, it := range items {
		if strings.ToLower(it.Content.Typename) != "issue" {
			continue
		}
		hasModule := false
		for _, l := range it.Content.Labels.Nodes {
			if strings.ToLower(l.Name) == moduleLabel {
				hasModule = true
				break
			}
		}
		if !hasModule {
			continue
		}

		title := it.Content.Title
		url := it.Content.URL
		body := nz(it.Content.Body)
		created := it.Content.CreatedAt

		fv := map[string]gqlFieldValue{}
		for _, v := range it.FieldValues.Nodes {
			fv[v.Field.Name] = v
		}

		estado := ""
		if v, ok := fv["Status"]; ok && v.Name != "" {
			estado = v.Name
		} else {
			switch strings.ToLower(it.Content.State) {
			case "open":
				estado = "Planificado"
			case "closed":
				estado = "Hecho"
			default:
				estado = "Planificado"
			}
		}

		porc := -1
		for _, key := range []string{"Progreso", "Progress", "Percent"} {
			if v, ok := fv[key]; ok && v.Number != nil {
				n := *v.Number
				if n <= 1.0 {
					porc = int(n*100 + 0.5)
				} else {
					porc = int(n + 0.5)
				}
				break
			}
		}
		if porc < 0 {
			porc = calcProgressFromBody(body)
		}
		porc = clamp(porc, 0, 100)

		eta := ""
		if v, ok := fv["ETA"]; ok && v.Date != nil {
			eta = *v.Date
		} else if it.Content.Milestone != nil && it.Content.Milestone.DueOn != nil {
			eta = *it.Content.Milestone.DueOn
			eta = strings.Split(eta, "T")[0]
		} else {
			eta = parseETAFromBody(body)
		}

		inicio := ""
		if v, ok := fv["Start date"]; ok && v.Date != nil {
			inicio = *v.Date
		} else if created != "" {
			inicio = strings.Split(created, "T")[0]
		}

		prop := joinUsersFromValues(fv, it)
		desc := firstParagraph(clean(body))

		modules = append(modules, Module{
			ID:          it.ID,
			Nombre:      title,
			Descripcion: desc,
			Estado:      estado,
			Porcentaje:  porc,
			Propietario: prop,
			Inicio:      inicio,
			ETA:         eta,
			Enlaces:     []Link{{Label: "Item", URL: url}},
		})
	}

	if err := writeJSON(out, modules); err != nil {
		fatal(err.Error())
	}
	fmt.Printf("✔ Generado %s con %d módulos (Project %s#%d)\n", out, len(modules), org, projectNumber)
}

func fetchProjectItems(token, org string, number int) ([]gqlItem, error) {
	const q = `
query ($org: String!, $number: Int!, $first: Int!, $after: String) {
  organization(login: $org) {
    projectV2(number: $number) {
      items(first: $first, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          content {
            __typename
            ... on Issue {
              title url body state createdAt
              milestone { dueOn }
              assignees(first: 10) { nodes { login } }
              labels(first: 50) { nodes { name } }
            }
            ... on PullRequest { title url body createdAt }
            ... on DraftIssue { title body }
          }
          fieldValues(first: 50) {
            nodes {
              __typename
              field { ... on ProjectV2FieldCommon { name } }
              ... on ProjectV2ItemFieldSingleSelectValue { name }
              ... on ProjectV2ItemFieldNumberValue       { number }
              ... on ProjectV2ItemFieldDateValue         { date }
              ... on ProjectV2ItemFieldTextValue         { text }
              ... on ProjectV2ItemFieldUserValue         { users(first: 10) { nodes { login } } }
            }
          }
        }
      }
    }
  }
}`
	items := make([]gqlItem, 0, 128)
	after := ""
	client := &http.Client{Timeout: 30 * time.Second}

	for {
		reqBody, _ := json.Marshal(gqlRequest{
			Query: q,
			Variables: map[string]interface{}{
				"org":    org,
				"number": number,
				"first":  100,
				"after":  nullable(after),
			},
		})
		req, _ := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GraphQL %d: %s", resp.StatusCode, string(b))
		}

		var gr gqlResponse
		if err := json.Unmarshal(b, &gr); err != nil {
			return nil, err
		}
		if len(gr.Errors) > 0 {
			msgs := make([]string, 0, len(gr.Errors))
			for _, e := range gr.Errors {
				msgs = append(msgs, e.Message)
			}
			return nil, fmt.Errorf("GraphQL errors: %s", strings.Join(msgs, "; "))
		}
		page := gr.Data.Organization.ProjectV2.Items
		items = append(items, page.Nodes...)
		if !page.PageInfo.HasNextPage {
			break
		}
		after = page.PageInfo.EndCursor
	}
	return items, nil
}

func firstParagraph(s string) string {
	parts := strings.Split(s, "\n\n")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			if len(p) > 300 {
				p = p[:300] + "…"
			}
			return p
		}
	}
	return ""
}

func clean(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	re := regexp.MustCompile(`(?m)^[#>*\-\s` + "`" + `]+`)
	s = re.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func calcProgressFromBody(body string) int {
	re := regexp.MustCompile(`(?i)(progress|progreso|avance)\s*:\s*(\d{1,3})%?`)
	if m := re.FindStringSubmatch(body); len(m) == 3 {
		return atoiSafe(m[2])
	}
	reDone := regexp.MustCompile(`(?m)^\s*-\s*\[x\]\s+`)
	reTodo := regexp.MustCompile(`(?m)^\s*-\s*\[ \]\s+`)
	done := len(reDone.FindAllStringIndex(body, -1))
	todo := len(reTodo.FindAllStringIndex(body, -1))
	if done+todo > 0 {
		return int(float64(done)/float64(done+todo)*100.0 + 0.5)
	}
	return 0
}

func parseETAFromBody(body string) string {
	re := regexp.MustCompile(`(?i)eta\s*:\s*(\d{4}-\d{2}-\d{2})`)
	if m := re.FindStringSubmatch(body); len(m) == 2 {
		return m[1]
	}
	return ""
}

func joinUsersFromValues(fv map[string]gqlFieldValue, it gqlItem) string {
	if v, ok := fv["Assignees"]; ok && v.Users != nil && len(v.Users.Nodes) > 0 {
		logins := make([]string, 0, len(v.Users.Nodes))
		for _, u := range v.Users.Nodes {
			logins = append(logins, u.Login)
		}
		return strings.Join(logins, ", ")
	}
	if len(it.Content.Assignees.Nodes) > 0 {
		logins := make([]string, 0, len(it.Content.Assignees.Nodes))
		for _, u := range it.Content.Assignees.Nodes {
			logins = append(logins, u.Login)
		}
		return strings.Join(logins, ", ")
	}
	return "—"
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func dir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

func clamp(x, a, b int) int {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func nz(s string) string {
	if s == "" {
		return ""
	}
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func getenvInt(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return def
}

func getEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
	os.Exit(1)
}
