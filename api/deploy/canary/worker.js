// Chapter 47 — the canary router.
//
// Blue-green flips everyone at once. A canary moves a percentage, holds, and
// watches. The detail that makes it an experiment rather than a coin toss is
// the STABLE hash: the same client keeps the same routing decision for the
// whole hold window, so if the canary is broken, the same small group sees it
// break rather than everybody seeing it flicker.
//
// [verbatim ch47]
export default {
  async fetch(request, env) {
    const canaryColor = await env.BEACON_CONFIG.get("canary_color");
    const canaryPctRaw = await env.BEACON_CONFIG.get("canary_pct");
    const canaryPct = parseInt(canaryPctRaw || "0", 10);

    // Cheap deterministic hash from a stable client identifier.
    // Same user keeps the same routing decision across a session.
    const clientId = request.headers.get("cf-connecting-ip") || "anon";
    const bucket = hashToInt(clientId) % 100;

    const goCanary = canaryColor && bucket < canaryPct;
    const stable = canaryColor === "green" ? "blue" : "green";
    const target = goCanary ? canaryColor : stable;

    const url = new URL(request.url);
    url.host = `beacon-api-${target}.fly.dev`;
    return fetch(url.toString(), request);
  },
};

function hashToInt(s) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}
