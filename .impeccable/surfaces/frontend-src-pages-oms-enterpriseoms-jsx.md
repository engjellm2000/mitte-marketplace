---
version: 1
slug: "frontend-src-pages-oms-enterpriseoms-jsx"
primary_target: "frontend/src/pages/oms/EnterpriseOMS.jsx"
related_targets:
  - "frontend/src/pages/oms/OMSDashboard.jsx"
  - "frontend/src/pages/oms/OMSBacklog.jsx"
  - "frontend/src/pages/oms/OMSWorkspace.jsx"
  - "frontend/src/pages/oms/OMSNavigation.jsx"
  - "frontend/src/pages/oms/OMSProcesses.jsx"
  - "frontend/src/pages/oms/OMSCustomers.jsx"
  - "frontend/src/pages/oms/OMSCategoryTree.jsx"
  - "frontend/src/pages/oms/OMSPriceWorkspace.jsx"
  - "frontend/src/pages/oms/OMSReorderQueue.jsx"
  - "frontend/src/pages/oms/OMSProcesses.jsx"
  - "frontend/src/pages/oms/CatalogAdmin.jsx"
  - "frontend/src/lib/oms-csv.js"
  - "frontend/src/oms-redesign.css"
---

# MITTE Order Management

- **Scope and mode:** Staff-only order management workspace; Operate.
- **Audience and job:** Staff search and process order records, inspect item snapshots and registered customer accounts, compare payment and fulfillment state, manage the open-order backlog, and maintain catalog products, selling prices, sellable inventory, and low-stock review.
- **Action and proof:** Search orders, order-line snapshots, registered customers, return/replacement requests and their linked warehouse return notices, and staff-entered shipment delay records; open order details and account-linked order records; distinguish invoice-approved orders from paid orders; use existing item-level actions; inspect a searchable, paginated backlog of payment-waiting orders, paid orders without fulfillment records, and active fulfillment; inspect paginated server-filtered payment, fulfillment, clarify, and historical matrices; apply validated CSV delivery-date updates with per-row outcomes and safe retry of only unsaved rows; review paginated product prices and recorded catalog audit history; inspect active in-stock products in a sellable queue; review low- and out-of-stock products in a permission-gated reorder queue; inspect order email delivery state and automatic retry metadata; and open the catalog and inventory workspaces for existing maintenance actions.
- **Constraints:** Keep staff authorization, CSRF-protected mutations, and existing payment/warehouse behavior. Do not invent historical data, operational states, totals, delivery promises, SLAs, or integrations. The backlog includes only recorded payment-waiting orders, paid orders without an active fulfillment record, and active fulfillment steps; it sorts by order creation time without labeling orders overdue. Delay records reflect staff-entered dates, not carrier telemetry or delivery confirmation. OMS returns and replacement-with-return actions create warehouse notices; staff record arrival and inspection, and only a resale-approved inspection posts to stock. Refund and replacement shipment decisions remain manual; carrier return-label generation is not configured. The email queue exposes delivery metadata only; the relay owns retries and message bodies are not shown. Surface load and authorization failures distinctly.
- **Direction and memorable moment:** MITTE's ink-and-navy header groups work into task-based modules and a contextual subnav; the order desk opens on direct lookup, followed by a compact operational pulse and recent orders. A dedicated backlog queue makes payment, missing fulfillment, and active warehouse work scannable in one oldest-first list, then sends staff into the existing order and warehouse actions. Dense searchable tables and focused order case views keep customer, payment, fulfillment, and item context close to each action. The attached legacy screens inform navigation grouping and data density, not the visual language.
- **Unresolved:** Legacy matrix semantics and source data must be confirmed before they can be represented as more than current order states.
