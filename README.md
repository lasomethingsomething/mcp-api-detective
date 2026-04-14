# shopware-dev-mcp

An MCP server for exploring Shopware Admin API and Store API contracts without having to manually piece together routes, auth requirements, and multi-step workflows.

It is built to help developers who get stuck on questions like:

- Which route do I actually need?
- Is this Admin API or Store API?
- What request shape should I start with?
- Why does this flow fail even though one step worked?
- How do I chain these calls together without guessing?

## What This Server Does

This server combines three things:

1. Shopware API contract discovery
2. request/example generation
3. curated workflow guidance and diagnostics for common multi-step developer tasks

It can answer both low-level and high-level questions:

- low-level:
  - find a route
  - describe a route
  - analyze the OpenAPI spec
  - analyze Product/Category search-capability gaps
  - assess workflow support against the spec
  - generate a flow-aware checklist and request pack
  - export an assessment report for review or docs work
  - explain auth
  - generate a request example
- high-level:
  - explain how to create a product and complete checkout
  - explain how to make a product discoverable in Store API
  - explain how to investigate search criteria for Product and Category
  - explain how customer registration, login, and order history fit together
  - explain how to manage order state transitions and refunds

## What It Contains

### Source code

- [main.go](/Users/l.apple/shopware-dev-mcp/main.go)
  The MCP server implementation and tool behavior
- [main_test.go](/Users/l.apple/shopware-dev-mcp/main_test.go)
  Regression tests for route ranking, flow selection, and real OpenAPI-backed behavior

### Bundled contract data

- [data/admin-openapi.json](/Users/l.apple/shopware-dev-mcp/data/admin-openapi.json)
- [data/store-openapi.json](/Users/l.apple/shopware-dev-mcp/data/store-openapi.json)
- [data/admin-routes.json](/Users/l.apple/shopware-dev-mcp/data/admin-routes.json)
- [data/store-routes.json](/Users/l.apple/shopware-dev-mcp/data/store-routes.json)
- [data/entity-schema.json](/Users/l.apple/shopware-dev-mcp/data/entity-schema.json)

These files let the server work even without live Shopware credentials, and they are also used in tests.

### Curated developer flows

- [data/flows/README.md](/Users/l.apple/shopware-dev-mcp/data/flows/README.md)
- [data/flows/create_product_and_complete_checkout.json](/Users/l.apple/shopware-dev-mcp/data/flows/create_product_and_complete_checkout.json)
- [data/flows/catalog_setup_for_real_discoverability.json](/Users/l.apple/shopware-dev-mcp/data/flows/catalog_setup_for_real_discoverability.json)
- [data/flows/investigate_search_criteria_for_product_and_category.json](/Users/l.apple/shopware-dev-mcp/data/flows/investigate_search_criteria_for_product_and_category.json)
- [data/flows/customer_registration_to_order_history.json](/Users/l.apple/shopware-dev-mcp/data/flows/customer_registration_to_order_history.json)
- [data/flows/manage_order_state_and_refund.json](/Users/l.apple/shopware-dev-mcp/data/flows/manage_order_state_and_refund.json)

These flow files are human-readable artifacts you can inspect directly, and they are also loaded by the `explain_flow` MCP tool at runtime.
They also power flow scoring, checklist generation, request-pack generation, and workflow-level assessment output.

## MCP Tools

The server exposes tools for route discovery, contract inspection, examples, and workflow guidance.

### Discovery and inspection

- `list_admin_routes`
  List Shopware Admin API routes
- `list_store_routes`
  List Shopware Store API routes
- `get_admin_openapi`
  Return the Admin API OpenAPI document
- `get_store_openapi`
  Return the Store API OpenAPI document
- `get_entity_schema`
  Return Shopware entity schema metadata
- `find_routes`
  Search routes using ranking, confidence, and match reasons
- `describe_route`
  Describe a route with method, parameters, request body, responses, confidence, and match reason
- `analyze_openapi`
  Run structural diagnostics and workflow-support heuristics against Shopware OpenAPI data, including an overall score, rating, and score breakdown
- `analyze_search_capabilities`
  Assess how discoverable Product and Category search capabilities are in the contract, including searchable/filterable/sortable metadata gaps and term-vs-filter clarity
- `assess_workflow_support`
  Score curated workflows against the current contract and report route coverage, missing routes, weak documentation, hidden-prerequisite steps, and flow-to-contract gaps
- `generate_flow_checklist`
  Turn a curated workflow into a step-by-step manual execution checklist with request examples, variable handoff hints, and route details
- `generate_flow_request_pack`
  Generate a flow-aware request pack with per-step starter payloads, example requests, and evolving variable handoff context
- `export_assessment_report`
  Combine OpenAPI analysis, workflow support assessment, and checklist output into a stable JSON or Markdown report

### Helpers

- `generate_criteria_payload`
  Generate starter Criteria payloads for Admin API search tasks
- `generate_api_request_example`
  Generate `curl` or JavaScript examples for a route
- `explain_surface`
  Explain whether a use case belongs in Admin API, Store API, or extension work
- `explain_auth`
  Explain likely authentication requirements

### Curated workflow tool

- `explain_flow`
  Select a curated workflow from `data/flows/*.json`, enrich it with route metadata from OpenAPI, and return a structured sequence of steps

## How It Works

### Route discovery

`find_routes` and `describe_route` do more than plain substring matching.

They now use:

- normalized route matching
- token matching
- verb-to-method intent
- entity-aware ranking
- path-like query boosts
- confidence values
- match reasons

That means queries like:

- `create product`
- `checkout order`
- `search product manufacturer`

should produce more useful top results than raw text matching alone.

### Workflow guidance

The flow-driven tools like `explain_flow`, `generate_flow_checklist`, and `generate_flow_request_pack` do this:

1. load all curated flow files from `data/flows/`
2. score them against the incoming use case with weighted trigger, token, name, summary, and step relevance
3. choose the best matching flow
4. enrich each step with route details from Admin or Store OpenAPI
5. return the combined structured result

So the server is not generating workflows from scratch every time. It is selecting curated guidance and grounding it in the current Shopware contract data.

## Auth Model

### Admin API

- uses `Authorization: Bearer <token>`

### Store API

- uses `sw-access-key`

The server can work from bundled fallback files, or from live Shopware if credentials are configured.

## Environment Variables

You can point the server at a live Shopware instance with:

- `SHOPWARE_BASE_URL`
- `SHOPWARE_ADMIN_TOKEN`
- `SHOPWARE_STORE_ACCESS_KEY`
- `SHOPWARE_PREFER_LIVE_DATA`
- `SHOPWARE_ADMIN_OPENAPI_FILE`
- `SHOPWARE_STORE_OPENAPI_FILE`
- `SHOPWARE_ENTITY_SCHEMA_FILE`
- `SHOPWARE_ADMIN_ROUTES_FILE`
- `SHOPWARE_STORE_ROUTES_FILE`

If live access is not available, the bundled files under `data/` can still be used.

## How To Use

This is the simplest practical sequence.

### 1. Start with the use case

Describe what you are trying to do in plain language.

Examples:

- `create a product and complete checkout`
- `make sure a product is visible in the sales channel`
- `investigate search criteria for product and category`
- `refund an order and update its state`

### 2. Ask `explain_surface`

Use this when you are unsure whether the task belongs in:

- Admin API
- Store API
- plugin/app/extension work

### 3. Ask `explain_flow`

If the task is a known workflow, this should be your main entry point.

It gives:

- summary
- ordered steps
- prerequisites
- common failure reasons
- diagnostics
- enriched route details where available

If you want the same workflow in a more execution-oriented shape, follow it with:

- `generate_flow_checklist`
- `generate_flow_request_pack`

### 4. Drill down with `find_routes`

If you need to inspect the exact available routes for a step, use `find_routes`.

Examples:

- `create product`
- `search product manufacturer`
- `checkout order`
- `order transaction state`

### 5. Inspect exact routes with `describe_route`

Use this to confirm:

- method
- route
- request body
- parameters
- response shape

### 6. Generate a starter request

Use:

- `generate_api_request_example`
- `generate_criteria_payload`
- `generate_flow_request_pack`

This gives you a concrete first request instead of a blank page.

### 7. Debug failures

If a chain still fails:

- check `explain_auth`
- check `explain_flow` failure reasons
- verify the route with `describe_route`
- compare Admin-side existence with Store-side visibility where relevant
- run `analyze_openapi` when you suspect the contract itself is incomplete, weakly described, or missing workflow-critical metadata
- run `analyze_search_capabilities` when you want a focused read on Product/Category search discoverability, field-capability metadata gaps, and term-vs-filter ambiguity
- run `assess_workflow_support` when you want to know how well the current contract supports the workflows you care about
- run `generate_flow_checklist` when you want an actionable manual sequence for executing a workflow step by step
- run `generate_flow_request_pack` when you want the actual starter requests for each step in a curated flow
- run `export_assessment_report` when you want a shareable artifact for docs, product review, or contract assessment

## Recommended Usage Patterns

### If you are lost in a workflow

Use:

1. `explain_flow`
2. `describe_route`
3. `generate_flow_request_pack`
4. `generate_api_request_example`

### If you only know the entity or action

Use:

1. `find_routes`
2. `describe_route`

### If you are debugging Criteria

Use:

1. `explain_flow`
2. `generate_criteria_payload`
3. `get_entity_schema`
4. `analyze_search_capabilities`
5. `describe_route`

## Manual Testing

Because the curated workflows now live in files, you can test them in two ways:

### Through MCP

Call the flow-oriented tools with natural-language use cases:

- `explain_flow`
- `generate_flow_checklist`
- `generate_flow_request_pack`
- `assess_workflow_support`

### Directly from the repo

Open the JSON files in [data/flows](/Users/l.apple/shopware-dev-mcp/data/flows) and follow the described steps manually.

This is useful when you want to:

- review workflow content
- edit triggers
- refine diagnostics
- compare the written flow to actual Shopware behavior

## Testing

Run:

```bash
GOCACHE=/tmp/shopware-dev-mcp-gocache go test ./...
```

The tests cover:

- route ranking behavior
- exact route description behavior
- curated flow loading
- flow enrichment against bundled OpenAPI data
- flow request-pack generation
- real-schema regression cases for Shopware Admin and Store APIs

## Current Limitations

- Curated flows are intentionally selective, not exhaustive
- Flow matching is now weighted and more resilient than plain trigger matching, but it is still heuristic and may need tuning as more workflows are added
- The server can now diagnose missing search-capability metadata, but it still cannot invent contract-level searchable/filterable/sortable fields that the Shopware spec does not expose
- Some workflow steps are still guidance-first because Shopware prerequisites are not always explicit in OpenAPI alone

## Best Fit

This server is best for:

- developers integrating with Shopware APIs
- people learning the difference between Admin API and Store API
- debugging chained API workflows
- reducing trial-and-error for common Shopware tasks

It is especially useful when raw route docs exist, but the sequence and hidden prerequisites are what actually block progress.
