import { defineConfig, devices } from "@playwright/test";

/**
 * The e2e suite runs against a REAL Beacon, not a mock.
 *
 * A mocked backend proves the client agrees with the mock. Every genuine bug
 * this project has hit — the CORS preflight that forbade Idempotency-Key, the
 * five list endpoints with the wrong envelope, the 501 that rendered nothing —
 * would have passed against a mock and failed against the server. So these
 * tests need `make db-up && make run` in ../beacon, and they say so loudly
 * when it is missing rather than failing with a cryptic timeout.
 */
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false, // one shared Beacon; parallel signups race on email
  forbidOnly: !!process.env["CI"],
  retries: process.env["CI"] ? 1 : 0,
  workers: 1,
  reporter: process.env["CI"] ? "github" : "list",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:5180",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "npm run dev",
    url: "http://localhost:5180",
    reuseExistingServer: true,
    timeout: 60_000,
  },
});
