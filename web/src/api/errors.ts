/**
 * One error type for every failed request.
 *
 * The server's envelope is `{"error":{"code","message","fields"}}` and `code`
 * is the stable part. Components branch on `code`; they show `message`. They
 * never parse the message, because that is a string a translator will change.
 */
import type { FieldError } from "./types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: FieldError[];
  /** Seconds, from Retry-After. Only ever set on a 429. */
  readonly retryAfter?: number;
  readonly requestId?: string;

  constructor(init: {
    status: number;
    code: string;
    message: string;
    fields?: FieldError[];
    retryAfter?: number;
    requestId?: string;
  }) {
    super(init.message);
    this.name = "ApiError";
    this.status = init.status;
    this.code = init.code;
    this.fields = init.fields ?? [];
    this.retryAfter = init.retryAfter;
    this.requestId = init.requestId;
  }

  /** The token is gone or expired. The only error the whole app handles globally. */
  get isUnauthenticated(): boolean {
    return this.status === 401;
  }

  get isForbidden(): boolean {
    return this.status === 403;
  }

  get isValidation(): boolean {
    return this.status === 422;
  }

  get isRateLimited(): boolean {
    return this.status === 429;
  }

  /** The server does not have this feature at all — storage is unconfigured. */
  get isNotImplemented(): boolean {
    return this.status === 501;
  }

  /**
   * Retrying a 4xx sends the same broken request again.
   *
   * 501 and 505 are the two 5xx codes that are permanent: "this server does
   * not implement that" and "not this HTTP version". Retrying them is five
   * round trips to be told the same thing five times — which is exactly what
   * this client did to /attachments on a Beacon with no storage configured,
   * until this line existed.
   */
  get isRetryable(): boolean {
    if (this.status === 501 || this.status === 505) return false;
    return this.status === 429 || this.status >= 500;
  }

  /**
   * Validation failures keyed by field name, ready to hand to react-hook-form.
   * A field the form does not know about is dropped by the caller, not here.
   */
  fieldErrors(): Record<string, string> {
    const out: Record<string, string> = {};
    for (const f of this.fields) out[f.field] = f.message;
    return out;
  }
}

/** The request never reached the server, or the response was not JSON. */
export class NetworkError extends Error {
  readonly cause?: unknown;
  constructor(message: string, cause?: unknown) {
    super(message);
    this.name = "NetworkError";
    this.cause = cause;
  }
}
