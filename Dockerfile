FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build backend API
RUN go build -o backend ./main.go

# Build indexes job
RUN go build -o indexes ./cmd/indexes


FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/backend /app/backend
COPY --from=builder /app/indexes /app/indexes

CMD ["/app/backend"]
