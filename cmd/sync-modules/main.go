package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/shurcooL/githubv4"
)

type Item struct {
	Content struct {
		Issue struct {
			Number int
			Title  string
			URL    githubv4.URI
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
			StartDate githubv4.Date
			Duration  int
		} `graphql:"... on ProjectV2ItemFieldIterationValue"`
	} `graphql:"iter: fieldValueByName(name:\"Iteration\")"`

	Start struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date githubv4.Date
		} `graphql:"... on ProjectV2ItemFieldDateValue"`
	} `graphql:"start: fieldValueByName(name:\"Start date\")"`

	ETA struct {
		Typename githubv4.String `graphql:"__typename"`
		DateVal  struct {
			Date githubv4.Date
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

type ModuleOut struct {
	Issue     int    `json:"issue"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Area      string `json:"area,omitempty"`
	Status    string `json:"status,omitempty"`
	Prioridad string `json:"prioridad,omitempty"`
	Size      string `json:"size,omitempty"`
	Iteration string `json:"iteration,omitempty"`
	IterStart string `json:"iterationStart,omitempty"`
	IterDays  int    `json:"iterationDays,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	ETA       string `json:"eta,omitempty"`
}

func singleName(typename githubv4.String, name githubv4.String) string {
	if typename == "ProjectV2ItemFieldSingleSelectValue" {
		return string(name)
	}
	return ""
}

func toISO(d githubv4.Date) string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.Format("2006-01-02")
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

	// HTTP client con auth
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
			// Sólo Issues; ignora PRs/Drafts si aparecieran
			iss := it.Content.Issue
			if iss.Number == 0 {
				continue
			}
			m := ModuleOut{
				Issue:     iss.Number,
				Title:     iss.Title,
				URL:       iss.URL.String(),
				Area:      singleName(it.Area.Typename, it.Area.Single.Name),
				Status:    singleName(it.Status.Typename, it.Status.Single.Name),
				Prioridad: singleName(it.Prioridad.Typename, it.Prioridad.Single.Name),
				Size:      singleName(it.Size.Typename, it.Size.Single.Name),
				StartDate: toISO(it.Start.DateVal.Date),
				ETA:       toISO(it.ETA.DateVal.Date),
			}
			if it.Iter.Typename == "ProjectV2ItemFieldIterationValue" {
				m.Iteration = string(it.Iter.Iteration.Title)
				m.IterStart = toISO(it.Iter.Iteration.StartDate)
				m.IterDays = it.Iter.Iteration.Duration
			}
			all = append(all, m)
		}

		if !q.Org.Project.Items.PageInfo.HasNextPage {
			break
		}
		after = &q.Org.Project.Items.PageInfo.EndCursor
	}

	// Crea carpeta si no existe
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

