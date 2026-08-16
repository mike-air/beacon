import { expect, test } from "@playwright/test";
import { API, asSignedIn, gotoBoard, requireBeacon, seedBoard } from "./helpers";

/**
 * The client/server seam. These assert things a mocked backend cannot check,
 * and that every real bug in this project has lived in.
 */
test.beforeAll(requireBeacon);

test("mutations carry a unique Idempotency-Key that CORS lets through", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, "Idempotent");
  await gotoBoard(page, projectID);

  const keys: string[] = [];
  const blocked: string[] = [];
  page.on("request", (r) => {
    const k = r.headers()["idempotency-key"];
    if (k) keys.push(k);
  });
  page.on("requestfailed", (r) => blocked.push(`${r.method()} ${r.url()}`));

  // Two separate intents: two separate keys.
  await page.getByRole("button", { name: "Add task" }).first().click();
  await page.getByLabel(/New task in/).fill("First");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByText("First")).toBeVisible();

  await page.getByLabel(/New task in/).fill("Second");
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByText("Second")).toBeVisible();

  expect(keys.length).toBeGreaterThanOrEqual(2);
  expect(new Set(keys).size, "a key identifies one intent, never two").toBe(keys.length);
  // A CORS-blocked preflight surfaces as a failed request. This is the exact
  // bug that made idempotency unreachable from any browser.
  expect(blocked, blocked.join("\n")).toEqual([]);
});

test("every list endpoint answers the same envelope", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { orgID, projectID, taskID } = await seedBoard(token, "Envelopes");
  await page.goto("/");

  const auth = { Authorization: `Bearer ${token}` };
  const paths = [
    `/v1/orgs`,
    `/v1/orgs/${orgID}/members`,
    `/v1/orgs/${orgID}/webhooks`,
    `/v1/orgs/${orgID}/projects`,
    `/v1/orgs/${orgID}/projects/${projectID}/tasks`,
    `/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskID}/comments`,
  ];
  for (const p of paths) {
    const body = await (await fetch(API + p, { headers: auth })).json();
    expect(Object.keys(body).sort(), `${p} envelope`).toEqual(
      expect.arrayContaining(["items", "limit", "offset"]),
    );
  }
});

test("a realtime event from another client reaches the board", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { orgID, projectID } = await seedBoard(token, "Realtime", "Board", "First task");
  await gotoBoard(page, projectID);
  await expect(page.getByText("First task")).toBeVisible();

  // Written by "somebody else". Nobody clicks refresh.
  await fetch(`${API}/v1/orgs/${orgID}/projects/${projectID}/tasks`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ title: "Arrived over SSE", status: "in_progress", position: 2000 }),
  });

  await expect(page.getByText("Arrived over SSE")).toBeVisible({ timeout: 20_000 });
});

test("search reports which engine answered", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, "Searchers", "Board", "Findable launch plan");
  await gotoBoard(page, projectID);

  await page.keyboard.press("Meta+k");
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Search").fill("launch");

  // Either engine is acceptable. Silently hiding which one is not.
  await expect(dialog.getByText(/meilisearch|postgres fallback/)).toBeVisible({ timeout: 20_000 });
});
