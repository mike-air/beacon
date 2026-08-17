import { expect, test, type Page } from "@playwright/test";

/**
 * The styleguide renders every primitive in the design system on one page and
 * needs neither a server nor a session, which makes it the one screen in this
 * app that is genuinely deterministic. That is what makes it worth
 * screenshotting: a diff here means a component or a token changed, never that
 * a fixture drifted.
 *
 * Both themes, because a token is only correct if it answers both. Several of
 * this project's colour bugs were visible in one theme only.
 *
 * WHAT THIS DOES NOT COVER, measured rather than assumed. The suite was
 * tested by breaking things on purpose:
 *
 *   widening Button's md padding   caught, both styleguide shots
 *   changing the bg-page token     never reached the diff — the contrast
 *                                  guard in gen-tokens failed the build
 *                                  first, which is the better outcome
 *   changing the bg-sunken token   NOT caught. Nothing on these two pages
 *                                  sits on that surface; it is the board well.
 *
 * So this covers the primitives and the two screens that render without a
 * server. The board — where bg-sunken and every drag state live — needs a
 * signed-in session and seeded data, which belongs next to a real Beacon in
 * the e2e suite and is not written yet. A green run here is not "the UI did
 * not change".
 */

async function open(page: Page, path: string, theme: "light" | "dark") {
  await page.addInitScript((t) => {
    localStorage.setItem("beacon-theme", t);
  }, theme);
  await page.goto(path);
  // Webfonts load after first paint; screenshotting before they land captures
  // the fallback face, and then every later run "fails".
  await page.evaluate(() => document.fonts.ready);
  await page.waitForLoadState("networkidle");
}

for (const theme of ["light", "dark"] as const) {
  test(`styleguide, ${theme}`, async ({ page }) => {
    await open(page, "/styleguide", theme);
    await expect(page.getByRole("heading").first()).toBeVisible();
    await expect(page).toHaveScreenshot(`styleguide-${theme}.png`, { fullPage: true });
  });

  test(`sign-in, ${theme}`, async ({ page }) => {
    await open(page, "/sign-in", theme);
    await expect(page.getByRole("button", { name: /sign in/i })).toBeVisible();
    await expect(page).toHaveScreenshot(`sign-in-${theme}.png`, { fullPage: true });
  });
}
