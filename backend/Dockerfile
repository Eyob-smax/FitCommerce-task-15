# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Install sqlc for code generation
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0

# Download deps (separate layer — cached unless go.mod changes)
COPY go.mod ./
ENV GONOSUMDB=*
ENV GOFLAGS=-mod=mod
RUN go mod download

# Copy full source
COPY . .

# Generate sqlc code (idempotent; output goes to internal/database/generated/)
RUN sqlc generate || true

# Build API binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/api ./cmd/api

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /app/bin/api ./api

# Migrations are embedded in the binary via go:embed (database/migrate.go)
# but we also copy for reference
COPY --from=builder /app/database/migrations ./database/migrations

# Export directory (mounted as volume)
RUN mkdir -p /app/exports

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8080

CMD ["./api"]
