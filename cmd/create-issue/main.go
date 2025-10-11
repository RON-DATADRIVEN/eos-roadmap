package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type fieldType string

const (
	fieldTypeMarkdown fieldType = "markdown"
	fieldTypeTextarea fieldType = "textarea"
	fieldTypeInput    fieldType = "input"
)

type templateField struct {
	ID       string
	Label    string
	Type     fieldType
	Required bool
	Value    string
}

type issueTemplate struct {
	ID     string
	Title  string
	Labels []string
	Body   []templateField
}

var templates = map[string]issueTemplate{
	"blank": {
		ID:    "blank",
		Title: "[ISSUE] Título",
		Labels: []string{
			"Status: Ideas",
			"Tipo :Blank Issue",
		},
		Body: []templateField{
			{
				ID:    "descripcion",
				Label: "Descripción",
				Type:  fieldTypeTextarea,
				Value: "**Contexto**\n-\n\n**Detalles**\n-\n\n**Criterio de aceptación**\n-",
			},
		},
	},
	"bug": {
		ID:    "bug",
		Title: "fix: <resumen>",
		Labels: []string{
			"Tipo: Bug",
			"Status :En planeación",
		},
		Body: []templateField{
			{ID: "summary", Label: "Resumen", Type: fieldTypeInput, Required: true},
			{ID: "steps", Label: "Pasos para reproducir", Type: fieldTypeTextarea, Required: true},
			{ID: "expected", Label: "Comportamiento esperado", Type: fieldTypeTextarea, Required: true},
			{ID: "actual", Label: "Comportamiento actual", Type: fieldTypeTextarea, Required: true},
			{ID: "env", Label: "Entorno", Type: fieldTypeTextarea},
			{ID: "logs", Label: "Logs/evidencia", Type: fieldTypeTextarea},
		},
	},
	"change_request": {
		ID:    "change_request",
		Title: "chore: change-request <resumen>",
		Labels: []string{
			"Tipo: Change Request",
			"Status: Ideas",
		},
		Body: []templateField{
			{
				ID:    "intro",
				Label: "",
				Type:  fieldTypeMarkdown,
				Value: "Describe el cambio propuesto y el impacto (tiempo, costo, riesgo). Será evaluado.",
			},
			{ID: "description", Label: "Descripción del cambio", Type: fieldTypeTextarea, Required: true},
			{ID: "impact", Label: "Impacto (alcance/tiempo/costo/riesgo)", Type: fieldTypeTextarea, Required: true},
			{ID: "requester", Label: "Solicitante", Type: fieldTypeInput, Required: true},
		},
	},
	"feature": {
		ID:    "feature",
		Title: "[FEAT] Título de la feature",
		Labels: []string{
			"Tipo: Feature",
			"Status: Ideas",
		},
		Body: []templateField{
			{ID: "descripcion", Label: "Descripción", Type: fieldTypeTextarea, Required: true},
			{ID: "criterio", Label: "Criterio de aceptación (resumen)", Type: fieldTypeInput, Required: true},
		},
	},
}

type issueRequest struct {
	TemplateID string            `json:"templateId"`
	Title      string            `json:"title"`
	Fields     map[string]string `json:"fields"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type issueResponse struct {
	IssueURL string    `json:"issueUrl,omitempty"`
	Error    *apiError `json:"error,omitempty"`
}

type githubIssueResponse struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	NodeID  string `json:"node_id"`
}

const (
	githubRepoOwner = "RON-DATADRIVEN"
	githubRepoName  = "eos-roadmap"
	userAgent       = "eos-roadmap-create-issue/1.0"
)

const defaultAllowedOrigin = "https://ron-datadriven.github.io"

type originEntry struct {
	raw        string
	normalized string
}

var (
	githubToken   = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	projectID     = strings.TrimSpace(os.Getenv("GITHUB_PROJECT_ID"))
	allowedOrigin = strings.TrimSpace(os.Getenv("ALLOWED_ORIGIN"))

	allowAnyOrigin       bool
	allowedOriginEntries = configureAllowedOrigins(allowedOrigin, defaultAllowedOrigin)
)

func main() {
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN no configurado")
	}
	if projectID == "" {
		log.Fatal("GITHUB_PROJECT_ID no configurado")
	}
	if allowAnyOrigin {
		log.Print("CORS abierto: se permiten todos los orígenes (ALLOWED_ORIGIN=*)")
	} else if len(allowedOriginEntries) == 0 {
		log.Print("ADVERTENCIA: ALLOWED_ORIGIN vacío o sin valores válidos, se rechazarán solicitudes con origen")
	} else {
		log.Printf("Orígenes permitidos: %s", allowedOrigin)
	}

	http.HandleFunc("/", handleRequest)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Escuchando en :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("error al iniciar servidor: %v", err)
	}
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if !handleCORS(w, r) {
		return
	}

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodPost:
		handlePost(w, r)
	default:
		http.Error(w, "método no permitido", http.StatusMethodNotAllowed)
	}
}

func handleCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	if !isOriginAllowed(origin) {
		http.Error(w, "origen no permitido", http.StatusForbidden)
		return false
	}

	if allowAnyOrigin {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "3600")
	return true
}

func isOriginAllowed(origin string) bool {
	if allowAnyOrigin {
		return true
	}

	if len(allowedOriginEntries) == 0 {
		return false
	}

	normalizedOrigin, err := normalizeOrigin(origin)
	if err != nil {
		return false
	}

	for _, entry := range allowedOriginEntries {
		if entry.normalized == normalizedOrigin {
			return true
		}
	}

	return false
}

func configureAllowedOrigins(current, fallback string) []originEntry {
	seen := map[string]struct{}{}
	var entries []originEntry

	addOrigin := func(value string, source string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}

		if value == "*" {
			allowAnyOrigin = true
			allowedOrigin = "*"
			entries = nil
			seen = map[string]struct{}{}
			return
		}

		if allowAnyOrigin {
			return
		}

		normalized, err := normalizeOrigin(value)
		if err != nil {
			log.Printf("origen permitido inválido ignorado (%s): %q", source, value)
			return
		}

		if _, ok := seen[normalized]; ok {
			return
		}

		entries = append(entries, originEntry{raw: value, normalized: normalized})
		seen[normalized] = struct{}{}
	}

	for _, candidate := range splitOriginCandidates(current) {
		addOrigin(candidate, "ALLOWED_ORIGIN")
	}

	if allowAnyOrigin {
		return nil
	}

	addOrigin(fallback, "predeterminado")

	if allowAnyOrigin {
		return nil
	}

	if len(entries) == 0 {
		allowedOrigin = ""
		return nil
	}

	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.raw)
	}
	allowedOrigin = strings.Join(values, ",")

	return entries
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("origen %q incompleto", value)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())

	port := parsed.Port()
	if port != "" {
		if !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
			host = fmt.Sprintf("%s:%s", host, port)
		}
	}

	return fmt.Sprintf("%s://%s", scheme, host), nil
}

func splitOriginCandidates(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ';':
			return true
		default:
			return false
		}
	})

	cleaned := make([]string, 0, len(fields))
	for _, candidate := range fields {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		cleaned = append(cleaned, candidate)
	}

	return cleaned
}

func handlePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "JSON inválido")
		return
	}

	tmpl, ok := templates[req.TemplateID]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_template", "Plantilla no válida")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "El título es obligatorio")
		return
	}

	fields := map[string]string{}
	for k, v := range req.Fields {
		fields[k] = strings.TrimSpace(v)
	}

	body, err := buildBody(tmpl, fields)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	issue, err := createIssue(ctx, title, tmpl.Labels, body)
	if err != nil {
		log.Printf("error al crear issue: %v", err)
		writeError(w, http.StatusBadGateway, "github_issue_error", "No se pudo crear el issue en GitHub")
		return
	}

	err = addToProject(ctx, issue.NodeID)
	if err != nil {
		log.Printf("issue #%d creado pero no se pudo agregar al proyecto: %v", issue.Number, err)
		writeResponse(w, http.StatusOK, issueResponse{
			IssueURL: issue.HTMLURL,
			Error: &apiError{
				Code:    "github_project_error",
				Message: "Issue creado pero no se pudo agregar al proyecto",
			},
		})
		return
	}

	writeResponse(w, http.StatusOK, issueResponse{IssueURL: issue.HTMLURL})
}

func buildBody(tmpl issueTemplate, fields map[string]string) (string, error) {
	var sections []string

	for _, field := range tmpl.Body {
		switch field.Type {
		case fieldTypeMarkdown:
			if strings.TrimSpace(field.Value) != "" {
				sections = append(sections, field.Value)
			}
		case fieldTypeTextarea, fieldTypeInput:
			value := strings.TrimSpace(fields[field.ID])
			if value == "" {
				if field.Required {
					return "", fmt.Errorf("El campo '%s' es obligatorio", displayLabel(field))
				}
				continue
			}
			sections = append(sections, fmt.Sprintf("### %s\n%s", displayLabel(field), value))
		default:
			return "", fmt.Errorf("Tipo de campo desconocido: %s", field.Type)
		}
	}

	return strings.TrimSpace(strings.Join(sections, "\n\n")), nil
}

func displayLabel(field templateField) string {
	if strings.TrimSpace(field.Label) == "" {
		return field.ID
	}
	return field.Label
}

func createIssue(ctx context.Context, title string, labels []string, body string) (*githubIssueResponse, error) {
	payload := map[string]any{
		"title":  title,
		"body":   body,
		"labels": labels,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", githubRepoOwner, githubRepoName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var apiResp map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, fmt.Errorf("estado inesperado %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("estado inesperado %d: %v", resp.StatusCode, apiResp)
	}

	var issue githubIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}
	if issue.NodeID == "" {
		return nil, errors.New("respuesta sin node_id")
	}
	return &issue, nil
}

func addToProject(ctx context.Context, nodeID string) error {
	if strings.TrimSpace(nodeID) == "" {
		return errors.New("node_id vacío")
	}

	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	httpClient := oauth2.NewClient(ctx, src)
	gqlClient := githubv4.NewClient(httpClient)

	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: githubv4.ID(projectID),
		ContentID: githubv4.ID(nodeID),
	}

	var mutation struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID githubv4.ID
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	return gqlClient.Mutate(ctx, &mutation, input, nil)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeResponse(w, status, issueResponse{Error: &apiError{Code: code, Message: message}})
}

func writeResponse(w http.ResponseWriter, status int, resp issueResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("error al escribir respuesta: %v", err)
	}
}
