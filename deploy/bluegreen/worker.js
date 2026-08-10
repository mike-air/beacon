// Chapter 46 — the blue-green router.
//
// Two identical stacks exist at all times. One is live and taking every
// request; the other is dark, with zero users on it. You deploy to the dark
// one, smoke-test it from outside like a real client, and then flip a single
// value. Every request after the flip goes to the new stack; every request
// before it finished on the old one.
//
// The thing worth internalising: ROLLBACK IS A CONFIG FLIP, NOT A REDEPLOY.
// The old stack stays warm for a ~30-minute window with its connection pool
// open, so undoing a bad deploy takes seconds instead of a build.
//
// Deploy with: wrangler deploy
//
// [verbatim ch46]
export default {
  async fetch(request, env) {
    const live = await env.BEACON_CONFIG.get("live_color");
    const upstream = live === "green"
      ? "https://beacon-api-green.fly.dev"
      : "https://beacon-api-blue.fly.dev";
    const url = new URL(request.url);
    url.host = new URL(upstream).host;
    return fetch(url.toString(), request);
  },
};
