# syntax=docker/dockerfile:1.7
# Build context = repo root.

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

COPY go.work go.work.sum ./
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
COPY apps/worker/go.mod apps/worker/go.sum ./apps/worker/
COPY pkg/go.mod pkg/go.sum ./pkg/

RUN cd apps/api    && go mod download \
 && cd ../worker   && go mod download \
 && cd ../../pkg   && go mod download

COPY apps/ ./apps/
COPY pkg/ ./pkg/

WORKDIR /workspace/apps/worker
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/worker /app/worker
CMD ["/app/worker"]
