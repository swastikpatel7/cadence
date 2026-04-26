import { ApiError } from './api-client';

const PUBLIC_API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

interface FetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
}

interface ClerkGlobal {
  session?: { getToken: () => Promise<string | null> };
}

/**
 * Client-side fetcher — for `'use client'` components. Reads the Clerk
 * session JWT off `window.Clerk`, which `<ClerkProvider>` mounts. Uses
 * the public API URL so the browser can reach Go directly (no Next.js
 * proxy hop in v1).
 */
export async function browserFetch<T = unknown>(
  path: string,
  options: FetchOptions = {},
): Promise<T> {
  if (typeof window === 'undefined') {
    throw new ApiError(0, null, 'browserFetch invoked outside a browser context');
  }
  const Clerk = (window as unknown as { Clerk?: ClerkGlobal }).Clerk;
  if (!Clerk?.session) {
    throw new ApiError(401, null, 'not signed in');
  }
  const token = await Clerk.session.getToken();

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

  const url = `${PUBLIC_API_URL}${path}`;
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
