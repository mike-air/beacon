import { expect, test } from "@playwright/test";
import { asSignedIn, gotoBoard, requireBeacon, seedBoard } from "./helpers";

test.beforeAll(requireBeacon);

test("a card opens and closes from the keyboard alone", async ({ page }) => {
  const { token } = await asSignedIn(page);
  const { projectID } = await seedBoard(token, "Keyboard", "Board", "Keyboard task");
  await gotoBoard(page, projectID);

  await page.getByRole("button", { name: 'Open "Keyboard task"' }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("dialog")).toBeVisible();

  // Escape must close it rather than trap the user inside.
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
});

test("every SVG is either labelled or explicitly decorative", async ({ page }) => {
  await page.goto("/sign-in");
  const unlabelled = await page.evaluate(() =>
    Array.from(document.querySelectorAll("svg"))
      .filter((el) => el.getAttribute("aria-hidden") !== "true")
      .filter((el) => !el.getAttribute("aria-label") && !el.querySelector("title"))
      .map((el) => el.outerHTML.slice(0, 80)),
  );
  expect(unlabelled, unlabelled.join("\n")).toEqual([]);
});

test("form errors are announced, not merely coloured", async ({ page }) => {
  await page.goto("/sign-up");
  await page.getByLabel("Email").fill("not-an-email");
  await page.getByLabel("Password").fill("short");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByLabel("Email")).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByRole("alert").first()).toBeVisible();
});

test("the stored theme is applied before first paint", async ({ page }) => {
  await page.goto("/sign-in");
  await page.evaluate(() => localStorage.setItem("beacon-theme", "dark"));
  await page.reload();
  // Set by the blocking script in index.html, above the stylesheet.
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});
