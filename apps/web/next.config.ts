import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // Emit a self-contained server bundle at .next/standalone — required by
  // the production Dockerfile (apps/web/Dockerfile) so the runtime image
  // doesn't need node_modules.
  output: 'standalone',
  // The standalone tracer needs the workspace root to follow pnpm symlinks
  // up to the monorepo root.
  outputFileTracingRoot: process.env.NODE_ENV === 'production' ? '/workspace' : undefined,
};

export default nextConfig;
