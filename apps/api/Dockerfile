# syntax=docker/dockerfile:1.7
# Build context = repo root (set via Railway "Root Directory = /" + this
# Dockerfile path). We need go.work + all module manifests to resolve the
# go workspace.

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# 1. Copy workspace manifest + each module's go.mod/go.sum first so the
#    dependency layer caches independently of source changes.
COPY go.work go.work.sum ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY apps/worker/go.mod apps/worker/go.sum ./apps/worker/
COPY pkg/go.mod pkg/go.sum ./pkg/

# 2. Pre-download deps (cached as long as go.mod/sum don't change).
RUN cd apps/api    && go mod download \
 && cd ../worker   && go mod download \
 && cd ../../pkg   && go mod download

# 3. Copy actual source.
COPY apps/ ./apps/
COPY pkg/ ./pkg/

# 4. Build the api binary, statically linked, stripped.
WORKDIR /workspace/apps/api
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

# 5. Goose for migrations (run via railway.toml's preDeployCommand).
WORKDIR /workspace
RUN --mount=type=cache,target=/root/.cache/go-build \
    go install -trimpath github.com/pressly/goose/v3/cmd/goose@latest

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /go/bin/goose /app/goose
COPY --from=builder /workspace/apps/api/internal/db/migrations /app/migrations

ENV PORT_API=8080
EXPOSE 8080
CMD ["/app/api"]
