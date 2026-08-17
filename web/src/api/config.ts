/**
 * Where the API lives, resolved once.
 *
 * Three sources, most specific first:
 *
 *   window.__BEACON__.apiBase   written by the container at start-up
 *   import.meta.env.VITE_API_BASE   inlined at build time (dev, and CI)
 *   http://localhost:8080       what `make api` serves
 *
 * The runtime source exists so ONE built image can run in staging and in
 * production. Vite inlines env vars at build time, so without this the base
 * is welded into the bundle and every environment needs its own build —
 * which means the artefact you tested is not the artefact you ship.
 */
declare global {
  interface Window {
    __BEACON__?: { apiBase?: string };
  }
}

function resolve(): string {
  const runtime = typeof window !== "undefined" ? window.__BEACON__?.apiBase : undefined;
  if (runtime) return runtime.replace(/\/$/, "");
  const built = import.meta.env["VITE_API_BASE"];
  if (built) return String(built).replace(/\/$/, "");
  return "http://localhost:8080";
}

export const API_BASE: string = resolve();
