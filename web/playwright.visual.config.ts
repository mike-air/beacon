import { defineConfig, devices } from "@playwright/test";

/**
 * Visual regression, run against the PRODUCTION CONTAINER.
 *
 * Two decisions here are worth the space.
 *
 * FIRST: these run in Linux, always, including on a Mac. Font rasterisation
 * differs between operating systems — the same CSS produces different pixels
 * on macOS and on the CI runner — so a baseline captured on a laptop fails on
 * every CI run for reasons that have nothing to do with the change. The fix is
 * not a loose threshold, which hides real regressions; it is to render in one
 * place. `make visual` puts both the app and the browser in containers.
 *
 * The guard below refuses to run outside Linux rather than quietly recording
 * macOS baselines that CI can never match.
 *
 * SECOND: the target is the built image on nginx, not `vite dev`. Dev serves
 * unminified modules with different font loading; the container is the thing
 * that ships. Testing the artefact is the point.
 */
if (process.platform !== "linux") {
  throw new Error(
    "Visual tests must run in the Linux container so baselines match CI.\n" +
      "Run:  make visual          (compare against the committed baselines)\n" +
      "      make visual-update   (re-record them after an intended change)",
  );
}

export default defineConfig({
  testDir: "./visual",
  fullyParallel: true,
  forbidOnly: !!process.env["CI"],
  retries: 0,
  reporter: process.env["CI"] ? "github" : "list",
  timeout: 30_000,

  // No platform suffix: there is only ever one platform, enforced above.
  snapshotPathTemplate: "{testDir}/__screenshots__/{arg}{ext}",

  expect: {
    toHaveScreenshot: {
      // A handful of anti-aliased pixels along a curve is not a regression.
      // Anything that changes layout, colour or spacing moves far more.
      maxDiffPixelRatio: 0.002,
      animations: "disabled",
      caret: "hide",
      scale: "css",
    },
  },

  use: {
    baseURL: process.env["VISUAL_BASE_URL"] ?? "http://web:8080",
    trace: "retain-on-failure",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 900 } },
    },
  ],
});
