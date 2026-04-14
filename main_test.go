package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func findStepByRoute(t *testing.T, steps []map[string]any, route string) map[string]any {
	t.Helper()
	for _, step := range steps {
		if step["route"] == route {
			return step
		}
	}
	t.Fatalf("step with route %s not found in %v", route, steps)
	return nil
}

func loadOpenAPIForTest(t *testing.T, path string) OpenAPI {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var openapi OpenAPI
	if err := json.Unmarshal(raw, &openapi); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	if len(openapi.Paths) == 0 {
		t.Fatalf("expected paths in %s", path)
	}
	return openapi
}

func loadJSONForTest(t *testing.T, path string) any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func TestLoadCuratedFlows(t *testing.T) {
	flows, err := loadCuratedFlows("data/flows")
	if err != nil {
		t.Fatalf("load curated flows: %v", err)
	}
	if len(flows) < 5 {
		t.Fatalf("expected at least 5 curated flows, got %d", len(flows))
	}
}

func TestLoadConfigDefaultsToBundledFiles(t *testing.T) {
	t.Setenv("SHOPWARE_BASE_URL", "")
	t.Setenv("SHOPWARE_ADMIN_TOKEN", "")
	t.Setenv("SHOPWARE_STORE_ACCESS_KEY", "")
	t.Setenv("SHOPWARE_PREFER_LIVE_DATA", "")
	t.Setenv("SHOPWARE_ADMIN_OPENAPI_FILE", "")
	t.Setenv("SHOPWARE_STORE_OPENAPI_FILE", "")
	t.Setenv("SHOPWARE_ENTITY_SCHEMA_FILE", "")
	t.Setenv("SHOPWARE_ADMIN_ROUTES_FILE", "")
	t.Setenv("SHOPWARE_STORE_ROUTES_FILE", "")

	cfg := loadConfig()
	if got := cfg.AdminOpenAPIFile; got != "data/admin-openapi.json" {
		t.Fatalf("expected default admin openapi file, got %q", got)
	}
	if got := cfg.StoreOpenAPIFile; got != "data/store-openapi.json" {
		t.Fatalf("expected default store openapi file, got %q", got)
	}
	if got := cfg.EntitySchemaFile; got != "data/entity-schema.json" {
		t.Fatalf("expected default entity schema file, got %q", got)
	}
	if got := cfg.AdminRoutesFile; got != "data/admin-routes.json" {
		t.Fatalf("expected default admin routes file, got %q", got)
	}
	if got := cfg.StoreRoutesFile; got != "data/store-routes.json" {
		t.Fatalf("expected default store routes file, got %q", got)
	}
}

func TestAnalyzeSingleOpenAPIFindings(t *testing.T) {
	openapi := OpenAPI{
		Paths: map[string]map[string]OpenAPIOperation{
			"/product": {
				"post": {
					Summary: "",
					Tags:    nil,
				},
			},
		},
	}
	result := analyzeSingleOpenAPI(openapi, "admin")
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary, got %v", result["summary"])
	}
	if got := summary["missingOperationIds"]; got != 1 {
		t.Fatalf("expected 1 missing operationId, got %v", got)
	}
	if _, ok := summary["score"]; !ok {
		t.Fatalf("expected score in summary, got %v", summary)
	}
	if _, ok := summary["rating"]; !ok {
		t.Fatalf("expected rating in summary, got %v", summary)
	}
	if _, ok := summary["scoreBreakdown"]; !ok {
		t.Fatalf("expected scoreBreakdown in summary, got %v", summary)
	}
	findings, ok := result["findings"].([]map[string]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected findings, got %v", result["findings"])
	}
}

func TestAnalyzeOpenAPIAgainstRealSpecs(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	result := analyzeOpenAPI(&admin, &store, "both")
	analysis, ok := result["analysis"].([]map[string]any)
	if !ok || len(analysis) != 2 {
		t.Fatalf("expected 2 analysis entries, got %v", result["analysis"])
	}
	for _, item := range analysis {
		summary, ok := item["summary"].(map[string]any)
		if !ok {
			t.Fatalf("expected summary map, got %v", item["summary"])
		}
		if _, ok := summary["score"]; !ok {
			t.Fatalf("expected score in summary, got %v", summary)
		}
		if _, ok := summary["rating"]; !ok {
			t.Fatalf("expected rating in summary, got %v", summary)
		}
		if _, ok := summary["scoreBreakdown"]; !ok {
			t.Fatalf("expected scoreBreakdown in summary, got %v", summary)
		}
	}
}

func TestAnalyzeSearchCapabilitiesSampleSchema(t *testing.T) {
	entitySchema := map[string]any{
		"entities": map[string]any{
			"product": map[string]any{
				"properties": map[string]any{
					"id":             map[string]any{"type": "uuid"},
					"name":           map[string]any{"type": "string", "searchable": true, "filterable": true, "sortable": true},
					"manufacturerId": map[string]any{"type": "uuid", "filterable": true},
				},
				"guidance": "Use term for broad search and filter for exact field constraints.",
			},
			"category": map[string]any{
				"properties": map[string]any{
					"id":   map[string]any{"type": "uuid"},
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	result := analyzeSearchCapabilities([]string{"product", "category"}, sampleOpenAPI(), entitySchema)
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary, got %v", result["summary"])
	}
	if got := summary["entityCount"]; got != 2 {
		t.Fatalf("expected 2 entities, got %v", got)
	}
	entities, ok := result["entities"].([]map[string]any)
	if !ok || len(entities) != 2 {
		t.Fatalf("expected 2 entity entries, got %v", result["entities"])
	}
	product := entities[1]
	if product["entity"] == "category" {
		product = entities[0]
	}
	if got := product["entity"]; got != "product" {
		t.Fatalf("expected product entry, got %v", got)
	}
	checks, ok := product["checks"].(map[string]any)
	if !ok {
		t.Fatalf("expected checks map, got %v", product["checks"])
	}
	if got := checks["explicitSearchableFieldsMetadata"]; got != true {
		t.Fatalf("expected explicit searchable metadata, got %v", got)
	}
	if got := checks["termVsFilterGuidanceInContract"]; got != true {
		t.Fatalf("expected term/filter guidance, got %v", got)
	}
}

func TestAnalyzeSearchCapabilitiesAgainstBundledData(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	entitySchema := loadJSONForTest(t, "data/entity-schema.json")
	result := analyzeSearchCapabilities(nil, admin, entitySchema)
	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary, got %v", result["summary"])
	}
	if _, ok := summary["averageScore"]; !ok {
		t.Fatalf("expected averageScore, got %v", summary)
	}
	entities, ok := result["entities"].([]map[string]any)
	if !ok || len(entities) < 2 {
		t.Fatalf("expected bundled entity analysis, got %v", result["entities"])
	}
	product := entities[0]
	category := entities[1]
	if fmt.Sprint(product["entity"]) != "category" {
		category = entities[0]
		product = entities[1]
	}
	for _, item := range []map[string]any{product, category} {
		if _, ok := item["score"]; !ok {
			t.Fatalf("expected score, got %v", item)
		}
		if _, ok := item["rating"]; !ok {
			t.Fatalf("expected rating, got %v", item)
		}
		checks, ok := item["checks"].(map[string]any)
		if !ok {
			t.Fatalf("expected checks map, got %v", item["checks"])
		}
		if _, ok := checks["entitySchemaError"]; !ok {
			t.Fatalf("expected entitySchemaError in checks, got %v", checks)
		}
	}
}

func TestAssessWorkflowSupportForAllFlows(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	result := assessWorkflowSupport("", admin, store)
	assessment, ok := result["workflowAssessment"].([]map[string]any)
	if !ok || len(assessment) < 5 {
		t.Fatalf("expected workflow assessments, got %v", result["workflowAssessment"])
	}
}

func TestAssessWorkflowSupportForSpecificUseCase(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	result := assessWorkflowSupport("create a product and complete checkout", admin, store)
	assessment, ok := result["workflowAssessment"].([]map[string]any)
	if !ok || len(assessment) != 1 {
		t.Fatalf("expected one workflow assessment, got %v", result["workflowAssessment"])
	}
	item := assessment[0]
	if got := item["name"]; got != "create_product_and_complete_checkout" {
		t.Fatalf("expected checkout workflow, got %v", got)
	}
	if _, ok := item["score"]; !ok {
		t.Fatalf("expected score, got %v", item)
	}
	if _, ok := item["contractGaps"]; !ok {
		t.Fatalf("expected contractGaps, got %v", item)
	}
}

func TestGenerateFlowChecklist(t *testing.T) {
	checklist := generateFlowChecklist("create a product and complete checkout", "curl", "http://localhost", sampleOpenAPI(), sampleOpenAPI())
	if got := checklist["name"]; got != "create_product_and_complete_checkout" {
		t.Fatalf("expected checkout checklist, got %v", got)
	}
	steps, ok := checklist["checklist"].([]map[string]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("expected checklist steps, got %v", checklist["checklist"])
	}
	first := steps[0]
	if got := first["stepNumber"]; got != 1 {
		t.Fatalf("expected first step number 1, got %v", got)
	}
	if _, ok := first["saveHints"]; !ok {
		t.Fatalf("expected saveHints, got %v", first)
	}
	if _, ok := steps[1]["requestExample"]; !ok {
		t.Fatalf("expected requestExample on a concrete route step, got %v", steps[1])
	}
}

func TestGenerateFlowChecklistAgainstRealOpenAPI(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	checklist := generateFlowChecklist("register customer then place order and inspect order history", "curl", "http://localhost", admin, store)
	if got := checklist["name"]; got != "customer_registration_to_order_history" {
		t.Fatalf("expected customer flow checklist, got %v", got)
	}
	steps, ok := checklist["checklist"].([]map[string]any)
	if !ok || len(steps) < 4 {
		t.Fatalf("expected checklist steps, got %v", checklist["checklist"])
	}
	registerStep := findStepByRoute(t, steps, "/account/register")
	if _, ok := registerStep["routeDetails"]; !ok {
		t.Fatalf("expected routeDetails for register step, got %v", registerStep)
	}
	if _, ok := registerStep["requestExample"]; !ok {
		t.Fatalf("expected requestExample for register step, got %v", registerStep)
	}
}

func TestBuildCriteriaPayloadAcceptsNaturalLanguageIntent(t *testing.T) {
	payload := buildCriteriaPayload(CriteriaInput{
		Entity:       "product",
		Intent:       "search product by term and manufacturer",
		Associations: []string{"manufacturer"},
		Limit:        5,
	})
	if got := payload["term"]; got != "<search-term>" {
		t.Fatalf("expected search_text payload, got %v", payload)
	}
	if _, ok := payload["note"]; ok {
		t.Fatalf("did not expect unknown-intent note, got %v", payload)
	}
}

func TestRequestExamplePrefixesStoreRoutesAndUsesStoreAuth(t *testing.T) {
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	surface := detectRouteSurface("/search", admin, store)
	if got := surface; got != "store" {
		t.Fatalf("expected store surface, got %q", got)
	}

	out := requestExampleForSurface("http://localhost:8000", RequestExampleInput{
		Route:    "/search",
		Method:   "POST",
		Language: "curl",
		Payload: map[string]any{
			"search": "shoe",
		},
	}, surface)

	if !strings.Contains(out, "http://localhost:8000/store-api/search") {
		t.Fatalf("expected store-api prefixed URL, got %s", out)
	}
	if !strings.Contains(out, "sw-access-key: $SHOPWARE_STORE_ACCESS_KEY") {
		t.Fatalf("expected store auth header, got %s", out)
	}
	if strings.Contains(out, "Authorization: Bearer $SHOPWARE_ADMIN_TOKEN") {
		t.Fatalf("did not expect admin auth header, got %s", out)
	}
}

func TestGenerateFlowRequestPack(t *testing.T) {
	pack := generateFlowRequestPack("create a product and complete checkout", "curl", "http://localhost", sampleOpenAPI(), sampleOpenAPI())
	if got := pack["name"]; got != "create_product_and_complete_checkout" {
		t.Fatalf("expected checkout request pack, got %v", got)
	}
	steps, ok := pack["requests"].([]map[string]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("expected request pack steps, got %v", pack["requests"])
	}
	first := steps[0]
	if _, ok := first["request"]; !ok {
		t.Fatalf("expected request object, got %v", first)
	}
	if _, ok := first["knownVariablesAfterStep"]; !ok {
		t.Fatalf("expected knownVariablesAfterStep, got %v", first)
	}
}

func TestGenerateFlowRequestPackAgainstRealOpenAPI(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	pack := generateFlowRequestPack("register customer then place order and inspect order history", "curl", "http://localhost", admin, store)
	if got := pack["name"]; got != "customer_registration_to_order_history" {
		t.Fatalf("expected customer flow request pack, got %v", got)
	}
	steps, ok := pack["requests"].([]map[string]any)
	if !ok || len(steps) < 4 {
		t.Fatalf("expected request pack steps, got %v", pack["requests"])
	}
	registerStep := findStepByRoute(t, steps, "/account/register")
	request, ok := registerStep["request"].(map[string]any)
	if !ok {
		t.Fatalf("expected request object for register step, got %v", registerStep["request"])
	}
	if _, ok := request["example"]; !ok {
		t.Fatalf("expected request example in request object, got %v", request)
	}
}

func TestSelectCuratedFlowUsesBroaderSignals(t *testing.T) {
	flows, err := loadCuratedFlows("data/flows")
	if err != nil {
		t.Fatalf("load curated flows: %v", err)
	}
	flow := selectCuratedFlow("show me the customer order history flow after registration and login", flows)
	if flow == nil {
		t.Fatal("expected a selected flow")
	}
	if got := flow.Name; got != "customer_registration_to_order_history" {
		t.Fatalf("expected customer_registration_to_order_history, got %v", got)
	}
}

func TestExportAssessmentReportJSON(t *testing.T) {
	report := exportAssessmentReport("create a product and complete checkout", "json", "curl", "http://localhost", sampleOpenAPI(), sampleOpenAPI())
	if got := report.(map[string]any)["format"]; got != "json" {
		t.Fatalf("expected json format, got %v", got)
	}
	body, ok := report.(map[string]any)["report"].(map[string]any)
	if !ok {
		t.Fatalf("expected report body, got %v", report)
	}
	if _, ok := body["openapiAnalysis"]; !ok {
		t.Fatalf("expected openapiAnalysis in report, got %v", body)
	}
	if _, ok := body["workflowSupport"]; !ok {
		t.Fatalf("expected workflowSupport in report, got %v", body)
	}
	if _, ok := body["flowChecklist"]; !ok {
		t.Fatalf("expected flowChecklist in report, got %v", body)
	}
	if _, ok := body["flowRequestPack"]; !ok {
		t.Fatalf("expected flowRequestPack in report, got %v", body)
	}
}

func TestExportAssessmentReportMarkdown(t *testing.T) {
	report := exportAssessmentReport("create a product and complete checkout", "markdown", "curl", "http://localhost", sampleOpenAPI(), sampleOpenAPI())
	out := report.(map[string]any)
	if got := out["format"]; got != "markdown" {
		t.Fatalf("expected markdown format, got %v", got)
	}
	content := fmt.Sprint(out["content"])
	if !strings.Contains(content, "# Assessment Report") {
		t.Fatalf("expected markdown header, got %s", content)
	}
	if !strings.Contains(content, "## Workflow Support") {
		t.Fatalf("expected workflow support section, got %s", content)
	}
	if !strings.Contains(content, "## Flow Request Pack") {
		t.Fatalf("expected flow request pack section, got %s", content)
	}
}

func sampleOpenAPI() OpenAPI {
	return OpenAPI{
		Paths: map[string]map[string]OpenAPIOperation{
			"/product": {
				"post": {
					Summary:     "Create a new Product resources.",
					OperationID: "createProduct",
					Tags:        []string{"Product"},
				},
				"get": {
					Summary:     "List with basic information of Product resources.",
					OperationID: "getProductList",
					Tags:        []string{"Product"},
				},
			},
			"/product/{id}": {
				"patch": {
					Summary:     "Partially update information about a Product resource.",
					OperationID: "updateProduct",
					Tags:        []string{"Product"},
				},
			},
			"/product-review/{id}": {
				"get": {
					Summary:     "Detailed information about a Product Review resource.",
					OperationID: "getProductReview",
					Tags:        []string{"Product Review"},
				},
			},
			"/search/product": {
				"post": {
					Summary:     "Search for the Product resources.",
					OperationID: "searchProduct",
					Tags:        []string{"Product"},
				},
			},
			"/checkout/order": {
				"post": {
					Summary:     "Create an order from a cart",
					OperationID: "createOrder",
					Tags:        []string{"Order"},
				},
			},
			"/category": {
				"post": {
					Summary:     "Create a new Category resources.",
					OperationID: "createCategory",
					Tags:        []string{"Category"},
				},
			},
			"/product-manufacturer": {
				"post": {
					Summary:     "Create a new Product Manufacturer resources.",
					OperationID: "createProductManufacturer",
					Tags:        []string{"Product Manufacturer"},
				},
			},
			"/account/login": {
				"post": {
					Summary:     "Log in a customer",
					OperationID: "loginCustomer",
					Tags:        []string{"Account"},
				},
			},
			"/account/register": {
				"post": {
					Summary:     "Register a customer",
					OperationID: "registerCustomer",
					Tags:        []string{"Account"},
				},
			},
			"/account/customer": {
				"get": {
					Summary:     "Get information about current customer",
					OperationID: "readCustomer",
					Tags:        []string{"Profile"},
				},
			},
			"/account/convert-guest": {
				"post": {
					Summary:     "Convert a guest customer",
					OperationID: "convertGuestCustomer",
					Tags:        []string{"Profile"},
				},
			},
			"/handle-payment": {
				"post": {
					Summary:     "Initiate a payment for an order",
					OperationID: "handlePaymentMethod",
					Tags:        []string{"Payment & Shipping"},
				},
			},
			"/order": {
				"get": {
					Summary:     "List orders",
					OperationID: "getOrderList",
					Tags:        []string{"Order"},
				},
				"post": {
					Summary:     "List customer orders",
					OperationID: "readOrderRoute",
					Tags:        []string{"Order"},
				},
			},
			"/order/download/{orderId}/{downloadId}": {
				"get": {
					Summary:     "Download an order asset",
					OperationID: "downloadOrderAsset",
					Tags:        []string{"Order"},
				},
			},
			"/_action/order/{orderId}/state/{transition}": {
				"post": {
					Summary:     "Transition an order to a new state",
					OperationID: "orderStateTransition",
					Tags:        []string{"Order Management"},
				},
			},
			"/_action/order_transaction/{orderTransactionId}/state/{transition}": {
				"post": {
					Summary:     "Transition an order transaction to a new state",
					OperationID: "orderTransactionStateTransition",
					Tags:        []string{"Order Management"},
				},
			},
			"/_action/order_delivery/{orderDeliveryId}/state/{transition}": {
				"post": {
					Summary:     "Transition an order delivery to a new state",
					OperationID: "orderDeliveryStateTransition",
					Tags:        []string{"Order Management"},
				},
			},
			"/_action/order_transaction_capture_refund/{refundId}": {
				"post": {
					Summary:     "Refund an order transaction capture",
					OperationID: "orderTransactionCaptureRefund",
					Tags:        []string{"Order Management"},
				},
			},
			"/search/product-manufacturer": {
				"post": {
					Summary:     "Search for the Product Manufacturer resources.",
					OperationID: "searchProductManufacturer",
					Tags:        []string{"Product Manufacturer"},
				},
			},
			"/search/category": {
				"post": {
					Summary:     "Search for the Category resources.",
					OperationID: "searchCategory",
					Tags:        []string{"Category"},
				},
			},
			"/search": {
				"post": {
					Summary:     "Search for products",
					OperationID: "searchPage",
					Tags:        []string{"Product"},
				},
			},
			"/_info/entity-schema.json": {
				"get": {
					Summary:     "Fetch the entity schema",
					OperationID: "getEntitySchema",
					Tags:        []string{"System"},
				},
			},
		},
	}
}

func TestFindMatchesPrefersCreateProductRoute(t *testing.T) {
	matches := findMatches(sampleOpenAPI(), "create product", "admin")
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if got := matches[0]["path"]; got != "/product" {
		t.Fatalf("expected /product first, got %v", got)
	}
	if got := matches[0]["method"]; got != "POST" {
		t.Fatalf("expected POST first, got %v", got)
	}
	if got := matches[0]["confidence"]; got == nil || got == "" {
		t.Fatalf("expected confidence to be present, got %v", got)
	}
	if got := matches[0]["matchReason"]; got == nil || got == "" {
		t.Fatalf("expected matchReason to be present, got %v", got)
	}
}

func TestFindMatchesFindsCheckoutOrder(t *testing.T) {
	matches := findMatches(sampleOpenAPI(), "checkout order", "store")
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if got := matches[0]["path"]; got != "/checkout/order" {
		t.Fatalf("expected /checkout/order first, got %v", got)
	}
}

func TestDescribeRoutePrefersExactPath(t *testing.T) {
	results := describeRoute(sampleOpenAPI(), "/product")
	if len(results) == 0 {
		t.Fatal("expected describe results")
	}
	if got := results[0]["path"]; got != "/product" {
		t.Fatalf("expected exact /product path first, got %v", got)
	}
	if got := results[0]["confidence"]; got == nil || got == "" {
		t.Fatalf("expected confidence to be present, got %v", got)
	}
}

func TestExplainFlowForProductCheckout(t *testing.T) {
	flow, ok := explainFlow("create a product, put it in a category, add it to cart, and complete checkout", sampleOpenAPI(), sampleOpenAPI()).(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if got := flow["name"]; got != "create_product_and_complete_checkout" {
		t.Fatalf("expected create_product_and_complete_checkout flow, got %v", got)
	}
	steps, ok := flow["steps"].([]map[string]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("expected steps, got %v", flow["steps"])
	}
	if got := steps[0]["route"]; got != "/category" {
		t.Fatalf("expected first step /category, got %v", got)
	}
	if _, ok := steps[0]["routeDetails"]; !ok {
		t.Fatalf("expected routeDetails on first step, got %v", steps[0])
	}
}

func TestExplainFlowForOrderStateAndRefund(t *testing.T) {
	flow, ok := explainFlow("refund an order and update its state", sampleOpenAPI(), sampleOpenAPI()).(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if got := flow["name"]; got != "manage_order_state_and_refund" {
		t.Fatalf("expected manage_order_state_and_refund flow, got %v", got)
	}
	steps, ok := flow["steps"].([]map[string]any)
	if !ok || len(steps) < 3 {
		t.Fatalf("expected refund/state steps, got %v", flow["steps"])
	}
	if got := steps[1]["route"]; got != "/_action/order/{orderId}/state/{transition}" {
		t.Fatalf("expected order state transition step, got %v", got)
	}
	if step := findStepByRoute(t, steps, "/_action/order_delivery/{orderDeliveryId}/state/{transition}"); step == nil {
		t.Fatal("expected delivery state step")
	}
}

func TestExplainFlowForSearchCriteriaInvestigation(t *testing.T) {
	flow, ok := explainFlow("investigate search criteria for product and category including manufacturer lookups", sampleOpenAPI(), sampleOpenAPI()).(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if got := flow["name"]; got != "investigate_search_criteria_for_product_and_category" {
		t.Fatalf("expected investigate_search_criteria_for_product_and_category flow, got %v", got)
	}
	steps, ok := flow["steps"].([]map[string]any)
	if !ok || len(steps) < 4 {
		t.Fatalf("expected search investigation steps, got %v", flow["steps"])
	}
	if got := steps[0]["route"]; got != "/search/product-manufacturer" {
		t.Fatalf("expected manufacturer discovery first, got %v", got)
	}
	if _, ok := steps[0]["routeDetails"]; !ok {
		t.Fatalf("expected routeDetails for manufacturer discovery step, got %v", steps[0])
	}
	examples, ok := flow["recommendedExamples"].([]map[string]any)
	if !ok || len(examples) < 3 {
		t.Fatalf("expected recommendedExamples, got %v", flow["recommendedExamples"])
	}
}

func TestExplainFlowForCatalogDiscoverability(t *testing.T) {
	flow, ok := explainFlow("catalog setup for real discoverability so a product is visible in the sales channel", sampleOpenAPI(), sampleOpenAPI()).(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if got := flow["name"]; got != "catalog_setup_for_real_discoverability" {
		t.Fatalf("expected catalog_setup_for_real_discoverability flow, got %v", got)
	}
	steps, ok := flow["steps"].([]map[string]any)
	if !ok || len(steps) < 6 {
		t.Fatalf("expected discoverability steps, got %v", flow["steps"])
	}
	if got := steps[0]["route"]; got != "/product-manufacturer" {
		t.Fatalf("expected manufacturer step first, got %v", got)
	}
	if step := findStepByRoute(t, steps, "/search"); step == nil {
		t.Fatal("expected store search verification step")
	}
	if checks, ok := flow["diagnosticChecks"].([]string); !ok || len(checks) < 3 {
		t.Fatalf("expected diagnosticChecks, got %v", flow["diagnosticChecks"])
	}
}

func TestExplainFlowForCustomerRegistrationToOrderHistory(t *testing.T) {
	flow, ok := explainFlow("register customer then place order and inspect order history", sampleOpenAPI(), sampleOpenAPI()).(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if got := flow["name"]; got != "customer_registration_to_order_history" {
		t.Fatalf("expected customer_registration_to_order_history flow, got %v", got)
	}
	steps, ok := flow["steps"].([]map[string]any)
	if !ok || len(steps) < 6 {
		t.Fatalf("expected customer/order-history steps, got %v", flow["steps"])
	}
	if got := steps[0]["route"]; got != "/account/register" {
		t.Fatalf("expected registration first, got %v", got)
	}
	if step := findStepByRoute(t, steps, "/order"); step == nil {
		t.Fatal("expected order history step")
	}
}

func TestFindMatchesAgainstRealAdminOpenAPI(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")

	t.Run("create product", func(t *testing.T) {
		matches := findMatches(admin, "create product", "admin")
		if len(matches) == 0 {
			t.Fatal("expected matches")
		}
		if got := matches[0]["path"]; got != "/product" {
			t.Fatalf("expected /product first, got %v", got)
		}
		if got := matches[0]["method"]; got != "POST" {
			t.Fatalf("expected POST first, got %v", got)
		}
	})

	t.Run("search product manufacturer", func(t *testing.T) {
		matches := findMatches(admin, "search product manufacturer", "admin")
		if len(matches) == 0 {
			t.Fatal("expected matches")
		}
		if got := matches[0]["path"]; got != "/search/product-manufacturer" {
			t.Fatalf("expected /search/product-manufacturer first, got %v", got)
		}
	})
}

func TestFindMatchesAgainstRealStoreOpenAPI(t *testing.T) {
	store := loadOpenAPIForTest(t, "data/store-openapi.json")
	matches := findMatches(store, "checkout order", "store")
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if got := matches[0]["path"]; got != "/checkout/order" {
		t.Fatalf("expected /checkout/order first, got %v", got)
	}
	if got := matches[0]["confidence"]; got == nil || got == "" {
		t.Fatalf("expected confidence, got %v", got)
	}
}

func TestFindMatchesHandlesSearchCriteriaPhrasing(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	matches := findMatches(admin, "investigate search criteria for product and category", "admin")
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}

	paths := map[string]bool{}
	for _, match := range matches {
		paths[fmt.Sprint(match["path"])] = true
	}
	if !paths["/search/product"] {
		t.Fatalf("expected /search/product in matches, got %v", matches)
	}
	if !paths["/search/category"] {
		t.Fatalf("expected /search/category in matches, got %v", matches)
	}
}

func TestDescribeRouteAgainstRealAdminOpenAPI(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	results := describeRoute(admin, "/product")
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if got := results[0]["path"]; got != "/product" {
		t.Fatalf("expected /product first, got %v", got)
	}
}

func TestExplainFlowAgainstRealOpenAPI(t *testing.T) {
	admin := loadOpenAPIForTest(t, "data/admin-openapi.json")
	store := loadOpenAPIForTest(t, "data/store-openapi.json")

	t.Run("product checkout flow", func(t *testing.T) {
		flow, ok := explainFlow("create a product, put it in a category, add it to cart, and complete checkout", admin, store).(map[string]any)
		if !ok {
			t.Fatal("expected map result")
		}
		steps, ok := flow["steps"].([]map[string]any)
		if !ok || len(steps) < 6 {
			t.Fatalf("expected enriched steps, got %v", flow["steps"])
		}
		firstDetails, ok := steps[0]["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on first step, got %v", steps[0])
		}
		if got := firstDetails["path"]; got != "/category" {
			t.Fatalf("expected first routeDetails path /category, got %v", got)
		}
		orderStep := findStepByRoute(t, steps, "/checkout/order")
		orderDetails, ok := orderStep["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on checkout/order step, got %v", orderStep)
		}
		if got := orderDetails["path"]; got != "/checkout/order" {
			t.Fatalf("expected /checkout/order routeDetails, got %v", got)
		}
	})

	t.Run("search criteria investigation flow", func(t *testing.T) {
		flow, ok := explainFlow("investigate search criteria for product and category including manufacturer lookups", admin, store).(map[string]any)
		if !ok {
			t.Fatal("expected map result")
		}
		steps, ok := flow["steps"].([]map[string]any)
		if !ok || len(steps) < 4 {
			t.Fatalf("expected enriched search steps, got %v", flow["steps"])
		}
		details, ok := steps[0]["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on manufacturer step, got %v", steps[0])
		}
		if got := details["path"]; got != "/search/product-manufacturer" {
			t.Fatalf("expected /search/product-manufacturer routeDetails, got %v", got)
		}
	})

	t.Run("customer registration to order history flow", func(t *testing.T) {
		flow, ok := explainFlow("register customer then place order and inspect order history", admin, store).(map[string]any)
		if !ok {
			t.Fatal("expected map result")
		}
		steps, ok := flow["steps"].([]map[string]any)
		if !ok || len(steps) < 6 {
			t.Fatalf("expected enriched customer steps, got %v", flow["steps"])
		}
		registerDetails, ok := steps[0]["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on register step, got %v", steps[0])
		}
		if got := registerDetails["path"]; got != "/account/register" {
			t.Fatalf("expected /account/register routeDetails, got %v", got)
		}
		orderHistoryStep := findStepByRoute(t, steps, "/order")
		orderHistoryDetails, ok := orderHistoryStep["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on order history step, got %v", orderHistoryStep)
		}
		if got := orderHistoryDetails["path"]; got != "/order" {
			t.Fatalf("expected /order routeDetails, got %v", got)
		}
	})

	t.Run("catalog discoverability flow", func(t *testing.T) {
		flow, ok := explainFlow("catalog setup for real discoverability so a product is visible in the sales channel", admin, store).(map[string]any)
		if !ok {
			t.Fatal("expected map result")
		}
		steps, ok := flow["steps"].([]map[string]any)
		if !ok || len(steps) < 6 {
			t.Fatalf("expected enriched discoverability steps, got %v", flow["steps"])
		}
		firstDetails, ok := steps[0]["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on manufacturer step, got %v", steps[0])
		}
		if got := firstDetails["path"]; got != "/product-manufacturer" {
			t.Fatalf("expected /product-manufacturer routeDetails, got %v", got)
		}
		storeSearchStep := findStepByRoute(t, steps, "/search")
		storeDetails, ok := storeSearchStep["routeDetails"].(map[string]any)
		if !ok {
			t.Fatalf("expected routeDetails on store search step, got %v", storeSearchStep)
		}
		if got := storeDetails["path"]; got != "/search" {
			t.Fatalf("expected /search routeDetails, got %v", got)
		}
	})
}
