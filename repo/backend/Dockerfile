# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Required for CGO/race builds in the backend-test container.
RUN apk add --no-cache gcc musl-dev

# Download deps (separate layer — cached unless go.mod changes)
COPY go.mod ./
ENV GONOSUMDB=*
ENV GOFLAGS=-mod=mod
RUN sh -ec 'for i in 1 2 3; do go mod download && go mod download all && exit 0; echo "go mod download failed (attempt $i), retrying..."; sleep $((i*5)); done; exit 1'

# Copy full source
COPY . .

# Build API binary
RUN sh -ec 'for i in 1 2 3; do CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/api ./cmd/api && exit 0; echo "go build failed (attempt $i), retrying..."; sleep $((i*5)); done; exit 1'

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /app/bin/api ./api

# Export directory (mounted as volume)
RUN mkdir -p /app/exports

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8080

CMD ["./api"]
