FROM node:22-alpine AS frontend-build

WORKDIR /app
RUN corepack enable && corepack prepare pnpm@10 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
RUN pnpm --dir frontend install --frozen-lockfile
COPY frontend ./frontend
RUN pnpm --dir frontend run build

FROM golang:1.27.3-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /app/internal/web/static/react ./internal/web/static/react
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-migrate ./cmd/migrate \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-order-relay ./cmd/order-relay \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-payment-expiry ./cmd/payment-expiry \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-email-relay ./cmd/email-relay \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-reconcile-stripe ./cmd/reconcile-stripe \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /mitte-oms ./cmd/oms

FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -D -H -G app app
COPY --from=build --chown=app:app /mitte /mitte
COPY --from=build --chown=app:app /mitte-migrate /mitte-migrate
COPY --from=build --chown=app:app /mitte-order-relay /mitte-order-relay
COPY --from=build --chown=app:app /mitte-payment-expiry /mitte-payment-expiry
COPY --from=build --chown=app:app /mitte-email-relay /mitte-email-relay
COPY --from=build --chown=app:app /mitte-reconcile-stripe /mitte-reconcile-stripe
COPY --from=build --chown=app:app /mitte-oms /mitte-oms
USER app
EXPOSE 8080
EXPOSE 9090
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
CMD ["/mitte"]
