# syntax=docker/dockerfile:1.7
# Build context = repo root. Builds the Next.js standalone bundle, then
# copies just the runtime artifacts into a slim node image.

FROM node:20-bookworm-slim AS deps
RUN corepack enable
WORKDIR /workspace

# Workspace manifests first for layer caching.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json ./
COPY apps/web/package.json ./apps/web/
COPY packages/ ./packages/

RUN --mount=type=cache,id=s/4abb0f50-6105-4c2b-badf-6d95a32b77dd-pnpm,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

# ---- builder ----
FROM node:20-bookworm-slim AS builder
RUN corepack enable
WORKDIR /workspace

COPY --from=deps /workspace/node_modules ./node_modules
COPY --from=deps /workspace/apps/web/node_modules ./apps/web/node_modules
COPY --from=deps /workspace/packages ./packages
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml turbo.json biome.json ./

COPY apps/web/ ./apps/web/

# Next.js inlines NEXT_PUBLIC_* into the client bundle at `pnpm build`,
# not at runtime. Railway forwards declared service variables as
# --build-arg, so anything client code reads via process.env must be
# declared here AND set on the cadence-web service's Variables tab.
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=${NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY}
ENV NODE_ENV=production
RUN cd apps/web && pnpm build

# ---- runtime ----
FROM node:20-bookworm-slim AS runtime
WORKDIR /workspace

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# Next.js standalone output bundles a minimal node_modules + server.js.
# `outputFileTracingRoot=/workspace` (set in next.config.ts) means the
# standalone tree is rooted at /workspace, with apps/web/server.js as the
# entry point.
COPY --from=builder /workspace/apps/web/.next/standalone/ ./
COPY --from=builder /workspace/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /workspace/apps/web/public ./apps/web/public

EXPOSE 3000
CMD ["node", "apps/web/server.js"]
