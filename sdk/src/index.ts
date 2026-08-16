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
import { BeaconError } from "./errors";

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
 * The fetch wrapper has already converted a failed response into a thrown
 * BeaconError, so `error` here is only populated in the cases it did not
 * handle — which is why this still guards rather than assuming.
 */
export async function unwrap<T>(
  call: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const { data, error, response } = await call;
  if (error !== undefined) {
    throw new BeaconError({
      status: response.status,
      code: `http_${response.status}`,
      message: response.statusText || "Request failed",
    });
  }
  return data as T;
}

// Every shape the API can send or receive. Re-exported so an application
// imports one package rather than reaching into a generated directory.
export type * from "./generated/types.gen";
