/**
 * Where the token lives, and what happens when it stops working.
 *
 * Beacon issues ONE JWT with a TTL (default 1h) and has no refresh endpoint.
 * production-frontend ch28 designs a refresh-race with single-flight; that
 * machinery has nothing to call here, so this is the honest version of it:
 * the first 401 ends the session exactly once, no matter how many requests
 * are in flight, and the UI asks the user to sign in again.
 *
 * See DEVIATIONS.md.
 */
const KEY = "beacon-token";

let listeners: (() => void)[] = [];
let expiring = false;

export function getToken(): string | null {
  return localStorage.getItem(KEY);
}

export function setToken(token: string) {
  localStorage.setItem(KEY, token);
  expiring = false;
  emit();
}

export function clearToken() {
  localStorage.removeItem(KEY);
  emit();
}

/**
 * Called by the client on any 401. Single-flight: twenty parallel requests
 * failing together produce ONE session-expired event, not twenty.
 */
export function expireSession() {
  if (expiring) return;
  expiring = true;
  localStorage.removeItem(KEY);
  emit();
}

export function isAuthenticated(): boolean {
  return getToken() !== null;
}

function emit() {
  for (const l of listeners) l();
}

/** Subscribe to sign-in / sign-out, including from another tab. */
export function subscribeSession(fn: () => void): () => void {
  listeners.push(fn);
  const onStorage = (e: StorageEvent) => {
    if (e.key === KEY) fn();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners = listeners.filter((l) => l !== fn);
    window.removeEventListener("storage", onStorage);
  };
}
