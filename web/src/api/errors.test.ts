import { describe, expect, it } from "vitest";
import { ApiError } from "./errors";

const err = (status: number, extra: Partial<ConstructorParameters<typeof ApiError>[0]> = {}) =>
  new ApiError({ status, code: `http_${status}`, message: "x", ...extra });

describe("ApiError.isRetryable", () => {
  it("never retries a 4xx — the request itself is wrong", () => {
    for (const s of [400, 401, 403, 404, 409, 422]) {
      expect(err(s).isRetryable).toBe(false);
    }
  });
  it("retries 429 and the transient 5xx", () => {
    for (const s of [429, 500, 502, 503, 504]) {
      expect(err(s).isRetryable).toBe(true);
    }
  });
  /**
   * The bug this guards. 501 and 505 are 5xx but permanent — the server does
   * not implement the thing and will not start to on the third attempt. The
   * client used to hammer /attachments five times on a Beacon with no storage.
   */
  it("does not retry the permanent 5xx", () => {
    expect(err(501).isRetryable).toBe(false);
    expect(err(505).isRetryable).toBe(false);
    expect(err(501).isNotImplemented).toBe(true);
  });
});

describe("ApiError.fieldErrors", () => {
  it("keys the server's 422 detail by field, ready for a form", () => {
    const e = err(422, {
      fields: [
        { field: "email", rule: "email", message: "not an email" },
        { field: "password", rule: "min", message: "too short" },
      ],
    });
    expect(e.isValidation).toBe(true);
    expect(e.fieldErrors()).toEqual({ email: "not an email", password: "too short" });
  });
  it("is empty when the server sent no field detail", () => {
    expect(err(422).fieldErrors()).toEqual({});
  });
});

describe("ApiError classification", () => {
  it("flags the one error handled globally", () => {
    expect(err(401).isUnauthenticated).toBe(true);
    expect(err(403).isUnauthenticated).toBe(false);
    expect(err(403).isForbidden).toBe(true);
  });
  it("carries Retry-After through so the client can honour it", () => {
    expect(err(429, { retryAfter: 12 }).retryAfter).toBe(12);
    expect(err(429).isRateLimited).toBe(true);
  });
});
