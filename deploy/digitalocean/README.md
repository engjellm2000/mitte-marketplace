# DigitalOcean production rollout

This runbook records the selected deployment shape; it does not provision infrastructure. Choose the region, Droplet sizes, domain, private addresses, Kafka endpoint, and credentials before rollout. Do not substitute example values for live network or secret configuration.

## Target topology

```text
Internet -> DNS -> TLS reverse proxy on app Droplet -> 127.0.0.1:8080
                                      |
                              Docker Compose services
                                /             \
                   private VPC                 private VPC
               Managed PostgreSQL       separate secured Kafka
```

The current Compose deployment is a single app Droplet and is not highly available. Kafka consumers are required at launch, but Kafka is operated separately from the app host. Decide whether that secured Kafka service is hosted on DigitalOcean or reached through another provider before configuring its network path.

## Staging first

Use a separate DigitalOcean project or clearly isolated staging resources: a staging Droplet, managed PostgreSQL cluster/database, Kafka topic and credentials, DNS name, Stripe test-mode account configuration, and email sandbox. Never point staging at production databases, topics, mailboxes, secrets, or Stripe live keys. Use synthetic catalog and customer data. Keep the staging environment alive long enough to exercise recovery and normal operating procedures before promoting a release.

Build and scan a release candidate in CI, then deploy that same immutable image digest to staging and later production. On the staging host, keep the environment file outside the checkout, owned by root and readable only by root (for example, mode `0600`). Populate it through the chosen secret manager; do not paste secret values into shell commands, commit them, or print the rendered Compose configuration. Docker administrators can still inspect container environment variables, so limit host access accordingly.

Run the existing production-hardened Compose stack under its own project name. Set `MITTE_IMAGE` to the immutable staging image digest, `PUBLIC_BASE_URL` and `TRUSTED_HOSTS` to the staging hostname, and supply staging-only values for every required variable in `compose.production.yaml`. In particular, Stripe must use a test-mode secret key (`sk_test_...`) and its matching webhook signing secret; SMTP must use a test inbox/sandbox; Kafka must use a staging-only topic and identity with TLS/SASL. Use clearly marked synthetic invoice/seller details approved for test documents. Do not enable the demo payment rails or invoice sandbox in this production-mode stack.

With the repository checkout and protected environment file available on the staging host, validate and deploy with:

```sh
docker compose --project-name mitte-staging --env-file /etc/mitte/staging.env -f compose.production.yaml config --quiet
docker compose --project-name mitte-staging --env-file /etc/mitte/staging.env -f compose.production.yaml pull
docker compose --project-name mitte-staging --env-file /etc/mitte/staging.env -f compose.production.yaml run --rm --no-deps -e RUN_MIGRATIONS=true app /mitte-migrate
docker compose --project-name mitte-staging --env-file /etc/mitte/staging.env -f compose.production.yaml up -d
docker compose --project-name mitte-staging --env-file /etc/mitte/staging.env -f compose.production.yaml ps
```

Do not use `docker compose config` without `--quiet` on a terminal that records output, because interpolation can reveal secrets. Confirm the host firewall exposes only SSH from approved administrator addresses and TCP 80/443; the app itself remains bound to loopback behind the TLS proxy. Set `TRUSTED_PROXY_CIDRS` to the observed, narrowly scoped proxy peer range.

Before promoting the digest, complete and record these staging checks:

1. Verify HTTPS, certificate renewal, `/healthz`, `/readyz`, and that `/metrics`, OMS, gRPC, and worker metrics are not public.
2. Create a customer with a controlled test mailbox; exercise verification, resend, password reset, and expected delivery failures through the email sandbox.
3. Place a synthetic order using Stripe test mode. Verify checkout success and cancellation, valid and invalid webhook signatures, duplicate delivery, expired checkout, inventory release, and the final order/payment state. Do not use real card details or live keys in staging.
4. Confirm the order relay delivers the expected versioned event to the staging Kafka topic, and that duplicate delivery is safely deduplicated by the consumer. Inspect worker health, error metrics, and logs.
5. Restore a staging database backup to a separate isolated database and verify the restored application can become ready. Record recovery time and data-loss window; a configured backup alone is not a completed restore test.
6. Run a bounded load test representative of expected launch traffic. Check latency/error rates, worker backlog, CPU/memory, and total database connections including migration and operator headroom.
7. Exercise the release/rollback procedure with the previous image digest and verify migrations remain compatible with rollback. Record unresolved issues and assign owners before production approval.

Production promotion is a separate change: confirm all legal and operational gates, configure live provider secrets only in production, take a fresh backup, apply the migration once, deploy the exact staging-tested digest, and monitor a controlled release. A live Stripe transaction can charge money; agree on the specific test order, amount, and refund procedure before any live payment test.

## Provisioning checklist

1. Create a DigitalOcean project and a VPC in the chosen region. Keep the app Droplet and managed PostgreSQL in a network configuration that supports private connectivity. Record the actual private endpoint and addresses only after provisioning.
2. Create the app Droplet with an Ubuntu LTS image, SSH-key access, and a size selected from measured staging load. Reserve its public IP for DNS. The Droplet runs the TLS reverse proxy and `compose.production.yaml`; its public address is not a reason to expose application or worker ports.
3. Create managed PostgreSQL and attach it to the intended private network. Restrict trusted sources to the app Droplet/resource that runs the database clients; do not allow public sources. Use the provider endpoint and require `sslmode=verify-full`. If the provider CA is not in the image's system trust store, extend `compose.production.yaml` to mount the provider CA read-only into every service that connects to PostgreSQL, then set `sslrootcert` in `DATABASE_URL` to its in-container path. The current Compose file does not mount a provider CA. The application, OMS, and workers share this database URL.
4. Create or select the separate Kafka service. Configure a TLS listener and a dedicated SASL identity for the order relay, restrict broker access to the app host's private source, and pre-create `KAFKA_ORDER_TOPIC` with an intentional partition, replication, and retention policy. Grant the relay only the topic permissions it needs. Do not expose an unauthenticated or plaintext listener to the public internet.
5. Configure DigitalOcean Cloud Firewalls and any host firewall. On the app Droplet, permit inbound TCP 80/443 for the storefront and SSH only from approved administrator CIDRs. Do not open 8080, OMS, gRPC, or metrics ports publicly. Permit database and Kafka traffic only over the intended private paths and only to their configured endpoints/ports. If Kafka is outside DigitalOcean, apply equivalent source restrictions at that provider. Preserve outbound access required for DNS, operating-system updates, image pulls, Stripe, SMTP, and any configured telemetry.
6. Point the storefront DNS name at the reserved app IP, install and test the TLS reverse proxy, and configure certificate renewal. The proxy should forward only to the loopback-bound app port. Determine `TRUSTED_PROXY_CIDRS` from the peer address the app actually observes from the local proxy/container network; do not guess it from the Droplet public or VPC address.
7. Put production secrets in the selected secret-management mechanism. Set `DATABASE_URL` with the exact provider hostname, `sslmode=verify-full`, and, when needed, `sslrootcert`; configure Kafka TLS/SASL, SMTP, Stripe, staff credentials, and legal seller data. Keep credentials out of images, source control, and command history.
8. Before accepting customers, apply migrations once using the release image digest, start the Compose services, verify readiness and relay health, and test account email verification/reset, a Stripe test-mode checkout and webhook in staging, and delivery of an order event to Kafka. Then verify database backups/PITR and restore into an isolated database; do not treat a backup setting alone as a successful recovery test.

## Launch inputs still required

- DigitalOcean region, app Droplet size, and whether the single-host availability risk is acceptable.
- Storefront domain, DNS operator, and verified TLS certificate-renewal path.
- Managed PostgreSQL plan/endpoint, private connectivity, trusted-source configuration, CA handling, backup retention, and restore target.
- Kafka provider/location, bootstrap endpoint, broker TLS CA, SASL credentials, topic policy, and firewall path.
- Actual proxy peer CIDR observed by the app, SMTP provider/domain authentication, Stripe live credentials and webhook endpoint, and verified seller/tax details.

Do not deploy until each item is decided and validated in staging. The general production release sequence and required application variables are documented in the repository's [production checklist](../../README.md#produktion).
