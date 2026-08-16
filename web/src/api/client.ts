/**
 * The one place a request is made.
 *
 * Everything the server does that a naive client gets wrong is handled here,
 * once, so no component has to remember it:
 *
 *   - the bearer token, and a single-flight session expiry on 401 (ch28)
 *   - the error envelope, unwrapped into ApiError (ch21)
 *   - Idempotency-Key on every mutation, generated per attempt-set (ch14 of
 *     the API course: the key identifies the INTENT, so a retry reuses it)
 *   - 429 with Retry-After, honoured rather than hammered (ch19)
 *   - a request timeout, because a hung fetch is a spinner that never stops
 */
import { ApiError, NetworkError } from "./errors";
import { expireSession, getToken } from "./session";

export const API_BASE: string = import.meta.env["VITE_API_BASE"] ?? "http://localhost:8080";

const TIMEOUT_MS = 15_000;
const MAX_RETRIES = 2;

type Method = "GET" | "POST" | "PATCH" | "DELETE";

export interface RequestOptions {
  method?: Method;
  body?: unknown;
  query?: Record<string, string | number | undefined>;
  signal?: AbortSignal;
  /**
   * Reuse a key across retries of the SAME user intent. Omit and one is
   * generated — which is right for a fresh action and wrong for a retry.
   */
  idempotencyKey?: string;
  headers?: Record<string, string>;
}

function buildUrl(path: string, query?: RequestOptions["query"]): string {
  const url = new URL(path.startsWith("http") ? path : API_BASE + path);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== "") url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}

export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function toApiError(res: Response): Promise<ApiError> {
  const requestId = res.headers.get("X-Request-Id") ?? undefined;
  const retryAfterHeader = res.headers.get("Retry-After");
  const retryAfter = retryAfterHeader ? Number(retryAfterHeader) : undefined;

  let code = `http_${res.status}`;
  let message = res.statusText || "Request failed";
  let fields;

  try {
    const body = (await res.json()) as {
      error?: { code?: string; message?: string; fields?: [] };
    };
    if (body.error) {
      code = body.error.code ?? code;
      message = body.error.message ?? message;
      fields = body.error.fields;
    }
  } catch {
    // A non-JSON error body (a proxy's HTML 502, say). The status is still true.
  }

  return new ApiError({
    status: res.status,
    code,
    message,
    fields,
    ...(retryAfter !== undefined && Number.isFinite(retryAfter) ? { retryAfter } : {}),
    ...(requestId ? { requestId } : {}),
  });
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const method = opts.method ?? "GET";
  const mutating = method !== "GET";

  // One key for the whole attempt-set. A retry of a failed POST must NOT
  // create a second row, and that is only true if the key survives the retry.
  const idemKey = mutating ? (opts.idempotencyKey ?? newIdempotencyKey()) : undefined;

  let attempt = 0;
  for (;;) {
    const timeout = new AbortController();
    const timer = setTimeout(() => timeout.abort(), TIMEOUT_MS);
    // Caller aborts (a cancelled query) and our timeout both have to work.
    const onAbort = () => timeout.abort();
    opts.signal?.addEventListener("abort", onAbort);

    try {
      const token = getToken();
      const headers: Record<string, string> = {
        Accept: "application/json",
        ...(opts.body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(idemKey ? { "Idempotency-Key": idemKey } : {}),
        ...opts.headers,
      };

      const res = await fetch(buildUrl(path, opts.query), {
        method,
        headers,
        ...(opts.body !== undefined ? { body: JSON.stringify(opts.body) } : {}),
        signal: timeout.signal,
      });

      if (res.status === 204) return undefined as T;

      if (!res.ok) {
        const err = await toApiError(res);

        // The only error handled globally. Single-flight lives in session.ts,
        // so twenty parallel 401s produce one sign-out.
        if (err.isUnauthenticated) {
          expireSession();
          throw err;
        }

        // Honour Retry-After rather than guessing. Without it, back off.
        if (err.isRetryable && attempt < MAX_RETRIES) {
          const waitMs = err.retryAfter != null ? err.retryAfter * 1000 : 2 ** attempt * 400;
          attempt += 1;
          await sleep(waitMs);
          continue;
        }
        throw err;
      }

      return (await res.json()) as T;
    } catch (e) {
      if (e instanceof ApiError) throw e;
      if (opts.signal?.aborted) throw e; // the caller cancelled; not our failure
      if (timeout.signal.aborted) {
        throw new NetworkError(`Request timed out after ${TIMEOUT_MS / 1000}s`, e);
      }
      if (attempt < MAX_RETRIES) {
        attempt += 1;
        await sleep(2 ** attempt * 400);
        continue;
      }
      throw new NetworkError("Could not reach the server", e);
    } finally {
      clearTimeout(timer);
      opts.signal?.removeEventListener("abort", onAbort);
    }
  }
}

export const api = {
  get: <T>(path: string, opts?: Omit<RequestOptions, "method" | "body">) =>
    request<T>(path, { ...opts, method: "GET" }),
  post: <T>(path: string, body?: unknown, opts?: Omit<RequestOptions, "method" | "body">) =>
    request<T>(path, { ...opts, method: "POST", body }),
  patch: <T>(path: string, body?: unknown, opts?: Omit<RequestOptions, "method" | "body">) =>
    request<T>(path, { ...opts, method: "PATCH", body }),
  del: <T>(path: string, opts?: Omit<RequestOptions, "method" | "body">) =>
    request<T>(path, { ...opts, method: "DELETE" }),
};
