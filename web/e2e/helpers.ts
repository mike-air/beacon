import { expect, type Page } from "@playwright/test";

export const API = process.env["VITE_API_BASE"] ?? "http://localhost:8080";
export const PASSWORD = "correct-horse-battery-staple";

/** Unique per run, so a re-run never collides with the last one's account. */
export function freshEmail(tag: string): string {
  return `${tag}-${Date.now()}-${Math.floor(Math.random() * 1e4)}@acme.test`;
}

/** Fail with a sentence an operator can act on, not a 30-second timeout. */
export async function requireBeacon() {
  let res: Response;
  try {
    res = await fetch(`${API}/healthz`);
  } catch {
    throw new Error(
      `Beacon is not running at ${API}. Start it with:\n` +
        `  cd ../api && make db-up && make run\n` +
        `These tests deliberately use a real server; a mock would pass on the bugs a real one catches.`,
    );
  }
  if (!res.ok) throw new Error(`Beacon answered ${res.status} at ${API}/healthz`);
}

// ---------------------------------------------------------------------------
// One account for the whole run.
//
// Beacon rate-limits /v1/auth by IP at 5 requests per minute (burst 10) —
// deliberately tight, because nobody legitimately fires hundreds of logins a
// minute from one address. A suite that signs up per test therefore trips the
// server's own limiter and fails with 429s that look like flakiness.
//
// That limiter is correct and must not be turned off to make tests pass. So
// the suite is shaped to fit inside it instead: exactly ONE account is created
// per run, and every test after that starts already signed in by seeding the
// token localStorage would have held. Isolation still holds where it matters,
// because each test creates its OWN organisation and works only inside it.
//
// The real sign-up and sign-in journeys are still exercised end to end — once
// each, in journey.spec.ts, which is the right number of times to test them.
// ---------------------------------------------------------------------------

type Account = { email: string; token: string };
let shared: Promise<Account> | null = null;

async function post(path: string, body: unknown, token?: string) {
  const res = await fetch(API + path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`POST ${path} → ${res.status} ${await res.text()}`);
  }
  return res.json();
}

/** The run's single account, created on first use. */
export function sharedAccount(): Promise<Account> {
  shared ??= (async () => {
    const email = freshEmail("suite");
    await post("/v1/auth/signup", { email, name: "Suite Runner", password: PASSWORD });
    const { token } = await post("/v1/auth/login", { email, password: PASSWORD });
    return { email, token };
  })();
  return shared;
}

/**
 * Start the page already signed in.
 *
 * Seeds the exact key the app reads, before any script runs, so the pre-paint
 * theme script and the router's auth guard both see a signed-in session on
 * first evaluation. No auth request is made at all.
 */
export async function asSignedIn(page: Page): Promise<Account> {
  const account = await sharedAccount();
  await page.addInitScript((t) => {
    localStorage.setItem("beacon-token", t);
  }, account.token);
  return account;
}

/**
 * Seed the session ONCE, without an init script.
 *
 * `asSignedIn` uses addInitScript, which Playwright re-runs on every document
 * load for the life of the page — including the load that sign-out triggers.
 * That is right for most tests and exactly wrong for testing sign-out, where
 * it silently puts the token back and the guard then behaves correctly for a
 * session that should not exist.
 *
 * This writes the token once, so signing out actually ends the session.
 */
export async function signInOnce(page: Page): Promise<Account> {
  const account = await sharedAccount();
  await page.goto("/sign-in");
  await page.evaluate((t) => localStorage.setItem("beacon-token", t), account.token);
  return account;
}

/** A fresh org, project and task, created over the API. Returns their ids. */
export async function seedBoard(
  token: string,
  orgName: string,
  projectName = "Board",
  taskTitle = "Seeded task",
) {
  const org = await post("/v1/orgs", { name: orgName }, token);
  const project = await post(`/v1/orgs/${org.id}/projects`, { name: projectName }, token);
  const task = await post(
    `/v1/orgs/${org.id}/projects/${project.id}/tasks`,
    { title: taskTitle, status: "todo", position: 1000 },
    token,
  );
  return { orgID: org.id, projectID: project.id, taskID: task.id };
}

/** Go straight to a board by id — no clicking through the project list. */
export async function gotoBoard(page: Page, projectID: string) {
  await page.goto(`/projects/${projectID}`);
  await expect(page.getByRole("region", { name: "Todo" })).toBeVisible({ timeout: 15_000 });
}

// ---- the full UI journeys, used once each in journey.spec.ts ----------------

export async function signUpThroughTheUI(page: Page, email: string, name = "Ama Mensah") {
  await page.goto("/sign-up");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/welcome/, { timeout: 15_000 });
}

export async function onboardThroughTheUI(
  page: Page,
  org: string,
  project: string,
  task: string,
) {
  await page.getByLabel("Organisation name").fill(org);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByRole("button", { name: /Skip for now|Continue/ }).click();
  await page.getByLabel("Project name").fill(project);
  await page.getByRole("button", { name: "Continue" }).click();
  await page.getByLabel("Task").fill(task);
  await page.getByRole("button", { name: "Open my board" }).click();
  await expect(page).toHaveURL(/localhost:5180\/$/, { timeout: 15_000 });
}
