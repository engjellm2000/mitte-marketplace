# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

MITTE serves online shoppers across a broad German marketplace, alongside staff operating the catalog, orders, payments, stock, and fulfillment. The primary inventory operator and their exact team structure are inferred from the existing staff-only catalog and warehouse tools.

## Product Purpose

MITTE is a German-language multi-category marketplace for technology, home and furniture, beauty, fashion, sport, kitchen, garden, family, office, pets, food, mobility, tools, and travel. It connects search-led product discovery and checkout to order, payment, inventory, and fulfillment operations.

## Current implementation snapshot

The repository currently ships as a single commerce platform with multiple processes rather than an assembled microservice mesh. `cmd/server` owns the web app and gRPC listener, while background workers handle order relays, email delivery, payment expiry, and Stripe reconciliation. Product, order, stock, invoice, and payment logic live in the PostgreSQL-backed `internal/store` and related packages, and the React frontend in `frontend` powers the storefront and staff operations together with server-rendered Go templates.

## Positioning

The application is built as an end-to-end commerce system, not only a storefront. This is inferred from the existing connected catalog, checkout, OMS, payment, and warehouse workflows.

## Operating Context

Staff use authenticated browser tools to maintain catalog records, reconcile stock, process orders, and complete fulfillment. The current inventory model is one shared stock pool; this assumption is retained for this inventory extension.

## Capabilities and Constraints

- The backend is Go with PostgreSQL and explicit, append-only schema migrations.
- The frontend is a React storefront and staff workspaces.
- The shopping assistant always searches the active public catalog; with a working Gemini API key it can also answer in natural language via server-side tool calls. Chat history is not persisted by MITTE.
- Checkout reserves stock transactionally and prevents overselling.
- Staff catalog writes and warehouse operations are protected by staff authorization and CSRF checks.
- OMS return requests and replacement-with-return actions create linked warehouse return notices; warehouse receipt and inspection are recorded, and only items approved for resale increase available inventory.
- Refunds and replacement shipments remain manual OMS decisions; carrier return-label generation is not configured.
- Payments use Stripe test-mode configuration and local email can be routed to Mailpit. Existing demo-product and production-integration constraints remain in force.
- A single stock location is assumed from the existing product-level inventory field; multiple warehouses and transfers are undecided.

## Evidence on Hand

The repository contains the MITTE storefront, OMS, catalog administration, warehouse fulfillment, payment tools, Go API, PostgreSQL migrations, and operational documentation. Optional local catalog fixtures are synthetic and are not launch-ready commercial evidence.

## Product Principles

- Keep storefront availability consistent with staff stock operations.
- Make stock-changing operations traceable to an actor and reason, including inventory accepted from inspected returns.
- Prevent overselling and accidental negative inventory.
- Preserve order and product snapshots as operational records.
- Keep demo data and unconfigured external integrations explicit.
- Record account-linked product views and use purchase history for recommendations only after explicit customer consent.
- Rank bestsellers only from confirmed paid or fulfilled product sales.
- When Gemini mode is active, disclose that shopping-assistant messages are sent to Google Gemini; do not transmit account, order, or payment data to the model.
