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

/**
 * Server-Sent Events fetcher — for `'use client'` components consuming
 * a long-lived `text/event-stream` endpoint. Follows the same Clerk-JWT
 * conventions as `browserFetch`. Calls `onEvent(eventName, data)` for
 * each SSE frame; resolves when the server closes the stream; rejects
 * on non-2xx response.
 *
 * Caller is expected to pass an `AbortSignal` and call `.abort()` on
 * unmount so the underlying fetch + reader can be torn down. The body
 * is parsed as a stream so partial frames at chunk boundaries are
 * buffered until the next chunk lands.
 */
export async function browserFetchSSE(
  path: string,
  onEvent: (event: string, data: unknown) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (typeof window === 'undefined') {
    throw new ApiError(0, null, 'browserFetchSSE invoked outside a browser context');
  }
  const Clerk = (window as unknown as { Clerk?: ClerkGlobal }).Clerk;
  if (!Clerk?.session) {
    throw new ApiError(401, null, 'not signed in');
  }
  const token = await Clerk.session.getToken();

  const headers = new Headers();
  if (token) headers.set('Authorization', `Bearer ${token}`);
  headers.set('Accept', 'text/event-stream');

  const url = `${PUBLIC_API_URL}${path}`;
  const res = await fetch(url, { headers, signal, cache: 'no-store' });
  if (!res.ok || !res.body) {
    let body: unknown = null;
    try {
      body = await res.text();
    } catch {
      // ignore
    }
    throw new ApiError(res.status, body, `GET ${url} → ${res.status}`);
  }

  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return;
    buf += value;
    // SSE frames are separated by a blank line (\n\n).
    for (let idx = buf.indexOf('\n\n'); idx !== -1; idx = buf.indexOf('\n\n')) {
      const frame = buf.slice(0, idx);
      buf = buf.slice(idx + 2);
      const eventLine = frame.match(/^event:\s*(.+)$/m)?.[1] ?? 'message';
      const dataLine = frame.match(/^data:\s*(.+)$/m)?.[1] ?? '';
      try {
        onEvent(eventLine, dataLine ? JSON.parse(dataLine) : null);
      } catch {
        // malformed frame — skip but keep streaming
      }
    }
  }
}
