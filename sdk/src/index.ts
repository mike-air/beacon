/**
 * @beacon/sdk — the typed client for the Beacon API.
 *
 * Everything under ./generated is produced by `npm run generate` from
 * ../api/openapi.json, which the Go handlers emit. Do not edit it; the next
 * regeneration will overwrite you.
 *
 * Everything else in this directory is hand-written and permanent: the error
 * type, and the wrapper that adds authentication, idempotency, retries and
 * timeouts to every generated call.
 *
 * Usage:
 *
 *   import { configureBeacon, beacon } from "@beacon/sdk";
 *
 *   configureBeacon({
 *     baseUrl: import.meta.env.VITE_API_BASE,
 *     getToken: () => localStorage.getItem("beacon-token"),
 *     onUnauthenticated: () => signOut(),
 *   });
 *
 *   const me = await beacon.getMe();
 *   const tasks = await beacon.listTasks({ path: { orgID, projectID } });
 *
 * Each method carries the documentation its Go handler declared, so hovering
 * `beacon.search` in an editor shows why a `postgres` source is worth telling
 * the user about.
 */
import { Sdk } from "./generated";
import { client } from "./generated/client.gen";

export { configureBeacon, resetSessionState, type BeaconConfig } from "./client";
export { BeaconError, NetworkError } from "./errors";
import { BeaconError, NetworkError } from "./errors";

/**
 * The SDK instance.
 *
 * Every method returns the generated `{ data, error, response }` triple. The
 * `unwrap` helper below turns that into "the value, or a thrown BeaconError",
 * which is the ergonomics an application wants — otherwise every call site
 * repeats the same four lines to decide whether it has an answer.
 */
export const beacon = new Sdk({ client });

export { Sdk };

/**
 * Turn a generated call into a value or a throw.
 *
 * The generated methods resolve with { data, error } rather than rejecting,
 * which is a sound default for a library and the wrong shape for a UI: React
 * Query, error boundaries and try/catch all speak exceptions.
 *
 * The error branch below is the ONLY one that runs on a failed request, which
 * is the opposite of what this function assumed until a bulk import made it
 * obvious. The old comment here reasoned that the fetch wrapper throws a
 * BeaconError, so `error` would be populated only in cases it had not
 * handled. In fact the generated client catches whatever the fetch layer
 * throws and hands it straight back as `error` — with NO `response`, because
 * there was no response to report. So this branch ran on every failure, read
 * `response.status` off undefined, and turned every error in the application
 * into "Cannot read properties of undefined (reading 'status')".
 *
 * The fix is to stop rewrapping. Our fetch wrapper already built a complete
 * BeaconError: status, the stable `code`, the message, the field errors and
 * the per-row import errors. Constructing a second one on top could only
 * throw that detail away even when it did not crash.
 */
export async function unwrap<T>(
  call: Promise<{ data?: T; error?: unknown; response?: Response }>,
): Promise<T> {
  const { data, error, response } = await call;
  if (error !== undefined) {
    // Already the error we want: rethrow it untouched.
    if (error instanceof BeaconError || error instanceof NetworkError) throw error;

    // Something the fetch wrapper did not produce. `response` may or may not
    // exist here, so this cannot assume it — assuming it is what broke.
    throw new BeaconError({
      status: response?.status ?? 0,
      code: response ? `http_${response.status}` : "client_error",
      message: response?.statusText || "Request failed",
    });
  }
  return data as T;
}

// Every shape the API can send or receive. Re-exported so an application
// imports one package rather than reaching into a generated directory.
export type * from "./generated/types.gen";
