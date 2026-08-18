import { expect, test } from "@playwright/test";
import {
  asSignedIn,
  freshEmail,
  gotoBoard,
  onboardThroughTheUI,
  PASSWORD,
  requireBeacon,
  seedBoard,
  signInOnce,
  signUpThroughTheUI,
} from "./helpers";

test.beforeAll(requireBeacon);

/**
 * The one test that walks the real front door: sign-up form, auto sign-in,
 * four onboarding steps, first board. Every other test seeds its session
 * instead, because re-testing auth eight times only proves it once and costs
 * eight requests against a limiter that allows five a minute.
 */
test("a new account reaches a board with work on it", async ({ page }) => {
  await signUpThroughTheUI(page, freshEmail("journey"));
  await onboardThroughTheUI(page, "Acme Industries", "Website relaunch", "Draft the launch plan");

  await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
  await page.getByRole("main").getByRole("link").first().click();

  await expect(page.getByRole("heading", { name: "Website relaunch" })).toBeVisible();
  await expect(page.getByRole("button", { name: 'Open "Draft the launch plan"' })).toBeVisible();
  await expect(page.getByRole("region", { name: "Todo" })).toBeVisible();
});

test("a task is a link somebody can send", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, "Linkable", "Board", "Shareable task");
  await gotoBoard(page, projectID);

  await page.getByRole("button", { name: 'Open "Shareable task"' }).click();
  await expect(page).toHaveURL(/\?task=/);
  const url = page.url();

  // Reload cold: the dialog must open from the URL alone.
  await page.goto(url);
  await expect(page.getByRole("dialog")).toContainText("Shareable task");
});

test("moving a card persists across a reload", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, "Movers", "Board", "Move me");
  await gotoBoard(page, projectID);

  await page.getByRole("button", { name: 'Move "Move me"' }).click();
  await page.getByRole("menuitem", { name: "Done" }).click();
  await expect(page.getByRole("region", { name: "Done" }).getByText("Move me")).toBeVisible();

  await page.reload();
  await expect(page.getByRole("region", { name: "Done" }).getByText("Move me")).toBeVisible();
});

test("signing out clears the session and the guard holds afterwards", async ({ page }) => {
  // signInOnce, not asSignedIn: an init script would re-seed the token on the
  // very load that sign-out performs, and the test would prove nothing.
  const { token } = await signInOnce(page);
  const { projectID } = await seedBoard(token, "Leavers");
  await gotoBoard(page, projectID);

  await page.getByRole("button", { name: "Account" }).click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/sign-in/, { timeout: 10_000 });

  // The guard must hold on a direct navigation, not only on the link.
  await page.goto("/settings");
  await expect(page).toHaveURL(/\/sign-in/);
});

test("a wrong password is reported, not swallowed", async ({ page }) => {
  await page.goto("/sign-in");
  await page.getByLabel("Email").fill(freshEmail("nobody"));
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  const alert = page.getByRole("alert");
  await expect(alert).toBeVisible({ timeout: 10_000 });
  await expect(page).toHaveURL(/\/sign-in/);

  // What it SAYS, not merely that it appeared.
  //
  // This assertion is the whole reason the test is worth having. Until the
  // unwrap() fix in @beacon/sdk, every failed request in the app surfaced as
  // "Cannot read properties of undefined (reading 'status')" — the client
  // crashed while rewrapping an error that was already complete. This test
  // passed throughout, because an alert was visible and it never looked
  // inside it. A message check is cheap; a message check that only asserts
  // presence is a test that cannot fail for the reason it exists.
  await expect(alert).toContainText(/invalid email or password/i);
  await expect(alert).not.toContainText(/undefined|cannot read/i);
});

test("a bad CSV import reports every bad row and writes nothing", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, `CSV bad ${Date.now()}`);
  await gotoBoard(page, projectID);

  const before = await page.getByRole("button", { name: /^Open "/ }).count();

  await page.getByRole("button", { name: "Import", exact: true }).click();
  await page
    .getByLabel("CSV")
    .fill("title,status\nFine row,todo\nBroken row,nonsense\n");
  await page.getByRole("dialog").getByRole("button", { name: "Import" }).click();

  // The failure names the line and the column, because "import failed" would
  // leave the user opening their spreadsheet to guess which of 200 rows.
  const alert = page.getByRole("alert");
  await expect(alert).toBeVisible({ timeout: 10_000 });
  await expect(alert).toContainText(/nothing was written/i);
  await expect(alert).toContainText(/line 3/);
  await expect(alert).toContainText(/status/);

  // All-or-nothing is the promise; this is the half a unit test cannot make.
  // The valid row on line 2 must NOT have landed.
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("button", { name: /^Open "/ })).toHaveCount(before);
  await expect(page.getByRole("button", { name: 'Open "Fine row"' })).toHaveCount(0);
});

test("a good CSV import puts every row on the board", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, `CSV good ${Date.now()}`);
  await gotoBoard(page, projectID);

  await page.getByRole("button", { name: "Import", exact: true }).click();
  await page
    .getByLabel("CSV")
    .fill("title,status\nImported todo,todo\nImported doing,in_progress\n");
  await page.getByRole("dialog").getByRole("button", { name: "Import" }).click();

  // Each row lands in the column its status names, not all in the first one.
  await expect(page.getByRole("button", { name: 'Open "Imported todo"' })).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.getByRole("button", { name: 'Open "Imported doing"' })).toBeVisible();
  await expect(page.getByRole("region", { name: "In progress" })).toContainText("Imported doing");
});
