FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build backend API
RUN go build -o backend ./main.go

# Build database cli
RUN go build -o database ./cmd/database

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/backend /app/backend
COPY --from=builder /app/database /app/database

# Run unprivileged.
#
# Nothing this binary does needs root: it binds :8080 (above the privileged
# range), writes no files anywhere (the schema migrations are compiled in via
# //go:embed), serves no static content from disk, and logs to stdout.
#
# UID 1000 rather than an arbitrary non-root UID, because the deployment mounts
# env files that are mode 600 owned by the login account. A different UID would
# trade "runs as root" for "cannot read its own configuration" — and since
# godotenv ignores a failed load, that would degrade silently rather than
# fail loudly.
ARG APP_UID=1000
ARG APP_GID=1000
RUN addgroup -g "${APP_GID}" -S app && adduser -u "${APP_UID}" -G app -S -H -h /app app
USER app

CMD ["/app/backend"]
