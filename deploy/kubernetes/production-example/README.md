# Production Kubernetes overlay

The `base` manifests are intentionally non-deployable: they contain image,
seller, hostname, and trusted-proxy placeholders. Create a protected overlay
outside this repository or in your GitOps configuration and replace every
`REPLACE_WITH_*` value before rendering the release.

The overlay must supply all of the following through a secret-manager-backed
`mitte-secrets` resource:

- `DATABASE_URL`, with TLS verification enabled
- `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET`
- `METRICS_BEARER_TOKEN`
- `STAFF_CREDENTIALS` and `STAFF_ROLES`; staff credentials must use
  `username:$argon2id$...` entries, never plaintext passwords
- `OMS_API_KEY`, `OMS_USERNAME`, and `OMS_PASSWORD`
- `SMTP_USERNAME` and `SMTP_PASSWORD` when SMTP authentication is used
- `INVOICE_SELLER_IBAN` when invoices need payment instructions

Set `TRUSTED_PROXY_CIDRS` to only the address range of the ingress proxy that
connects directly to the pods. Do not use `0.0.0.0/0`; the value controls
whether forwarded client-IP headers are trusted for security rate limits.

Before promotion, render the overlay with `kubectl kustomize`, validate it
with server-side dry-run against the target cluster, and confirm that no
placeholder values remain. Run the migration job exactly once for the release
before rolling out the workloads.
