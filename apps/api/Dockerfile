# syntax=docker/dockerfile:1.7
# Build context = repo root (set via Railway "Root Directory = /" + this
# Dockerfile path). We need go.work + each module's go.mod/go.sum to
# resolve the go workspace.

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# 1. Copy workspace manifest + each module's go.mod/go.sum first so the
#    dependency layer caches independently of source changes.
COPY go.work go.work.sum ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY pkg/go.mod pkg/go.sum ./pkg/

# 2. Pre-download deps (cached as long as go.mod/sum don't change).
RUN cd apps/api && go mod download \
 && cd ../../pkg && go mod download

# 3. Copy actual source.
COPY apps/ ./apps/
COPY pkg/ ./pkg/

# 4. Build the api binary, statically linked, stripped. The binary
#    embeds both the HTTP server and the River worker pool.
WORKDIR /workspace/apps/api
RUN --mount=type=cache,id=s/2b23bc27-e613-4155-bc5a-cfd2bc71ac7b-gobuild,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

# 5. Goose for migrations (run via railway.toml's preDeployCommand).
WORKDIR /workspace
RUN --mount=type=cache,id=s/2b23bc27-e613-4155-bc5a-cfd2bc71ac7b-goose,target=/root/.cache/go-build \
    go install -trimpath github.com/pressly/goose/v3/cmd/goose@latest

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /go/bin/goose /app/goose
COPY --from=builder /workspace/apps/api/internal/db/migrations /app/migrations

# 6. Migration wrapper. Railway's preDeployCommand exec's its argv directly
#    without going through a shell, so `$DATABASE_URL` in railway.toml never
#    gets expanded. Sh expansion happens inside this script via the shebang.
RUN printf '#!/bin/sh\nset -e\nexec /app/goose -dir /app/migrations postgres "$DATABASE_URL" up\n' > /app/migrate.sh \
 && chmod +x /app/migrate.sh

ENV PORT_API=8080
EXPOSE 8080
CMD ["/app/api"]
