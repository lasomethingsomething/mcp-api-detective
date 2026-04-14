# Curated Flows

This folder stores the curated workflow definitions used by the Shopware MCP server's flow-oriented tools.

## What This Does

Each JSON file in this directory describes a developer workflow that is easy to get lost in when chaining Shopware API calls together.

Examples:

- creating a product and completing checkout
- making a product discoverable in the Store API
- investigating Product/Category search criteria
- managing order state transitions and refunds

The MCP server loads these files at runtime, scores them against the use case with weighted matching, and then enriches each step with live route details from the bundled Shopware OpenAPI data.

That means the flow files are:

- human-readable content you can review directly
- the source material for `explain_flow`
- the source material for `generate_flow_checklist`
- the source material for `generate_flow_request_pack`
- part of the evidence used by `assess_workflow_support`
- a manual testing aid for verifying workflows outside the MCP client

## How The Flow Tools Use These Files

At runtime, the server:

1. loads all `*.json` files in `data/flows/`
2. scores them against the incoming `useCase`
3. picks the best matching flow
4. enriches each step with route metadata from Admin or Store OpenAPI
5. returns the final structured result in different shapes depending on the tool

If no curated flow matches well enough, the server falls back to generic guidance.

## File Format

Each flow file can contain:

- `name`
- `confidence`
- `surface`
- `summary`
- `triggers`
- `steps`
- `prerequisites`
- `commonFailureReasons`
- `diagnosticChecks`
- `recommendedExamples`

Each step supports:

- `surface`
- `method`
- `route`
- `purpose`
- `notes`
- `optional`

## How To Use It

### 1. Read a flow manually

Open any file in this folder and follow the steps directly.

Good starting files:

- [create_product_and_complete_checkout.json](/Users/l.apple/shopware-dev-mcp/data/flows/create_product_and_complete_checkout.json)
- [catalog_setup_for_real_discoverability.json](/Users/l.apple/shopware-dev-mcp/data/flows/catalog_setup_for_real_discoverability.json)
- [investigate_search_criteria_for_product_and_category.json](/Users/l.apple/shopware-dev-mcp/data/flows/investigate_search_criteria_for_product_and_category.json)
- [manage_order_state_and_refund.json](/Users/l.apple/shopware-dev-mcp/data/flows/manage_order_state_and_refund.json)

### 2. Use it through the MCP server

Ask the MCP server with a natural-language use case.

Useful tools:

- `explain_flow`
- `generate_flow_checklist`
- `generate_flow_request_pack`
- `assess_workflow_support`

Example prompts:

- `create a product and complete checkout`
- `make sure a product is visible in the sales channel`
- `investigate search criteria for product and category`
- `refund an order and update its state`

### 3. Extend a flow

To add or improve a workflow:

1. create or edit a JSON file in this folder
2. add strong `triggers` that match likely user phrasing
3. define ordered `steps`
4. include hidden prerequisites and common failure reasons
5. run the test suite

## Writing Guidance

These files should optimize for developers who get lost between calls, not just endpoint discovery.

Prefer:

- step-by-step sequences
- explicit prerequisites
- common failure interpretation
- practical diagnostics

Avoid:

- vague prose without routes
- endpoint dumps without ordering
- assuming developers already know hidden Shopware requirements

## Testing

Run:

```bash
GOCACHE=/tmp/shopware-dev-mcp-gocache go test ./...
```

This validates:

- flow loading
- flow selection
- route ranking
- exact route description behavior
- real OpenAPI-backed flow enrichment

## Current Limitation

These curated flows are content-driven guidance. They improve workflow discoverability, but they do not yet replace true contract-level capability metadata such as searchable/filterable/sortable field declarations.
