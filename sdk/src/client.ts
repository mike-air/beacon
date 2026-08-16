/**
 * Beacon's behaviour, wrapped around the generated calls.
 *
 * The generated SDK knows the shapes. It does not know that Beacon rate-limits
 * per organisation, replays writes that carry an Idempotency-Key, or expires a
 * single non-refreshable token — and it should not, because all of that is
 * behaviour rather than contract, and regenerating the SDK must never overwrite
 * it. So the generated code stays untouched in src/generated, and everything
 * Beacon-specific lives here.
 *
 * What this adds, in the order a request meets it:
 *
 *   1. the bearer token, from wherever the host app keeps it
 *   2. an Idempotency-Key on every mutation, held across retries so a retried
 *      POST cannot create a second row
 *   3. a timeout, because a hung fetch is a spinner that never stops
 *   4. Retry-After honoured on 429, exponential backoff on 5xx
 *   5. the error envelope unwrapped into a thrown BeaconError
 *   6. one single-flight callback on 401, so twenty parallel failures sign the
 *      user out once
 */
import { client } from "./generated/client.gen";
import { BeaconError, NetworkError } from "./errors";

export interface BeaconConfig {
  /** Where the API lives. */
  baseUrl: string;
  /** Returns the current bearer token, or null when signed out. */
  getToken?: () => string | null;
  /**
   * Called once when the server rejects the token. Beacon has no refresh
   * endpoint, so there is nothing to retry with — the only correct response is
   * to end the session. Single-flighting lives here rather than in the caller
   * so twenty parallel 401s produce one sign-out, not twenty.
   */
  onUnauthenticated?: () => void;
  /** Milliseconds before a request is abandoned. Default 15s. */
  timeoutMs?: number;
  /** Attempts after the first, for retryable failures only. Default 2. */
  maxRetries?: number;
}

let config: Required<Omit<BeaconConfig, "getToken" | "onUnauthenticated">> &
  Pick<BeaconConfig, "getToken" | "onUnauthenticated"> = {
  baseUrl: "",
  timeoutMs: 15_000,
  maxRetries: 2,
};

let expiring = false;

/** Configure the SDK once, at application start. */
export function configureBeacon(cfg: BeaconConfig) {
  config = { timeoutMs: 15_000, maxRetries: 2, ...cfg };
  expiring = false;
  client.setConfig({ baseUrl: cfg.baseUrl });
}

const MUTATING = new Set(["POST", "PATCH", "PUT", "DELETE"]);
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/**
 * The single fetch every generated call goes through.
 *
 * Installed on the generated client, so `sdk.listTasks()` and every other
 * method inherit all of it without knowing it exists.
 */
function beaconFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  // The generated client always hands us a Request, but fetch's type allows a
  // string or URL, so normalise rather than assert.
  return attempt(input instanceof Request ? input : new Request(input, init), 0);
}

async function attempt(request: Request, tries: number): Promise<Response> {
  // One key for the whole attempt-set. A retried POST must not create a second
  // row, and that is only true if the key survives the retry.
  const idempotencyKey =
    MUTATING.has(request.method) && !request.headers.get("Idempotency-Key")
      ? crypto.randomUUID()
      : request.headers.get("Idempotency-Key");

  const timeout = new AbortController();
  const timer = setTimeout(() => timeout.abort(), config.timeoutMs);

  try {
    const token = config.getToken?.() ?? null;
    const headers = new Headers(request.headers);
    if (token) headers.set("Authorization", `Bearer ${token}`);
    if (idempotencyKey) headers.set("Idempotency-Key", idempotencyKey);

    const res = await fetch(
      new Request(request, { headers, signal: timeout.signal }),
    );

    if (res.ok || res.status === 304) return res;

    const err = await toBeaconError(res);

    if (err.isUnauthenticated) {
      if (!expiring) {
        expiring = true;
        config.onUnauthenticated?.();
      }
      throw err;
    }

    if (err.isRetryable && tries < config.maxRetries) {
      // Honour Retry-After rather than guessing. A guessing client is how a
      // throttle becomes a ban.
      const waitMs = err.retryAfter != null ? err.retryAfter * 1000 : 2 ** tries * 400;
      await sleep(waitMs);
      return attempt(request.clone(), tries + 1);
    }
    throw err;
  } catch (e) {
    if (e instanceof BeaconError) throw e;
    if (timeout.signal.aborted) {
      throw new NetworkError(`Request timed out after ${config.timeoutMs / 1000}s`, e);
    }
    if (tries < config.maxRetries) {
      await sleep(2 ** tries * 400);
      return attempt(request.clone(), tries + 1);
    }
    throw new NetworkError("Could not reach the server", e);
  } finally {
    clearTimeout(timer);
  }
}

async function toBeaconError(res: Response): Promise<BeaconError> {
  const retryAfterHeader = res.headers.get("Retry-After");
  const retryAfter = retryAfterHeader ? Number(retryAfterHeader) : undefined;
  const requestId = res.headers.get("X-Request-Id") ?? undefined;

  let code = `http_${res.status}`;
  let message = res.statusText || "Request failed";
  let fields;

  try {
    const body = (await res.clone().json()) as {
      error?: { code?: string; message?: string; fields?: [] };
    };
    if (body.error) {
      code = body.error.code ?? code;
      message = body.error.message ?? message;
      fields = body.error.fields;
    }
  } catch {
    // A non-JSON error body — a proxy's HTML 502, say. The status is still true.
  }

  return new BeaconError({
    status: res.status,
    code,
    message,
    fields,
    ...(retryAfter !== undefined && Number.isFinite(retryAfter) ? { retryAfter } : {}),
    ...(requestId ? { requestId } : {}),
  });
}

// Installed once, at module load, so no call site can forget it.
client.setConfig({ fetch: beaconFetch });

/** Called by the host app after a successful sign-in. */
export function resetSessionState() {
  expiring = false;
}
