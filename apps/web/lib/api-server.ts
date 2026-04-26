import 'server-only';

import { auth } from '@clerk/nextjs/server';
import { ApiError } from './api-client';

const INTERNAL_API_URL = process.env.INTERNAL_API_URL ?? 'http://localhost:8080';

interface FetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
}

/**
 * Server-side fetcher — for server components, route handlers, and
 * server actions. Forwards the Clerk session JWT to the Go API.
 */
export async function serverFetch<T = unknown>(
  path: string,
  options: FetchOptions = {},
): Promise<T> {
  const session = await auth();
  const token = await session.getToken();

  const headers = new Headers(options.headers);
  if (token) headers.set('Authorization', `Bearer ${token}`);
  if (options.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const init: RequestInit = {
    ...options,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    cache: options.cache ?? 'no-store',
  };

  const url = `${INTERNAL_API_URL}${path}`;
  const res = await fetch(url, init);
  let body: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }
  if (!res.ok) {
    throw new ApiError(res.status, body, `${init.method ?? 'GET'} ${url} → ${res.status}`);
  }
  return body as T;
}
