package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "shopware-dev"
	serverVersion = "0.2.0"
	timeout       = 30 * time.Second
)

type NoInput struct{}

type Config struct {
	BaseURL          string
	AdminToken       string
	StoreAccessKey   string
	PreferLiveData   bool
	AdminOpenAPIFile string
	StoreOpenAPIFile string
	EntitySchemaFile string
	AdminRoutesFile  string
	StoreRoutesFile  string
}

type ShopwareClient struct {
	cfg    Config
	client *http.Client
}

type OpenAPI struct {
	Paths map[string]map[string]OpenAPIOperation `json:"paths"`
}

type OpenAPIOperation struct {
	Summary     string                    `json:"summary"`
	Description string                    `json:"description"`
	OperationID string                    `json:"operationId"`
	Tags        []string                  `json:"tags"`
	Parameters  []map[string]any          `json:"parameters"`
	RequestBody map[string]any            `json:"requestBody"`
	Responses   map[string]map[string]any `json:"responses"`
}

type FindRoutesInput struct {
	Query   string `json:"query"`
	APIType string `json:"apiType,omitempty"`
}

type DescribeRouteInput struct {
	PathOrName string `json:"pathOrName"`
	APIType    string `json:"apiType"`
}

type CriteriaInput struct {
	Entity       string   `json:"entity"`
	Intent       string   `json:"intent"`
	Associations []string `json:"associations,omitempty"`
	Limit        int      `json:"limit,omitempty"`
}

type RequestExampleInput struct {
	Route    string         `json:"route"`
	Method   string         `json:"method"`
	Language string         `json:"language,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

type ExplainSurfaceInput struct {
	UseCase string `json:"useCase"`
}

type ExplainAuthInput struct {
	APIType string `json:"apiType"`
	Route   string `json:"route,omitempty"`
}

type ExplainFlowInput struct {
	UseCase string `json:"useCase"`
}

type AnalyzeOpenAPIInput struct {
	APIType string `json:"apiType,omitempty"`
}

type AnalyzeSearchCapabilitiesInput struct {
	Entities []string `json:"entities,omitempty"`
}

type AssessWorkflowSupportInput struct {
	UseCase string `json:"useCase,omitempty"`
}

type GenerateFlowChecklistInput struct {
	UseCase  string `json:"useCase"`
	Language string `json:"language,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

type GenerateFlowRequestPackInput struct {
	UseCase  string `json:"useCase"`
	Language string `json:"language,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

type ExportAssessmentReportInput struct {
	UseCase  string `json:"useCase,omitempty"`
	Format   string `json:"format,omitempty"`
	Language string `json:"language,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

type flowStep struct {
	Surface  string   `json:"surface"`
	Method   string   `json:"method"`
	Route    string   `json:"route"`
	Purpose  string   `json:"purpose"`
	Notes    []string `json:"notes,omitempty"`
	Optional bool     `json:"optional,omitempty"`
}

type curatedFlow struct {
	Name                 string           `json:"name"`
	Confidence           string           `json:"confidence"`
	Surface              string           `json:"surface"`
	Summary              string           `json:"summary"`
	Triggers             []string         `json:"triggers"`
	Steps                []flowStep       `json:"steps"`
	Prerequisites        []string         `json:"prerequisites,omitempty"`
	CommonFailureReasons []string         `json:"commonFailureReasons,omitempty"`
	DiagnosticChecks     []string         `json:"diagnosticChecks,omitempty"`
	RecommendedExamples  []map[string]any `json:"recommendedExamples,omitempty"`
}

type openAPIFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Method   string `json:"method,omitempty"`
}

func loadConfig() Config {
	adminOpenAPIFile := strings.TrimSpace(os.Getenv("SHOPWARE_ADMIN_OPENAPI_FILE"))
	if adminOpenAPIFile == "" {
		adminOpenAPIFile = "data/admin-openapi.json"
	}

	storeOpenAPIFile := strings.TrimSpace(os.Getenv("SHOPWARE_STORE_OPENAPI_FILE"))
	if storeOpenAPIFile == "" {
		storeOpenAPIFile = "data/store-openapi.json"
	}

	entitySchemaFile := strings.TrimSpace(os.Getenv("SHOPWARE_ENTITY_SCHEMA_FILE"))
	if entitySchemaFile == "" {
		entitySchemaFile = "data/entity-schema.json"
	}

	adminRoutesFile := strings.TrimSpace(os.Getenv("SHOPWARE_ADMIN_ROUTES_FILE"))
	if adminRoutesFile == "" {
		adminRoutesFile = "data/admin-routes.json"
	}

	storeRoutesFile := strings.TrimSpace(os.Getenv("SHOPWARE_STORE_ROUTES_FILE"))
	if storeRoutesFile == "" {
		storeRoutesFile = "data/store-routes.json"
	}

	return Config{
		BaseURL:          strings.TrimRight(os.Getenv("SHOPWARE_BASE_URL"), "/"),
		AdminToken:       os.Getenv("SHOPWARE_ADMIN_TOKEN"),
		StoreAccessKey:   os.Getenv("SHOPWARE_STORE_ACCESS_KEY"),
		PreferLiveData:   strings.EqualFold(os.Getenv("SHOPWARE_PREFER_LIVE_DATA"), "true"),
		AdminOpenAPIFile: adminOpenAPIFile,
		StoreOpenAPIFile: storeOpenAPIFile,
		EntitySchemaFile: entitySchemaFile,
		AdminRoutesFile:  adminRoutesFile,
		StoreRoutesFile:  storeRoutesFile,
	}
}

func newShopwareClient(cfg Config) *ShopwareClient {
	return &ShopwareClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *ShopwareClient) canUseLive(apiType string) bool {
	if c.cfg.BaseURL == "" {
		return false
	}

	switch apiType {
	case "admin":
		return c.cfg.AdminToken != ""
	case "store":
		return c.cfg.StoreAccessKey != ""
	default:
		return false
	}
}

func (c *ShopwareClient) getJSON(ctx context.Context, path string, apiType string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")

	switch apiType {
	case "admin":
		if c.cfg.AdminToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.cfg.AdminToken)
		}
	case "store":
		if c.cfg.StoreAccessKey != "" {
			req.Header.Set("sw-access-key", c.cfg.StoreAccessKey)
		}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed for %s: status %d", path, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func readJSONFile(path string, target any) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("no fallback file configured")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

func (c *ShopwareClient) fetchOrLoadJSON(ctx context.Context, livePath string, apiType string, filePath string, target any) (string, error) {
	if !c.cfg.PreferLiveData && strings.TrimSpace(filePath) != "" {
		if err := readJSONFile(filePath, target); err == nil {
			return "file", nil
		}
	}

	var liveErr error
	if c.canUseLive(apiType) {
		if err := c.getJSON(ctx, livePath, apiType, target); err == nil {
			return "live", nil
		} else {
			liveErr = err
		}
	}

	if strings.TrimSpace(filePath) != "" {
		if err := readJSONFile(filePath, target); err == nil {
			return "file", nil
		} else if liveErr != nil {
			return "", fmt.Errorf("live failed: %v; file fallback failed: %v", liveErr, err)
		} else {
			return "", fmt.Errorf("file fallback failed: %v", err)
		}
	}

	if liveErr != nil {
		return "", fmt.Errorf("live failed and no fallback file configured: %v", liveErr)
	}

	return "", fmt.Errorf("no live config and no fallback file configured")
}

func marshalPretty(v any) string {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("marshal error: %v", err)
	}
	return string(out)
}

func withSourceLabel(source string, body string) string {
	return fmt.Sprintf("[source: %s]\n%s", source, body)
}

type routeMatch struct {
	Path        string
	Method      string
	Summary     string
	Description string
	OperationID string
	Tags        []string
	Parameters  []map[string]any
	RequestBody map[string]any
	Responses   map[string]map[string]any
	Score       int
	MatchReason string
	Confidence  string
}

func normalizeRouteValue(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimPrefix(v, "/api")
	v = strings.TrimPrefix(v, "/store-api")
	if v == "" {
		return "/"
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	if len(v) > 1 {
		v = strings.TrimRight(v, "/")
	}
	return v
}

func queryTokens(query string) []string {
	raw := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	stop := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true,
		"for": true, "with": true, "without": true, "in": true, "on": true,
		"criteria": true, "criterion": true, "behavior": true, "workflows": true,
		"workflow": true, "investigate": true, "investigation": true,
		"debug": true, "issue": true, "problem": true,
	}

	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.Trim(token, " ,.:;!?()[]{}\"'")
		if token == "" || stop[token] {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func tokenizeRouteText(parts ...string) map[string]bool {
	tokens := map[string]bool{}
	replacer := strings.NewReplacer("/", " ", "-", " ", "_", " ", "{", " ", "}", " ")
	for _, part := range parts {
		normalized := replacer.Replace(strings.ToLower(part))
		for _, token := range strings.Fields(normalized) {
			tokens[token] = true
		}
	}
	return tokens
}

func pathDepth(path string) int {
	path = strings.Trim(path, "/")
	if path == "" {
		return 0
	}
	return len(strings.Split(path, "/"))
}

func inferIntent(tokens []string) string {
	for _, token := range tokens {
		switch token {
		case "create", "add", "new":
			return "create"
		case "update", "edit", "modify", "change", "set":
			return "update"
		case "delete", "remove":
			return "delete"
		case "search", "find", "lookup":
			return "search"
		case "aggregate", "stats":
			return "aggregate"
		case "list", "get", "read", "show", "fetch":
			return "read"
		}
	}
	return ""
}

func expectedMethods(intent string) map[string]bool {
	switch intent {
	case "create":
		return map[string]bool{"POST": true}
	case "update":
		return map[string]bool{"PATCH": true}
	case "delete":
		return map[string]bool{"DELETE": true}
	case "search", "aggregate":
		return map[string]bool{"POST": true}
	case "read":
		return map[string]bool{"GET": true}
	default:
		return nil
	}
}

func entityCandidates(tokens []string) []string {
	stop := map[string]bool{
		"admin": true, "store": true, "api": true,
		"create": true, "add": true, "new": true,
		"update": true, "edit": true, "modify": true, "change": true, "set": true,
		"delete": true, "remove": true,
		"search": true, "find": true, "lookup": true,
		"aggregate": true, "stats": true,
		"list": true, "get": true, "read": true, "show": true, "fetch": true,
		"criteria": true, "criterion": true, "behavior": true,
		"investigate": true, "investigation": true,
	}
	var entities []string
	var filtered []string
	for _, token := range tokens {
		if !stop[token] {
			filtered = append(filtered, token)
			entities = append(entities, token)
		}
	}
	for i := 0; i < len(filtered)-1; i++ {
		entities = append(entities, filtered[i]+"-"+filtered[i+1])
		entities = append(entities, filtered[i]+" "+filtered[i+1])
	}
	for i := 0; i < len(filtered)-2; i++ {
		entities = append(entities, filtered[i]+"-"+filtered[i+1]+"-"+filtered[i+2])
		entities = append(entities, filtered[i]+" "+filtered[i+1]+" "+filtered[i+2])
	}

	seen := map[string]bool{}
	var deduped []string
	for _, entity := range entities {
		if !seen[entity] {
			seen[entity] = true
			deduped = append(deduped, entity)
		}
	}
	return deduped
}

func scoreRoute(query string, method string, path string, op OpenAPIOperation) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}

	normalizedPath := normalizeRouteValue(path)
	normalizedQuery := normalizeRouteValue(q)
	tokens := queryTokens(q)
	pathLikeQuery := "/"
	if len(tokens) > 0 {
		pathLikeQuery = "/" + strings.Join(tokens, "/")
	}
	tokenSet := tokenizeRouteText(path, method, op.Summary, op.Description, op.OperationID, strings.Join(op.Tags, " "))

	score := 0
	if strings.Contains(strings.ToLower(strings.Join([]string{
		path,
		method,
		op.Summary,
		op.Description,
		op.OperationID,
		strings.Join(op.Tags, " "),
	}, " ")), q) {
		score += 80
	}

	allTokensMatch := true
	matchedTokens := 0
	for _, token := range tokens {
		if tokenSet[token] {
			matchedTokens++
			score += 10
			continue
		}
		allTokensMatch = false
	}
	if allTokensMatch {
		score += 40
	}

	if normalizedPath == normalizedQuery {
		score += 250
	}
	if normalizedPath == pathLikeQuery {
		score += 220
	}
	if strings.TrimPrefix(normalizedPath, "/") == strings.TrimPrefix(normalizedQuery, "/") {
		score += 100
	}
	if strings.Contains(normalizedPath, normalizedQuery) && normalizedQuery != "/" {
		score += 30
	}
	if strings.Contains(normalizedPath, pathLikeQuery) && pathLikeQuery != "/" {
		score += 80
	}

	intent := inferIntent(tokens)
	expected := expectedMethods(intent)
	upperMethod := strings.ToUpper(method)
	if expected != nil && expected[upperMethod] {
		score += 35
	}

	for _, entity := range entityCandidates(tokens) {
		baseEntityPath := "/" + entity
		switch {
		case normalizedPath == baseEntityPath:
			score += 150
		case normalizedPath == baseEntityPath+"/{id}":
			score += 100
		case normalizedPath == "/search/"+entity && intent == "search":
			score += 150
		case normalizedPath == "/aggregate/"+entity && intent == "aggregate":
			score += 150
		case strings.Contains(normalizedPath, baseEntityPath):
			score += 20
		}
	}

	if matchedTokens == 0 && score < 100 {
		return 0
	}

	score -= pathDepth(normalizedPath)
	return score
}

func confidenceForScore(score int) string {
	switch {
	case score >= 250:
		return "high"
	case score >= 140:
		return "medium"
	default:
		return "low"
	}
}

func buildMatchReason(query string, method string, path string, op OpenAPIOperation, score int) string {
	tokens := queryTokens(query)
	normalizedPath := normalizeRouteValue(path)
	normalizedQuery := normalizeRouteValue(query)
	intent := inferIntent(tokens)
	reasons := make([]string, 0, 4)

	if normalizedPath == normalizedQuery {
		reasons = append(reasons, "exact path match")
	} else if strings.EqualFold(op.OperationID, strings.TrimSpace(query)) {
		reasons = append(reasons, "exact operationId match")
	} else if strings.Contains(normalizedPath, normalizedQuery) && normalizedQuery != "/" {
		reasons = append(reasons, "normalized path match")
	}

	if intent != "" {
		expected := expectedMethods(intent)
		if expected[strings.ToUpper(method)] {
			reasons = append(reasons, "method matches query intent")
		}
	}

	entities := entityCandidates(tokens)
	for _, entity := range entities {
		baseEntityPath := "/" + entity
		if normalizedPath == baseEntityPath || normalizedPath == baseEntityPath+"/{id}" || normalizedPath == "/search/"+entity || normalizedPath == "/aggregate/"+entity {
			reasons = append(reasons, "entity route match")
			break
		}
	}

	tokenSet := tokenizeRouteText(path, method, op.Summary, op.Description, op.OperationID, strings.Join(op.Tags, " "))
	matched := 0
	for _, token := range tokens {
		if tokenSet[token] {
			matched++
		}
	}
	if matched > 0 {
		reasons = append(reasons, fmt.Sprintf("%d query token(s) matched", matched))
	}

	if len(reasons) == 0 {
		reasons = append(reasons, fmt.Sprintf("ranked by fuzzy route score %d", score))
	}

	return strings.Join(reasons, "; ")
}

func findMatches(openapi OpenAPI, query string, apiType string) []map[string]any {
	if strings.TrimSpace(query) == "" {
		return nil
	}

	var ranked []routeMatch
	for path, methods := range openapi.Paths {
		for method, op := range methods {
			score := scoreRoute(query, method, path, op)
			if score <= 0 {
				continue
			}
			ranked = append(ranked, routeMatch{
				Path:        path,
				Method:      strings.ToUpper(method),
				Summary:     op.Summary,
				Description: op.Description,
				OperationID: op.OperationID,
				Tags:        op.Tags,
				Score:       score,
				MatchReason: buildMatchReason(query, method, path, op, score),
				Confidence:  confidenceForScore(score),
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			if ranked[i].Path == ranked[j].Path {
				return ranked[i].Method < ranked[j].Method
			}
			return ranked[i].Path < ranked[j].Path
		}
		return ranked[i].Score > ranked[j].Score
	})

	matches := make([]map[string]any, 0, min(len(ranked), 20))
	for _, match := range ranked {
		matches = append(matches, map[string]any{
			"apiType":     apiType,
			"path":        match.Path,
			"method":      match.Method,
			"summary":     match.Summary,
			"operationId": match.OperationID,
			"tags":        match.Tags,
			"confidence":  match.Confidence,
			"matchReason": match.MatchReason,
		})
		if len(matches) == 20 {
			break
		}
	}
	return matches
}

func canonicalCriteriaIntent(intent string) string {
	intent = strings.ToLower(strings.TrimSpace(intent))
	switch intent {
	case "", "search_by_id", "find_by_exact_field", "search_text", "list_recent", "detail_with_associations":
		return intent
	}

	switch {
	case strings.Contains(intent, "exact"), strings.Contains(intent, "equals"), strings.Contains(intent, "field"):
		return "find_by_exact_field"
	case strings.Contains(intent, "detail"), strings.Contains(intent, "association"), strings.Contains(intent, "with_association"):
		return "detail_with_associations"
	case strings.Contains(intent, "recent"), strings.Contains(intent, "latest"), strings.Contains(intent, "newest"), strings.Contains(intent, "last"):
		return "list_recent"
	case strings.Contains(intent, "id"), strings.Contains(intent, "uuid"):
		return "search_by_id"
	case strings.Contains(intent, "search"), strings.Contains(intent, "term"), strings.Contains(intent, "text"), strings.Contains(intent, "keyword"):
		return "search_text"
	default:
		return ""
	}
}

func describeRoute(openapi OpenAPI, needle string) []map[string]any {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return nil
	}

	var ranked []routeMatch
	normalizedNeedle := normalizeRouteValue(needle)
	lowerNeedle := strings.ToLower(needle)

	for path, methods := range openapi.Paths {
		for method, op := range methods {
			score := 0
			normalizedPath := normalizeRouteValue(path)
			switch {
			case normalizedPath == normalizedNeedle:
				score += 300
			case strings.EqualFold(op.OperationID, needle):
				score += 260
			case strings.EqualFold(op.Summary, needle):
				score += 220
			case strings.Contains(normalizedPath, normalizedNeedle) && normalizedNeedle != "/":
				score += 40
			case strings.Contains(strings.ToLower(op.OperationID), lowerNeedle):
				score += 30
			case strings.Contains(strings.ToLower(op.Summary), lowerNeedle):
				score += 20
			default:
				continue
			}
			score -= pathDepth(normalizedPath)
			ranked = append(ranked, routeMatch{
				Path:        path,
				Method:      strings.ToUpper(method),
				Summary:     op.Summary,
				Description: op.Description,
				OperationID: op.OperationID,
				Tags:        op.Tags,
				Parameters:  op.Parameters,
				RequestBody: op.RequestBody,
				Responses:   op.Responses,
				Score:       score,
				MatchReason: buildMatchReason(needle, method, path, op, score),
				Confidence:  confidenceForScore(score),
			})
		}
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score == ranked[j].Score {
			if ranked[i].Path == ranked[j].Path {
				return ranked[i].Method < ranked[j].Method
			}
			return ranked[i].Path < ranked[j].Path
		}
		return ranked[i].Score > ranked[j].Score
	})

	results := make([]map[string]any, 0, min(len(ranked), 5))
	for _, match := range ranked {
		results = append(results, map[string]any{
			"path":        match.Path,
			"method":      match.Method,
			"summary":     match.Summary,
			"description": match.Description,
			"operationId": match.OperationID,
			"tags":        match.Tags,
			"parameters":  match.Parameters,
			"requestBody": match.RequestBody,
			"responses":   match.Responses,
			"confidence":  match.Confidence,
			"matchReason": match.MatchReason,
		})
		if len(results) == 5 {
			break
		}
	}
	return results
}

func buildCriteriaPayload(input CriteriaInput) map[string]any {
	limit := input.Limit
	if limit <= 0 {
		limit = 25
	}

	payload := map[string]any{
		"limit": limit,
	}

	switch canonicalCriteriaIntent(input.Intent) {
	case "search_by_id":
		payload["ids"] = []string{"<uuid>"}
	case "find_by_exact_field":
		payload["filter"] = []map[string]any{{
			"type":  "equals",
			"field": fmt.Sprintf("%s.<field>", input.Entity),
			"value": "<value>",
		}}
	case "search_text":
		payload["term"] = "<search-term>"
	case "list_recent":
		payload["sort"] = []map[string]any{{
			"field": "createdAt",
			"order": "DESC",
		}}
	case "detail_with_associations":
		payload["filter"] = []map[string]any{{
			"type":  "equals",
			"field": "id",
			"value": "<uuid>",
		}}
	default:
		payload["note"] = "Unknown intent. Supported values: search_by_id, find_by_exact_field, search_text, list_recent, detail_with_associations"
	}

	if len(input.Associations) > 0 {
		associations := map[string]any{}
		for _, a := range input.Associations {
			associations[a] = map[string]any{}
		}
		payload["associations"] = associations
	}

	return payload
}

func requestExample(baseURL string, input RequestExampleInput) string {
	return requestExampleForSurface(baseURL, input, "")
}

func requestExampleForSurface(baseURL string, input RequestExampleInput, surface string) string {
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	lang := strings.ToLower(strings.TrimSpace(input.Language))
	if lang == "" {
		lang = "curl"
	}

	bodyBytes, err := json.MarshalIndent(input.Payload, "", "  ")
	if err != nil {
		return "Failed to render payload"
	}

	body := string(bodyBytes)
	if input.Payload == nil {
		body = "{}"
	}

	if baseURL == "" {
		baseURL = "http://localhost"
	}

	route := routeWithSurfacePrefix(surface, input.Route)
	isStore := strings.HasPrefix(route, "/store-api/")
	if route == "" {
		route = input.Route
	}

	if lang == "js" {
		authHeader := "\"Authorization\": \"Bearer \" + process.env.SHOPWARE_ADMIN_TOKEN"
		if isStore {
			authHeader = "\"sw-access-key\": process.env.SHOPWARE_STORE_ACCESS_KEY"
		}

		return fmt.Sprintf("const response = await fetch(\"%s%s\", {\n  method: \"%s\",\n  headers: {\n    \"Accept\": \"application/json\",\n    %s,\n    \"Content-Type\": \"application/json\"\n  },\n  body: JSON.stringify(%s)\n});\n\nconst data = await response.json();\nconsole.log(data);", baseURL, route, method, authHeader, body)
	}

	authHeader := "  -H \"Authorization: Bearer $SHOPWARE_ADMIN_TOKEN\" \\\n"
	if isStore {
		authHeader = "  -H \"sw-access-key: $SHOPWARE_STORE_ACCESS_KEY\" \\\n"
	}

	return fmt.Sprintf("curl -X %s \"%s%s\" \\\n  -H \"Accept: application/json\" \\\n%s  -H \"Content-Type: application/json\" \\\n  -d '%s'", method, baseURL, route, authHeader, body)
}

func routeWithSurfacePrefix(surface string, route string) string {
	route = strings.TrimSpace(route)
	if route == "" || strings.Contains(route, " or ") {
		return route
	}
	if strings.HasPrefix(route, "/api/") || strings.HasPrefix(route, "/store-api/") {
		return route
	}
	switch surface {
	case "admin":
		return "/api" + route
	case "store":
		return "/store-api" + route
	default:
		return route
	}
}

func starterPayloadForRoute(route string) map[string]any {
	switch route {
	case "/category":
		return map[string]any{
			"name": "<category-name>",
		}
	case "/product-manufacturer":
		return map[string]any{
			"name": "<manufacturer-name>",
		}
	case "/product":
		return map[string]any{
			"name":          "<product-name>",
			"productNumber": "<product-number>",
			"stock":         10,
			"price": []map[string]any{{
				"currencyId": "<currency-id>",
				"gross":      19.99,
				"net":        16.8,
				"linked":     true,
			}},
		}
	case "/product/{id}":
		return map[string]any{
			"id": "<product-id>",
		}
	case "/search/product":
		return map[string]any{
			"limit": 3,
			"includes": map[string]any{
				"product": []string{"id", "name", "productNumber"},
			},
		}
	case "/search/category":
		return map[string]any{
			"limit": 3,
			"includes": map[string]any{
				"category": []string{"id", "name", "active"},
			},
		}
	case "/search/product-manufacturer":
		return map[string]any{
			"limit": 10,
			"includes": map[string]any{
				"product_manufacturer": []string{"id", "name"},
			},
		}
	case "/search":
		return map[string]any{
			"search": "<term>",
			"limit":  3,
		}
	case "/checkout/cart/line-item":
		return map[string]any{
			"items": []map[string]any{{
				"type":         "product",
				"id":           "<line-item-id>",
				"referencedId": "<product-id>",
				"quantity":     1,
			}},
		}
	case "/checkout/order":
		return map[string]any{
			"customerComment": "<optional-comment>",
		}
	case "/account/register":
		return map[string]any{
			"email":        "dev@example.com",
			"password":     "<password>",
			"firstName":    "<first-name>",
			"lastName":     "<last-name>",
			"salutationId": "<salutation-id>",
		}
	case "/account/login":
		return map[string]any{
			"username": "dev@example.com",
			"password": "<password>",
		}
	case "/account/convert-guest":
		return map[string]any{
			"password": "<new-password>",
		}
	default:
		return map[string]any{}
	}
}

func saveHintsForStep(step flowStep) []string {
	switch step.Route {
	case "/category":
		return []string{"Save `categoryId` from the created category response."}
	case "/product-manufacturer", "/search/product-manufacturer":
		return []string{"Save `manufacturerId` from the manufacturer result."}
	case "/product":
		return []string{"Save `productId` from the product response.", "If present, save `visibilities` or related sales-channel identifiers used during creation."}
	case "/search/product":
		return []string{"Save the returned product identifiers to compare Admin API existence with later Store API visibility checks."}
	case "/search":
		return []string{"Save any returned `productId` or listing evidence to confirm Store API visibility."}
	case "/checkout/cart/line-item":
		return []string{"Save the current `contextToken` and any line-item identifiers if later steps need cart continuity."}
	case "/account/register", "/account/login":
		return []string{"Save the new `contextToken` if the storefront session changes after auth."}
	case "/checkout/order":
		return []string{"Save `orderId` for payment, order-history, or follow-up state transitions."}
	case "/order":
		return []string{"Save `orderId` values you want to inspect further."}
	case "/_action/order/{orderId}/state/{transition}":
		return []string{"Save the target `orderId` and the successful transition used."}
	case "/_action/order_transaction/{orderTransactionId}/state/{transition}":
		return []string{"Save `orderTransactionId` and resulting transaction state."}
	case "/_action/order_delivery/{orderDeliveryId}/state/{transition}":
		return []string{"Save `orderDeliveryId` and resulting delivery state."}
	default:
		return nil
	}
}

func inputHintsForStep(step flowStep) []string {
	switch step.Route {
	case "/product":
		return []string{"Needs `categoryId` and often a sales-channel visibility target.", "May also need `manufacturerId`, price, stock, and active-state assumptions."}
	case "/product/{id}":
		return []string{"Needs `productId` from an earlier step."}
	case "/search/product":
		return []string{"May need `manufacturerId`, term text, or known field names from entity schema."}
	case "/checkout/cart/line-item":
		return []string{"Needs `productId` from a product creation or Store API search step."}
	case "/checkout/order":
		return []string{"Needs an authenticated customer context and a non-empty cart."}
	case "/account/login":
		return []string{"Needs customer credentials from registration or existing test data."}
	case "/account/register":
		return []string{"Needs customer profile fields and storefront-required IDs like `salutationId` depending on the instance."}
	case "/_action/order/{orderId}/state/{transition}":
		return []string{"Needs `orderId` and a valid order transition name."}
	case "/_action/order_transaction/{orderTransactionId}/state/{transition}":
		return []string{"Needs `orderTransactionId` and a valid transaction transition name."}
	case "/_action/order_delivery/{orderDeliveryId}/state/{transition}":
		return []string{"Needs `orderDeliveryId` and a valid delivery transition name."}
	case "/_action/order_transaction_capture_refund/{refundId}":
		return []string{"Needs `refundId` from an earlier capture/refund flow."}
	default:
		return nil
	}
}

func generateFlowChecklist(useCase string, language string, baseURL string, admin OpenAPI, store OpenAPI) map[string]any {
	flows, err := loadCuratedFlows("data/flows")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	flow := selectCuratedFlow(useCase, flows)
	if flow == nil {
		return map[string]any{
			"name":    "generic_flow_checklist",
			"summary": "No curated flow matched exactly.",
		}
	}
	if strings.TrimSpace(language) == "" {
		language = "curl"
	}
	steps := make([]map[string]any, 0, len(flow.Steps))
	for i, step := range flow.Steps {
		stepOut := map[string]any{
			"stepNumber": i + 1,
			"surface":    step.Surface,
			"method":     step.Method,
			"route":      step.Route,
			"purpose":    step.Purpose,
			"optional":   step.Optional,
			"saveHints":  saveHintsForStep(step),
			"inputHints": inputHintsForStep(step),
		}
		if len(step.Notes) > 0 {
			stepOut["notes"] = step.Notes
		}

		var openapi OpenAPI
		switch step.Surface {
		case "admin":
			openapi = admin
		case "store":
			openapi = store
		}
		if details := findRouteDetails(openapi, step.Route, step.Method); details != nil {
			stepOut["routeDetails"] = details
		}

		if !strings.Contains(step.Route, " or ") {
			stepOut["requestExample"] = requestExampleForSurface(baseURL, RequestExampleInput{
				Route:    routeWithSurfacePrefix(step.Surface, step.Route),
				Method:   step.Method,
				Language: language,
				Payload:  starterPayloadForRoute(step.Route),
			}, step.Surface)
		}

		steps = append(steps, stepOut)
	}

	return map[string]any{
		"name":          flow.Name,
		"surface":       flow.Surface,
		"summary":       flow.Summary,
		"language":      language,
		"checklist":     steps,
		"prerequisites": flow.Prerequisites,
	}
}

func flowStepVariableTokens(step flowStep) []string {
	switch step.Route {
	case "/category":
		return []string{"categoryId"}
	case "/product-manufacturer", "/search/product-manufacturer":
		return []string{"manufacturerId"}
	case "/product":
		return []string{"productId", "salesChannelId"}
	case "/search/product":
		return []string{"manufacturerId"}
	case "/search":
		return []string{"productId"}
	case "/checkout/cart/line-item":
		return []string{"productId", "contextToken"}
	case "/account/register", "/account/login":
		return []string{"contextToken"}
	case "/checkout/order":
		return []string{"orderId", "contextToken"}
	case "/order":
		return []string{"orderId"}
	case "/_action/order/{orderId}/state/{transition}":
		return []string{"orderId", "transition"}
	case "/_action/order_transaction/{orderTransactionId}/state/{transition}":
		return []string{"orderTransactionId", "transition"}
	case "/_action/order_delivery/{orderDeliveryId}/state/{transition}":
		return []string{"orderDeliveryId", "transition"}
	case "/_action/order_transaction_capture_refund/{refundId}":
		return []string{"refundId"}
	default:
		return nil
	}
}

func generateFlowRequestPack(useCase string, language string, baseURL string, admin OpenAPI, store OpenAPI) map[string]any {
	flows, err := loadCuratedFlows("data/flows")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	flow := selectCuratedFlow(useCase, flows)
	if flow == nil {
		return map[string]any{
			"name":    "generic_flow_request_pack",
			"summary": "No curated flow matched exactly.",
		}
	}
	if strings.TrimSpace(language) == "" {
		language = "curl"
	}

	steps := make([]map[string]any, 0, len(flow.Steps))
	knownVars := make([]string, 0)
	seenVars := map[string]bool{}
	for i, step := range flow.Steps {
		stepOut := map[string]any{
			"stepNumber": i + 1,
			"surface":    step.Surface,
			"method":     step.Method,
			"route":      step.Route,
			"purpose":    step.Purpose,
			"optional":   step.Optional,
		}
		if len(step.Notes) > 0 {
			stepOut["notes"] = step.Notes
		}
		if hints := inputHintsForStep(step); len(hints) > 0 {
			stepOut["inputHints"] = hints
		}
		if hints := saveHintsForStep(step); len(hints) > 0 {
			stepOut["saveHints"] = hints
		}

		var openapi OpenAPI
		switch step.Surface {
		case "admin":
			openapi = admin
		case "store":
			openapi = store
		}
		if details := findRouteDetails(openapi, step.Route, step.Method); details != nil {
			stepOut["routeDetails"] = details
		}

		if !strings.Contains(step.Route, " or ") {
			stepOut["request"] = map[string]any{
				"route":    routeWithSurfacePrefix(step.Surface, step.Route),
				"method":   strings.ToUpper(step.Method),
				"language": language,
				"payload":  starterPayloadForRoute(step.Route),
				"example": requestExampleForSurface(baseURL, RequestExampleInput{
					Route:    routeWithSurfacePrefix(step.Surface, step.Route),
					Method:   step.Method,
					Language: language,
					Payload:  starterPayloadForRoute(step.Route),
				}, step.Surface),
			}
		}

		if vars := flowStepVariableTokens(step); len(vars) > 0 {
			stepOut["variableTokens"] = vars
			for _, variable := range vars {
				if !seenVars[variable] {
					seenVars[variable] = true
					knownVars = append(knownVars, variable)
				}
			}
		}

		if len(knownVars) > 0 {
			stepOut["knownVariablesAfterStep"] = append([]string(nil), knownVars...)
		}
		steps = append(steps, stepOut)
	}

	return map[string]any{
		"name":          flow.Name,
		"surface":       flow.Surface,
		"summary":       flow.Summary,
		"language":      language,
		"prerequisites": flow.Prerequisites,
		"requests":      steps,
	}
}

func exportAssessmentReport(useCase string, format string, language string, baseURL string, admin OpenAPI, store OpenAPI) any {
	if strings.TrimSpace(format) == "" {
		format = "json"
	}

	openapiReport := analyzeOpenAPI(&admin, &store, "both")
	workflowReport := assessWorkflowSupport(useCase, admin, store)
	checklistReport := generateFlowChecklist(useCase, language, baseURL, admin, store)
	requestPackReport := generateFlowRequestPack(useCase, language, baseURL, admin, store)

	report := map[string]any{
		"useCase":         useCase,
		"openapiAnalysis": openapiReport,
		"workflowSupport": workflowReport,
		"flowChecklist":   checklistReport,
		"flowRequestPack": requestPackReport,
	}

	if strings.EqualFold(format, "markdown") {
		var b strings.Builder
		b.WriteString("# Assessment Report\n\n")
		if strings.TrimSpace(useCase) != "" {
			b.WriteString("## Use Case\n\n")
			b.WriteString(useCase)
			b.WriteString("\n\n")
		}

		if analysis, ok := openapiReport["analysis"].([]map[string]any); ok {
			b.WriteString("## OpenAPI Analysis\n\n")
			for _, item := range analysis {
				summary, _ := item["summary"].(map[string]any)
				apiType := fmt.Sprint(summary["apiType"])
				b.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(apiType)))
				b.WriteString(fmt.Sprintf("- Score: `%v`\n", summary["score"]))
				b.WriteString(fmt.Sprintf("- Rating: `%v`\n", summary["rating"]))
				b.WriteString(fmt.Sprintf("- Paths: `%v`\n", summary["paths"]))
				b.WriteString(fmt.Sprintf("- Operations: `%v`\n", summary["operations"]))
				b.WriteString(fmt.Sprintf("- Missing operationIds: `%v`\n", summary["missingOperationIds"]))
				b.WriteString(fmt.Sprintf("- Missing summaries: `%v`\n", summary["missingSummaries"]))
				b.WriteString(fmt.Sprintf("- Missing responses: `%v`\n\n", summary["missingResponses"]))
			}
		}

		if workflows, ok := workflowReport["workflowAssessment"].([]map[string]any); ok {
			b.WriteString("## Workflow Support\n\n")
			for _, item := range workflows {
				b.WriteString(fmt.Sprintf("### %s\n\n", item["name"]))
				b.WriteString(fmt.Sprintf("- Score: `%v`\n", item["score"]))
				b.WriteString(fmt.Sprintf("- Rating: `%v`\n", item["rating"]))
				b.WriteString(fmt.Sprintf("- Coverage: `%v%%`\n", item["coveragePercent"]))
				b.WriteString(fmt.Sprintf("- Auth clarity: `%v`\n", item["authClarity"]))
				if gaps, ok := item["contractGaps"].([]string); ok && len(gaps) > 0 {
					b.WriteString("- Contract gaps:\n")
					for _, gap := range gaps {
						b.WriteString(fmt.Sprintf("  - %s\n", gap))
					}
				}
				b.WriteString("\n")
			}
		}

		if checklistName, ok := checklistReport["name"]; ok {
			b.WriteString("## Flow Checklist\n\n")
			b.WriteString(fmt.Sprintf("- Flow: `%v`\n", checklistName))
			if steps, ok := checklistReport["checklist"].([]map[string]any); ok {
				for _, step := range steps {
					b.WriteString(fmt.Sprintf("- Step %v: `%s %s` - %s\n", step["stepNumber"], step["method"], step["route"], step["purpose"]))
				}
			}
			b.WriteString("\n")
		}

		if requestPackName, ok := requestPackReport["name"]; ok {
			b.WriteString("## Flow Request Pack\n\n")
			b.WriteString(fmt.Sprintf("- Flow: `%v`\n", requestPackName))
			if steps, ok := requestPackReport["requests"].([]map[string]any); ok {
				for _, step := range steps {
					b.WriteString(fmt.Sprintf("- Step %v: `%s %s`\n", step["stepNumber"], step["method"], step["route"]))
				}
			}
			b.WriteString("\n")
		}

		return map[string]any{
			"format":  "markdown",
			"content": b.String(),
		}
	}

	return map[string]any{
		"format": "json",
		"report": report,
	}
}

func explainSurface(useCase string) string {
	text := strings.ToLower(useCase)

	switch {
	case strings.Contains(text, "search criteria"), strings.Contains(text, "discoverab"), strings.Contains(text, "search behavior"):
		return "Best fit: both Admin API and Store API. Use Admin API to inspect entity search criteria and schema-backed search inputs, then compare with Store API listing or search behavior if shopper-facing discoverability is part of the problem."
	case strings.Contains(text, "checkout"), strings.Contains(text, "cart"), strings.Contains(text, "storefront"), strings.Contains(text, "customer-facing"):
		return "Best fit: Store API. Pair it with a storefront or plugin extension if you also need in-platform rendering or behavior changes."
	case strings.Contains(text, "admin ui"), strings.Contains(text, "administration ui"), strings.Contains(text, "merchant backend"):
		return "Best fit: Administration extension plus Admin API. Choose plugin or app depending on distribution and permission constraints."
	case strings.Contains(text, "erp"), strings.Contains(text, "catalog import"), strings.Contains(text, "entity sync"), strings.Contains(text, "back office"):
		return "Best fit: Admin API integration."
	default:
		return "Default guidance: use Admin API for back-office entity operations, Store API for shopper-facing flows, and plugin/app extensions when you need to change Shopware behavior or UI."
	}
}

func explainAuth(apiType string, route string) string {
	switch strings.ToLower(apiType) {
	case "admin":
		return "Admin API requests usually require a Bearer token. In this server, a local JSON fallback can also be used for static discovery data."
	case "store":
		return "Store API requests usually need a sales channel access key via sw-access-key. In this server, a local JSON fallback can also be used for static discovery data."
	default:
		return "Unknown apiType. Use admin or store."
	}
}

func containsPath(openapi OpenAPI, needle string) bool {
	_, ok := openapi.Paths[needle]
	return ok
}

func routeExists(openapi OpenAPI, route string) bool {
	normalizedNeedle := normalizeRouteValue(route)
	for path := range openapi.Paths {
		if normalizeRouteValue(path) == normalizedNeedle {
			return true
		}
	}
	return false
}

func detectRouteSurface(route string, admin OpenAPI, store OpenAPI) string {
	normalizedRoute := normalizeRouteValue(route)
	switch {
	case strings.HasPrefix(normalizedRoute, "/store-api/"), strings.HasPrefix(route, "/store-api/"):
		return "store"
	case strings.HasPrefix(normalizedRoute, "/api/"), strings.HasPrefix(route, "/api/"):
		return "admin"
	}

	adminMatch := routeExists(admin, route)
	storeMatch := routeExists(store, route)
	switch {
	case storeMatch && !adminMatch:
		return "store"
	case adminMatch && !storeMatch:
		return "admin"
	case strings.HasPrefix(normalizedRoute, "/checkout/"), strings.HasPrefix(normalizedRoute, "/account/"), normalizedRoute == "/search", normalizedRoute == "/search-suggest":
		return "store"
	default:
		return ""
	}
}

func routeOptions(route string) []string {
	raw := strings.TrimSpace(route)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, " or ")
	options := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			options = append(options, part)
		}
	}
	return options
}

func findRouteDetails(openapi OpenAPI, route string, method string) map[string]any {
	options := routeOptions(route)
	if len(options) == 0 {
		return nil
	}
	for _, option := range options {
		matches := describeRoute(openapi, option)
		if len(matches) == 0 {
			continue
		}
		if method == "" {
			return matches[0]
		}
		for _, match := range matches {
			if strings.EqualFold(fmt.Sprint(match["method"]), method) {
				return match
			}
		}
		return matches[0]
	}
	return nil
}

func severityWeight(severity string) int {
	switch severity {
	case "error":
		return 12
	case "warning":
		return 5
	default:
		return 2
	}
}

func scoreLabel(score int) string {
	switch {
	case score >= 90:
		return "excellent"
	case score >= 75:
		return "good"
	case score >= 55:
		return "fair"
	default:
		return "poor"
	}
}

func analyzeSingleOpenAPI(openapi OpenAPI, apiType string) map[string]any {
	findings := make([]openAPIFinding, 0)
	operationCount := 0
	missingOperationID := 0
	missingSummary := 0
	missingDescription := 0
	missingTags := 0
	missingResponses := 0
	duplicateOperationIDs := map[string]int{}
	seenOperationIDs := map[string]bool{}

	for path, methods := range openapi.Paths {
		for method, op := range methods {
			operationCount++
			upperMethod := strings.ToUpper(method)
			if strings.TrimSpace(op.OperationID) == "" {
				missingOperationID++
				findings = append(findings, openAPIFinding{
					Severity: "warning",
					Code:     "missing_operation_id",
					Message:  "Operation is missing operationId",
					Path:     path,
					Method:   upperMethod,
				})
			} else {
				if seenOperationIDs[op.OperationID] {
					duplicateOperationIDs[op.OperationID]++
				}
				seenOperationIDs[op.OperationID] = true
			}

			if strings.TrimSpace(op.Summary) == "" {
				missingSummary++
				findings = append(findings, openAPIFinding{
					Severity: "warning",
					Code:     "missing_summary",
					Message:  "Operation is missing summary",
					Path:     path,
					Method:   upperMethod,
				})
			}
			if strings.TrimSpace(op.Description) == "" {
				missingDescription++
			}
			if len(op.Tags) == 0 {
				missingTags++
				findings = append(findings, openAPIFinding{
					Severity: "info",
					Code:     "missing_tags",
					Message:  "Operation has no tags",
					Path:     path,
					Method:   upperMethod,
				})
			}
			if len(op.Responses) == 0 {
				missingResponses++
				findings = append(findings, openAPIFinding{
					Severity: "error",
					Code:     "missing_responses",
					Message:  "Operation has no documented responses",
					Path:     path,
					Method:   upperMethod,
				})
			}
			if (upperMethod == "POST" || upperMethod == "PATCH" || upperMethod == "PUT") && op.RequestBody == nil {
				findings = append(findings, openAPIFinding{
					Severity: "info",
					Code:     "missing_request_body",
					Message:  "Mutation operation has no documented request body",
					Path:     path,
					Method:   upperMethod,
				})
			}
		}
	}

	for operationID, count := range duplicateOperationIDs {
		findings = append(findings, openAPIFinding{
			Severity: "error",
			Code:     "duplicate_operation_id",
			Message:  fmt.Sprintf("operationId %q is duplicated %d extra time(s)", operationID, count),
		})
	}

	workflowRisks := make([]openAPIFinding, 0)
	if apiType == "admin" {
		if !containsPath(openapi, "/search/product") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "warning",
				Code:     "missing_product_search_route",
				Message:  "Admin OpenAPI does not expose /search/product, which weakens Criteria workflow support.",
			})
		}
		if !containsPath(openapi, "/search/category") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "warning",
				Code:     "missing_category_search_route",
				Message:  "Admin OpenAPI does not expose /search/category, which weakens Criteria workflow support.",
			})
		}
		if !containsPath(openapi, "/product") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "warning",
				Code:     "missing_product_crud_route",
				Message:  "Admin OpenAPI does not expose /product, which weakens catalog setup and checkout walkthroughs.",
			})
		}
		if !containsPath(openapi, "/category") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "warning",
				Code:     "missing_category_crud_route",
				Message:  "Admin OpenAPI does not expose /category, which weakens catalog setup walkthroughs.",
			})
		}
		hasCapabilityMetadata := false
		for path := range openapi.Paths {
			if strings.Contains(path, "capabilit") || strings.Contains(path, "search-field") || strings.Contains(path, "sortable") || strings.Contains(path, "filterable") {
				hasCapabilityMetadata = true
				break
			}
		}
		if !hasCapabilityMetadata && containsPath(openapi, "/search/product") && containsPath(openapi, "/search/category") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "info",
				Code:     "heuristic_missing_entity_capability_metadata",
				Message:  "Heuristic: no dedicated contract metadata was found for searchable/filterable/sortable Product and Category fields.",
			})
		}
	}
	if apiType == "store" {
		if !containsPath(openapi, "/checkout/order") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "warning",
				Code:     "missing_checkout_order_route",
				Message:  "Store OpenAPI does not expose /checkout/order, which weakens checkout workflow support.",
			})
		}
		if !containsPath(openapi, "/account/register") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "info",
				Code:     "missing_account_register_route",
				Message:  "Store OpenAPI does not expose /account/register, which limits customer onboarding workflows.",
			})
		}
		if !containsPath(openapi, "/order") {
			workflowRisks = append(workflowRisks, openAPIFinding{
				Severity: "info",
				Code:     "missing_order_history_route",
				Message:  "Store OpenAPI does not expose /order, which limits order-history workflows.",
			})
		}
	}

	summary := map[string]any{
		"apiType":             apiType,
		"paths":               len(openapi.Paths),
		"operations":          operationCount,
		"missingOperationIds": missingOperationID,
		"missingSummaries":    missingSummary,
		"missingDescriptions": missingDescription,
		"missingTags":         missingTags,
		"missingResponses":    missingResponses,
	}

	score := 100
	deductionOperationIDs := missingOperationID * 3
	deductionSummaries := missingSummary * 2
	deductionDescriptions := missingDescription
	deductionTags := missingTags
	deductionResponses := missingResponses * 4
	deductionDuplicateOperationIDs := 0
	deductionWorkflowRisks := 0

	score -= deductionOperationIDs
	score -= deductionSummaries
	score -= deductionDescriptions
	score -= deductionTags
	score -= deductionResponses
	for _, count := range duplicateOperationIDs {
		deductionDuplicateOperationIDs += (count + 1) * 8
	}
	score -= deductionDuplicateOperationIDs
	for _, finding := range workflowRisks {
		deductionWorkflowRisks += severityWeight(finding.Severity)
	}
	score -= deductionWorkflowRisks
	if score < 0 {
		score = 0
	}
	summary["score"] = score
	summary["rating"] = scoreLabel(score)
	summary["scoreBreakdown"] = map[string]any{
		"baseScore":                    100,
		"missingOperationIdsPenalty":   deductionOperationIDs,
		"missingSummariesPenalty":      deductionSummaries,
		"missingDescriptionsPenalty":   deductionDescriptions,
		"missingTagsPenalty":           deductionTags,
		"missingResponsesPenalty":      deductionResponses,
		"duplicateOperationIdsPenalty": deductionDuplicateOperationIDs,
		"workflowRisksPenalty":         deductionWorkflowRisks,
		"finalScore":                   score,
	}

	findingMaps := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		findingMaps = append(findingMaps, map[string]any{
			"severity": f.Severity,
			"code":     f.Code,
			"message":  f.Message,
			"path":     f.Path,
			"method":   f.Method,
		})
	}
	workflowRiskMaps := make([]map[string]any, 0, len(workflowRisks))
	for _, f := range workflowRisks {
		workflowRiskMaps = append(workflowRiskMaps, map[string]any{
			"severity": f.Severity,
			"code":     f.Code,
			"message":  f.Message,
		})
	}

	return map[string]any{
		"summary":                summary,
		"findings":               findingMaps,
		"workflowRisks":          workflowRiskMaps,
		"workflowRiskCount":      len(workflowRiskMaps),
		"structuralFindingCount": len(findingMaps),
	}
}

func analyzeOpenAPI(admin *OpenAPI, store *OpenAPI, apiType string) map[string]any {
	apiType = strings.ToLower(strings.TrimSpace(apiType))
	if apiType == "" {
		apiType = "both"
	}
	results := make([]map[string]any, 0, 2)
	if (apiType == "admin" || apiType == "both") && admin != nil {
		results = append(results, analyzeSingleOpenAPI(*admin, "admin"))
	}
	if (apiType == "store" || apiType == "both") && store != nil {
		results = append(results, analyzeSingleOpenAPI(*store, "store"))
	}
	return map[string]any{
		"analysis": results,
	}
}

func titleCaseToken(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func capabilityEntities(in []string) []string {
	if len(in) == 0 {
		return []string{"product", "category"}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, entity := range in {
		entity = strings.ToLower(strings.TrimSpace(entity))
		if entity == "" || seen[entity] {
			continue
		}
		seen[entity] = true
		out = append(out, entity)
	}
	if len(out) == 0 {
		return []string{"product", "category"}
	}
	return out
}

func schemaErrorMessage(entitySchema any) string {
	root, ok := entitySchema.(map[string]any)
	if !ok {
		return ""
	}
	errors, ok := root["errors"].([]any)
	if !ok || len(errors) == 0 {
		return ""
	}
	first, ok := errors[0].(map[string]any)
	if !ok {
		return "entity schema payload contains errors"
	}
	status := strings.TrimSpace(fmt.Sprint(first["status"]))
	title := strings.TrimSpace(fmt.Sprint(first["title"]))
	detail := strings.TrimSpace(fmt.Sprint(first["detail"]))
	parts := make([]string, 0, 3)
	if status != "" {
		parts = append(parts, status)
	}
	if title != "" {
		parts = append(parts, title)
	}
	if detail != "" {
		parts = append(parts, detail)
	}
	return strings.Join(parts, " - ")
}

func schemaEntityEntry(entitySchema any, entity string) map[string]any {
	root, ok := entitySchema.(map[string]any)
	if !ok {
		return nil
	}
	candidates := []string{
		entity,
		strings.ToLower(entity),
		titleCaseToken(entity),
	}
	for _, key := range candidates {
		if entry, ok := root[key].(map[string]any); ok {
			return entry
		}
	}
	for _, parentKey := range []string{"entities", "definitions", "components", "schemas"} {
		parent, ok := root[parentKey].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range candidates {
			if entry, ok := parent[key].(map[string]any); ok {
				return entry
			}
		}
		if parentKey == "components" {
			if schemas, ok := parent["schemas"].(map[string]any); ok {
				for _, key := range candidates {
					if entry, ok := schemas[key].(map[string]any); ok {
						return entry
					}
				}
			}
		}
	}
	return nil
}

func recursiveAny(v any, fn func(key string, str string) bool) bool {
	switch value := v.(type) {
	case map[string]any:
		for key, nested := range value {
			if fn(key, "") {
				return true
			}
			if recursiveAny(nested, fn) {
				return true
			}
		}
	case []any:
		for _, nested := range value {
			if recursiveAny(nested, fn) {
				return true
			}
		}
	case string:
		return fn("", value)
	}
	return false
}

func recursiveContainsKey(v any, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return false
	}
	return recursiveAny(v, func(key string, str string) bool {
		return key != "" && strings.Contains(strings.ToLower(key), needle)
	})
}

func recursiveContainsAllStrings(v any, needles ...string) bool {
	if len(needles) == 0 {
		return false
	}
	found := map[string]bool{}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" {
			found[needle] = false
		}
	}
	if len(found) == 0 {
		return false
	}
	recursiveAny(v, func(key string, str string) bool {
		text := strings.ToLower(strings.TrimSpace(str))
		if text == "" {
			return false
		}
		for needle := range found {
			if strings.Contains(text, needle) {
				found[needle] = true
			}
		}
		return false
	})
	for _, ok := range found {
		if !ok {
			return false
		}
	}
	return true
}

func fieldPreview(entry map[string]any, limit int) []string {
	if entry == nil || limit <= 0 {
		return nil
	}
	candidates := []string{"properties", "fields", "attributes"}
	seen := map[string]bool{}
	out := make([]string, 0, limit)
	appendKeys := func(m map[string]any) {
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
			if len(out) == limit {
				return
			}
		}
	}
	for _, key := range candidates {
		if nested, ok := entry[key].(map[string]any); ok {
			appendKeys(nested)
			if len(out) == limit {
				return out
			}
		}
	}
	appendKeys(entry)
	return out
}

func analyzeEntitySearchCapabilities(entity string, admin OpenAPI, entitySchema any) map[string]any {
	entity = strings.ToLower(strings.TrimSpace(entity))
	searchRoute := "/search/" + entity
	aggregateRoute := "/aggregate/" + entity
	crudRoute := "/" + entity
	searchDetails := findRouteDetails(admin, searchRoute, "POST")
	schemaEntry := schemaEntityEntry(entitySchema, entity)
	schemaError := schemaErrorMessage(entitySchema)

	explicitSearchable := recursiveContainsKey(schemaEntry, "searchable")
	explicitFilterable := recursiveContainsKey(schemaEntry, "filterable")
	explicitSortable := recursiveContainsKey(schemaEntry, "sortable")
	termVsFilterGuidance := recursiveContainsAllStrings(schemaEntry, "term", "filter")
	if !termVsFilterGuidance && searchDetails != nil {
		termVsFilterGuidance = recursiveContainsAllStrings(searchDetails, "term", "filter")
	}

	requestBodyDocumented := searchDetails != nil && searchDetails["requestBody"] != nil
	descriptionQuality := "weak"
	if searchDetails != nil {
		summary := strings.TrimSpace(fmt.Sprint(searchDetails["summary"]))
		description := strings.TrimSpace(fmt.Sprint(searchDetails["description"]))
		switch {
		case summary != "" && description != "":
			descriptionQuality = "strong"
		case summary != "" || description != "":
			descriptionQuality = "partial"
		}
	}

	score := 100
	gaps := make([]string, 0)
	recommendations := make([]string, 0)

	if searchDetails == nil {
		score -= 30
		gaps = append(gaps, fmt.Sprintf("No %s Admin search route was found.", searchRoute))
		recommendations = append(recommendations, fmt.Sprintf("Expose %s clearly in the contract so developers can start from a known endpoint.", searchRoute))
	}
	if !containsPath(admin, crudRoute) {
		score -= 10
		gaps = append(gaps, fmt.Sprintf("No %s collection route was found for baseline entity CRUD context.", crudRoute))
	}
	if !containsPath(admin, aggregateRoute) {
		score -= 3
		recommendations = append(recommendations, fmt.Sprintf("Consider whether %s should be discoverable for analytics-style Criteria use cases.", aggregateRoute))
	}
	if schemaEntry == nil {
		score -= 20
		if schemaError != "" {
			gaps = append(gaps, fmt.Sprintf("Entity schema payload is not usable for %s capability discovery: %s.", entity, schemaError))
		} else {
			gaps = append(gaps, fmt.Sprintf("No entity-schema entry was found for %s.", entity))
		}
		recommendations = append(recommendations, fmt.Sprintf("Expose machine-readable schema metadata for %s so developers can inspect valid field names without guessing.", entity))
	}
	if !requestBodyDocumented {
		score -= 10
		gaps = append(gaps, fmt.Sprintf("%s is present but does not document a request body schema for Criteria.", searchRoute))
	}
	if descriptionQuality == "weak" {
		score -= 8
		gaps = append(gaps, fmt.Sprintf("%s has weak description coverage, so intent and usage are hard to infer from the contract alone.", searchRoute))
	} else if descriptionQuality == "partial" {
		score -= 4
	}
	if !explicitSearchable {
		score -= 10
		gaps = append(gaps, fmt.Sprintf("No explicit searchable-field metadata was found for %s.", entity))
		recommendations = append(recommendations, fmt.Sprintf("Add searchable-field metadata for %s to the schema or a dedicated discovery endpoint.", entity))
	}
	if !explicitFilterable {
		score -= 10
		gaps = append(gaps, fmt.Sprintf("No explicit filterable-field metadata was found for %s.", entity))
		recommendations = append(recommendations, fmt.Sprintf("Add filterable-field metadata for %s, especially for common Criteria jobs.", entity))
	}
	if !explicitSortable {
		score -= 10
		gaps = append(gaps, fmt.Sprintf("No explicit sortable-field metadata was found for %s.", entity))
		recommendations = append(recommendations, fmt.Sprintf("Add sortable-field metadata for %s so order-by choices are discoverable without trial and error.", entity))
	}
	if !termVsFilterGuidance {
		score -= 8
		gaps = append(gaps, fmt.Sprintf("The contract does not make the practical distinction between term and filter clear for %s.", entity))
		recommendations = append(recommendations, "Add contract-level guidance or validation hints that explain when to use term versus explicit filter operators.")
	}

	if score < 0 {
		score = 0
	}

	checks := map[string]any{
		"searchRoutePresent":               searchDetails != nil,
		"aggregateRoutePresent":            containsPath(admin, aggregateRoute),
		"crudCollectionRoutePresent":       containsPath(admin, crudRoute),
		"requestBodyDocumented":            requestBodyDocumented,
		"entitySchemaPresent":              schemaEntry != nil,
		"entitySchemaError":                schemaError,
		"explicitSearchableFieldsMetadata": explicitSearchable,
		"explicitFilterableFieldsMetadata": explicitFilterable,
		"explicitSortableFieldsMetadata":   explicitSortable,
		"termVsFilterGuidanceInContract":   termVsFilterGuidance,
		"searchRouteDescriptionQuality":    descriptionQuality,
	}

	result := map[string]any{
		"entity":          entity,
		"score":           score,
		"rating":          scoreLabel(score),
		"checks":          checks,
		"gaps":            gaps,
		"recommendations": recommendations,
	}
	if searchDetails != nil {
		result["searchRouteDetails"] = searchDetails
	}
	if preview := fieldPreview(schemaEntry, 12); len(preview) > 0 {
		result["fieldPreview"] = preview
	}
	return result
}

func analyzeSearchCapabilities(entities []string, admin OpenAPI, entitySchema any) map[string]any {
	targets := capabilityEntities(entities)
	results := make([]map[string]any, 0, len(targets))
	totalScore := 0
	for _, entity := range targets {
		item := analyzeEntitySearchCapabilities(entity, admin, entitySchema)
		results = append(results, item)
		totalScore += item["score"].(int)
	}
	sort.Slice(results, func(i, j int) bool {
		return fmt.Sprint(results[i]["entity"]) < fmt.Sprint(results[j]["entity"])
	})
	averageScore := 0
	if len(results) > 0 {
		averageScore = totalScore / len(results)
	}
	return map[string]any{
		"summary": map[string]any{
			"entityCount":   len(results),
			"averageScore":  averageScore,
			"rating":        scoreLabel(averageScore),
			"analyzerScope": "search_capability_discoverability",
		},
		"entities": results,
	}
}

func assessSingleFlow(flow curatedFlow, admin OpenAPI, store OpenAPI) map[string]any {
	requiredRoutesFound := make([]map[string]any, 0)
	missingRoutes := make([]map[string]any, 0)
	weaklyDocumentedRoutes := make([]map[string]any, 0)
	hiddenPrerequisiteRoutes := make([]map[string]any, 0)
	contractGaps := make([]string, 0)

	score := 100
	requiredStepCount := 0
	foundRequiredStepCount := 0
	authClarity := "clear"

	for _, step := range flow.Steps {
		if !step.Optional {
			requiredStepCount++
		}

		var openapi OpenAPI
		switch step.Surface {
		case "admin":
			openapi = admin
		case "store":
			openapi = store
		default:
			authClarity = "partial"
			score -= 5
		}

		details := findRouteDetails(openapi, step.Route, step.Method)
		if details == nil {
			item := map[string]any{
				"surface":  step.Surface,
				"method":   step.Method,
				"route":    step.Route,
				"purpose":  step.Purpose,
				"optional": step.Optional,
			}
			missingRoutes = append(missingRoutes, item)
			if !step.Optional {
				score -= 15
			} else {
				score -= 5
			}
			if len(step.Notes) > 0 {
				contractGaps = append(contractGaps, fmt.Sprintf("Flow relies on curated guidance for %s because no matching route was found for %s.", step.Purpose, step.Route))
			}
			continue
		}

		item := map[string]any{
			"surface":     step.Surface,
			"method":      step.Method,
			"route":       step.Route,
			"purpose":     step.Purpose,
			"routeDetail": details,
			"optional":    step.Optional,
		}
		requiredRoutesFound = append(requiredRoutesFound, item)
		if !step.Optional {
			foundRequiredStepCount++
		}

		summary := strings.TrimSpace(fmt.Sprint(details["summary"]))
		description := strings.TrimSpace(fmt.Sprint(details["description"]))
		if summary == "" || description == "" {
			weaklyDocumentedRoutes = append(weaklyDocumentedRoutes, map[string]any{
				"route":       step.Route,
				"method":      step.Method,
				"summary":     summary,
				"description": description,
				"reason":      "Route is present but summary or description is weak/missing.",
			})
			score -= 4
		}

		if len(step.Notes) > 0 {
			hiddenPrerequisiteRoutes = append(hiddenPrerequisiteRoutes, map[string]any{
				"route":  step.Route,
				"method": step.Method,
				"notes":  step.Notes,
			})
			score -= 1
			contractGaps = append(contractGaps, fmt.Sprintf("Flow step %s depends on curated notes, suggesting the OpenAPI contract does not capture all prerequisites.", step.Route))
		}

		upperMethod := strings.ToUpper(step.Method)
		if (upperMethod == "POST" || upperMethod == "PATCH" || upperMethod == "PUT") && details["requestBody"] == nil {
			contractGaps = append(contractGaps, fmt.Sprintf("Route %s (%s) is used in the flow but has no documented request body.", step.Route, step.Method))
			score -= 3
		}
	}

	if len(flow.Prerequisites) > 0 || len(flow.CommonFailureReasons) > 0 || len(flow.DiagnosticChecks) > 0 {
		score -= 0
	} else {
		contractGaps = append(contractGaps, "Flow has little or no prerequisite/failure guidance, so more knowledge is implied outside the contract.")
		score -= 8
	}

	if strings.Contains(flow.Name, "search_criteria") {
		contractGaps = append(contractGaps, "Search/Criteria support still depends on curated guidance because searchable/filterable/sortable field metadata is not explicitly modeled.")
		score -= 5
	}
	if strings.Contains(flow.Name, "discoverability") {
		contractGaps = append(contractGaps, "Store discoverability still depends on inferred catalog prerequisites such as visibility, active state, and sales-channel alignment.")
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	coveragePercent := 100
	if requiredStepCount > 0 {
		coveragePercent = int(float64(foundRequiredStepCount) / float64(requiredStepCount) * 100)
	}

	return map[string]any{
		"name":                     flow.Name,
		"surface":                  flow.Surface,
		"summary":                  flow.Summary,
		"score":                    score,
		"rating":                   scoreLabel(score),
		"requiredStepCount":        requiredStepCount,
		"requiredRoutesCovered":    foundRequiredStepCount,
		"coveragePercent":          coveragePercent,
		"authClarity":              authClarity,
		"requiredRoutesFound":      requiredRoutesFound,
		"missingRoutes":            missingRoutes,
		"weaklyDocumentedRoutes":   weaklyDocumentedRoutes,
		"hiddenPrerequisiteRoutes": hiddenPrerequisiteRoutes,
		"contractGaps":             contractGaps,
	}
}

func assessWorkflowSupport(useCase string, admin OpenAPI, store OpenAPI) map[string]any {
	flows, err := loadCuratedFlows("data/flows")
	if err != nil {
		return map[string]any{
			"error": err.Error(),
		}
	}

	results := make([]map[string]any, 0)
	if strings.TrimSpace(useCase) != "" {
		if flow := selectCuratedFlow(useCase, flows); flow != nil {
			results = append(results, assessSingleFlow(*flow, admin, store))
		}
	} else {
		for _, flow := range flows {
			results = append(results, assessSingleFlow(flow, admin, store))
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i]["name"].(string) < results[j]["name"].(string)
	})

	return map[string]any{
		"workflowAssessment": results,
	}
}

func enrichFlowSteps(steps []flowStep, admin OpenAPI, store OpenAPI) []map[string]any {
	out := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		item := map[string]any{
			"surface": step.Surface,
			"method":  step.Method,
			"route":   step.Route,
			"purpose": step.Purpose,
		}
		if len(step.Notes) > 0 {
			item["notes"] = step.Notes
		}
		if step.Optional {
			item["optional"] = true
		}

		var openapi OpenAPI
		switch step.Surface {
		case "admin":
			openapi = admin
		case "store":
			openapi = store
		}

		if len(openapi.Paths) > 0 {
			matches := describeRoute(openapi, step.Route)
			if len(matches) > 0 {
				item["routeDetails"] = matches[0]
			}
		}

		out = append(out, item)
	}
	return out
}

func loadCuratedFlows(dir string) ([]curatedFlow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var flows []curatedFlow
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var flow curatedFlow
		if err := json.Unmarshal(raw, &flow); err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}
	return flows, nil
}

func scoreCuratedFlow(useCase string, flow curatedFlow) int {
	text := strings.ToLower(strings.TrimSpace(useCase))
	if text == "" {
		return 0
	}
	tokenSet := tokenizeRouteText(text)
	score := 0
	for _, trigger := range flow.Triggers {
		trigger = strings.ToLower(strings.TrimSpace(trigger))
		if trigger == "" {
			continue
		}
		if strings.Contains(text, trigger) {
			score += len(strings.Fields(trigger))*10 + len(trigger) + 20
		}
		triggerTokens := tokenizeRouteText(trigger)
		overlap := 0
		for token := range triggerTokens {
			if tokenSet[token] {
				overlap++
			}
		}
		if overlap > 0 {
			score += overlap * 6
		}
	}

	nameTokens := tokenizeRouteText(flow.Name)
	nameOverlap := 0
	for token := range nameTokens {
		if tokenSet[token] {
			nameOverlap++
		}
	}
	score += nameOverlap * 5

	summaryTokens := tokenizeRouteText(flow.Summary)
	summaryOverlap := 0
	for token := range summaryTokens {
		if tokenSet[token] {
			summaryOverlap++
		}
	}
	score += summaryOverlap * 2

	for _, step := range flow.Steps {
		stepTokens := tokenizeRouteText(step.Route, step.Purpose, step.Method, step.Surface)
		stepOverlap := 0
		for token := range stepTokens {
			if tokenSet[token] {
				stepOverlap++
			}
		}
		if stepOverlap > 0 {
			score += stepOverlap
		}
	}

	if strings.Contains(text, strings.ToLower(flow.Name)) {
		score += 25
	}

	if score > 0 {
		if flow.Surface == "both" && (tokenSet["admin"] || tokenSet["store"]) {
			score -= 2
		}
		if flow.Surface == "admin" && tokenSet["store"] {
			score -= 2
		}
		if flow.Surface == "store" && tokenSet["admin"] {
			score -= 2
		}
	}
	return score
}

func selectCuratedFlow(useCase string, flows []curatedFlow) *curatedFlow {
	bestScore := 0
	var best *curatedFlow
	for i := range flows {
		score := scoreCuratedFlow(useCase, flows[i])
		if score > bestScore {
			bestScore = score
			best = &flows[i]
		}
	}
	return best
}

func renderCuratedFlow(flow curatedFlow, admin OpenAPI, store OpenAPI) map[string]any {
	result := map[string]any{
		"name":       flow.Name,
		"confidence": flow.Confidence,
		"surface":    flow.Surface,
		"summary":    flow.Summary,
		"steps":      enrichFlowSteps(flow.Steps, admin, store),
	}
	if len(flow.Prerequisites) > 0 {
		result["prerequisites"] = flow.Prerequisites
	}
	if len(flow.CommonFailureReasons) > 0 {
		result["commonFailureReasons"] = flow.CommonFailureReasons
	}
	if len(flow.DiagnosticChecks) > 0 {
		result["diagnosticChecks"] = flow.DiagnosticChecks
	}
	if len(flow.RecommendedExamples) > 0 {
		result["recommendedExamples"] = flow.RecommendedExamples
	}
	return result
}

func explainFlow(useCase string, admin OpenAPI, store OpenAPI) any {
	flows, err := loadCuratedFlows("data/flows")
	if err == nil {
		if flow := selectCuratedFlow(useCase, flows); flow != nil {
			return renderCuratedFlow(*flow, admin, store)
		}
	}
	return map[string]any{
		"name":       "generic_flow_guidance",
		"confidence": "low",
		"summary":    "No curated flow matched exactly. Use find_routes for route discovery and explain_surface to choose between Admin API, Store API, or extension work.",
	}
}

func toolResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func main() {
	cfg := loadConfig()
	client := newShopwareClient(cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_admin_routes",
		Description: "List Shopware Admin API routes from live Shopware or a local fallback file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NoInput) (*mcp.CallToolResult, any, error) {
		var out any
		source, err := client.fetchOrLoadJSON(ctx, "/api/_info/routes", "admin", cfg.AdminRoutesFile, &out)
		if err != nil {
			return toolResult("Failed to fetch admin routes: " + err.Error()), nil, nil
		}
		return toolResult(withSourceLabel(source, marshalPretty(out))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_store_routes",
		Description: "List Shopware Store API routes from live Shopware or a local fallback file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NoInput) (*mcp.CallToolResult, any, error) {
		var out any
		source, err := client.fetchOrLoadJSON(ctx, "/store-api/_info/routes", "store", cfg.StoreRoutesFile, &out)
		if err != nil {
			return toolResult("Failed to fetch store routes: " + err.Error()), nil, nil
		}
		return toolResult(withSourceLabel(source, marshalPretty(out))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_admin_openapi",
		Description: "Fetch the Shopware Admin API OpenAPI document from live Shopware or a local fallback file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NoInput) (*mcp.CallToolResult, any, error) {
		var out any
		source, err := client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &out)
		if err != nil {
			return toolResult("Failed to fetch admin OpenAPI: " + err.Error()), nil, nil
		}
		return toolResult(withSourceLabel(source, marshalPretty(out))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_store_openapi",
		Description: "Fetch the Shopware Store API OpenAPI document from live Shopware or a local fallback file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NoInput) (*mcp.CallToolResult, any, error) {
		var out any
		source, err := client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &out)
		if err != nil {
			return toolResult("Failed to fetch store OpenAPI: " + err.Error()), nil, nil
		}
		return toolResult(withSourceLabel(source, marshalPretty(out))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_entity_schema",
		Description: "Fetch the Shopware Admin entity schema document from live Shopware or a local fallback file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NoInput) (*mcp.CallToolResult, any, error) {
		var out any
		source, err := client.fetchOrLoadJSON(ctx, "/api/_info/entity-schema.json", "admin", cfg.EntitySchemaFile, &out)
		if err != nil {
			return toolResult("Failed to fetch entity schema: " + err.Error()), nil, nil
		}
		return toolResult(withSourceLabel(source, marshalPretty(out))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_routes",
		Description: "Search route metadata in Shopware Admin and Store OpenAPI documents",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input FindRoutesInput) (*mcp.CallToolResult, any, error) {
		apiType := strings.ToLower(strings.TrimSpace(input.APIType))
		if apiType == "" {
			apiType = "both"
		}

		var matches []map[string]any

		if apiType == "admin" || apiType == "both" {
			var admin OpenAPI
			_, err := client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
			if err == nil {
				matches = append(matches, findMatches(admin, input.Query, "admin")...)
			}
		}

		if apiType == "store" || apiType == "both" {
			var store OpenAPI
			_, err := client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
			if err == nil {
				matches = append(matches, findMatches(store, input.Query, "store")...)
			}
		}

		return toolResult(marshalPretty(matches)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_route",
		Description: "Describe a Shopware route using the OpenAPI document",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input DescribeRouteInput) (*mcp.CallToolResult, any, error) {
		apiType := strings.ToLower(strings.TrimSpace(input.APIType))
		if apiType != "admin" && apiType != "store" {
			return toolResult("apiType must be admin or store"), nil, nil
		}

		var openapi OpenAPI
		livePath := "/api/_info/openapi3.json"
		filePath := cfg.AdminOpenAPIFile
		if apiType == "store" {
			livePath = "/store-api/_info/openapi3.json"
			filePath = cfg.StoreOpenAPIFile
		}

		source, err := client.fetchOrLoadJSON(ctx, livePath, apiType, filePath, &openapi)
		if err != nil {
			return toolResult("Failed to fetch OpenAPI: " + err.Error()), nil, nil
		}

		matches := describeRoute(openapi, input.PathOrName)
		if len(matches) == 0 {
			return toolResult("No matching route found."), nil, nil
		}

		return toolResult(withSourceLabel(source, marshalPretty(matches))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_criteria_payload",
		Description: "Generate a starter Shopware Criteria payload for Admin API search use cases",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input CriteriaInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(input.Entity) == "" {
			return toolResult("entity is required"), nil, nil
		}
		return toolResult(marshalPretty(buildCriteriaPayload(input))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_api_request_example",
		Description: "Generate a Shopware API request example in curl or js",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input RequestExampleInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		surface := detectRouteSurface(input.Route, admin, store)
		return toolResult(requestExampleForSurface(cfg.BaseURL, input, surface)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "explain_surface",
		Description: "Explain whether a use case fits Admin API, Store API, plugin, app, or Administration extension work",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ExplainSurfaceInput) (*mcp.CallToolResult, any, error) {
		return toolResult(explainSurface(input.UseCase)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "explain_auth",
		Description: "Explain likely authentication requirements for Shopware Admin API or Store API usage",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ExplainAuthInput) (*mcp.CallToolResult, any, error) {
		return toolResult(explainAuth(input.APIType, input.Route)), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "explain_flow",
		Description: "Explain a likely Shopware API flow for a use case, including ordered steps, matched route details, and common prerequisites",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ExplainFlowInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		return toolResult(marshalPretty(explainFlow(input.UseCase, admin, store))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_openapi",
		Description: "Run structural OpenAPI diagnostics and workflow-support heuristics against the bundled or live Shopware API specifications",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeOpenAPIInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		apiType := strings.ToLower(strings.TrimSpace(input.APIType))
		if apiType == "" {
			apiType = "both"
		}
		if apiType == "admin" || apiType == "both" {
			_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		}
		if apiType == "store" || apiType == "both" {
			_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		}
		return toolResult(marshalPretty(analyzeOpenAPI(&admin, &store, apiType))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "analyze_search_capabilities",
		Description: "Assess how discoverable Product and Category search capabilities are in the Shopware contract, including field-capability metadata gaps and term-vs-filter clarity",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeSearchCapabilitiesInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var entitySchema any
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/entity-schema.json", "admin", cfg.EntitySchemaFile, &entitySchema)
		return toolResult(marshalPretty(analyzeSearchCapabilities(input.Entities, admin, entitySchema))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assess_workflow_support",
		Description: "Assess how well the current Shopware contract supports curated developer workflows, including route coverage, gaps, and workflow-level scores",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input AssessWorkflowSupportInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		return toolResult(marshalPretty(assessWorkflowSupport(input.UseCase, admin, store))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_flow_checklist",
		Description: "Generate a step-by-step manual execution checklist for a curated workflow, including request examples and variable handoff hints",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GenerateFlowChecklistInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		return toolResult(marshalPretty(generateFlowChecklist(input.UseCase, input.Language, input.BaseURL, admin, store))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_flow_request_pack",
		Description: "Generate a flow-aware starter request pack for a curated workflow, including per-step payloads, examples, and variable handoff context",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GenerateFlowRequestPackInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		return toolResult(marshalPretty(generateFlowRequestPack(input.UseCase, input.Language, input.BaseURL, admin, store))), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_assessment_report",
		Description: "Export a combined contract and workflow assessment report in stable JSON or Markdown format",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ExportAssessmentReportInput) (*mcp.CallToolResult, any, error) {
		var admin OpenAPI
		var store OpenAPI
		_, _ = client.fetchOrLoadJSON(ctx, "/api/_info/openapi3.json", "admin", cfg.AdminOpenAPIFile, &admin)
		_, _ = client.fetchOrLoadJSON(ctx, "/store-api/_info/openapi3.json", "store", cfg.StoreOpenAPIFile, &store)
		return toolResult(marshalPretty(exportAssessmentReport(input.UseCase, input.Format, input.Language, input.BaseURL, admin, store))), nil, nil
	})

	log.Printf("starting %s server", serverName)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
