/**
 * One error type for every failed request.
 *
 * The generated SDK returns `{ data, error, response }` rather than throwing,
 * which is a reasonable default and the wrong ergonomics for an application:
 * every call site would need the same four lines to decide whether it had an
 * answer. The wrapper in client.ts throws this instead.
 *
 * `code` is the stable part of Beacon's error envelope. Branch on it. Never
 * branch on `message` — that is a string a translator will change.
 */
import type { FieldError, RowError } from "./generated";

export class BeaconError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: FieldError[];
  /**
   * Per-row detail from a bulk import that failed validation. Empty for
   * every other error — only the import endpoint fills it, and it does so
   * INSTEAD of writing anything, so a non-empty rows means nothing landed.
   */
  readonly rows: RowError[];
  /** Seconds, from Retry-After. Only ever set on a 429. */
  readonly retryAfter?: number;
  readonly requestId?: string;

  constructor(init: {
    status: number;
    code: string;
    message: string;
    fields?: FieldError[];
    rows?: RowError[];
    retryAfter?: number;
    requestId?: string;
  }) {
    super(init.message);
    this.name = "BeaconError";
    this.status = init.status;
    this.code = init.code;
    this.fields = init.fields ?? [];
    this.rows = init.rows ?? [];
    this.retryAfter = init.retryAfter;
    this.requestId = init.requestId;
  }

  /** The token is gone or expired — the one error handled globally. */
  get isUnauthenticated() {
    return this.status === 401;
  }
  get isForbidden() {
    return this.status === 403;
  }
  get isValidation() {
    return this.status === 422;
  }
  get isRateLimited() {
    return this.status === 429;
  }
  /** The server does not have this feature at all — storage is unconfigured. */
  get isNotImplemented() {
    return this.status === 501;
  }

  /**
   * Retrying a 4xx sends the same wrong request again.
   *
   * 501 and 505 are the two 5xx codes that are permanent: "this server does
   * not implement that" and "not this HTTP version". Retrying them is several
   * round trips to be told the same thing several times — which is exactly
   * what an earlier version of this client did to /attachments on a Beacon
   * with no storage configured.
   */
  get isRetryable() {
    if (this.status === 501 || this.status === 505) return false;
    return this.status === 429 || this.status >= 500;
  }

  /** Validation detail keyed by field, ready to hand to a form. */
  fieldErrors(): Record<string, string> {
    const out: Record<string, string> = {};
    for (const f of this.fields) if (f.field) out[f.field] = f.message ?? "";
    return out;
  }
}

/** The request never reached the server, or the response was not readable. */
export class NetworkError extends Error {
  readonly cause?: unknown;
  constructor(message: string, cause?: unknown) {
    super(message);
    this.name = "NetworkError";
    this.cause = cause;
  }
}
