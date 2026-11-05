---
version: 1
slug: "frontend-src-pages-oms-marketingadmin-jsx"
primary_target: "frontend/src/pages/oms/MarketingAdmin.jsx"
related_targets:
  - "frontend/src/marketing.css"
  - "internal/web/marketing.go"
---

# MITTE Campaign Operations

- **Scope and mode:** Staff-only promotion and campaign editor; Operate.
- **Audience and job:** Authorized catalog staff creating, scheduling, targeting, and reviewing coupon and automatic campaign records.
- **Action and proof:** Inspect campaign status and code; edit discount type and amount, eligibility, schedule, redemption limits, public copy, destination targeting, automatic application, and demo labeling; save through the authenticated campaign API.
- **Constraints:** Preserve staff authorization and CSRF protection. Show API failures distinctly from empty data and saving state. Keep the demo-data warning visible, and do not imply that campaign content proves real prices, inventory, customer activity, or commercial outcomes. Automatic campaigns do not stack with customer-entered codes.
- **Direction and memorable moment:** Inherit MITTE's compact operations language: a scannable campaign/status list beside a focused editor, with schedule and eligibility controls grouped separately from public-facing campaign content. Keep status and save feedback visible without interrupting the editing task.
- **Unresolved:** Confirm the intended staff role and timezone interpretation for campaign scheduling before production use.
