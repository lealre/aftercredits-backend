FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build backend API
RUN go build -o backend ./main.go

# Build database cli
RUN go build -o database ./cmd/database

# Temporary (removed after the production cutover): the one-time
# Mongo -> Postgres migration tool, runnable on the Pi without a Go toolchain.
RUN go build -o mongo-to-postgres ./cmd/mongo-to-postgres


FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/backend /app/backend
COPY --from=builder /app/database /app/database
COPY --from=builder /app/mongo-to-postgres /app/mongo-to-postgres

CMD ["/app/backend"]
