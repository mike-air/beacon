/**
 * Capture the README screenshots.
 *
 * Not a test, and deliberately not part of the visual suite. That suite exists
 * to FAIL when a pixel moves, so it only shoots screens that render without a
 * server. These shots are the opposite: they need a real Beacon with real rows
 * in it, because a README showing an empty board shows nothing.
 *
 * Run against a seeded instance:
 *
 *   make up && make api          # in ../beacon
 *   npm run dev                  # in web
 *   npx tsx scripts/shoot.ts
 *
 * Output: docs/screenshots/, 2x, which is what Retina and GitHub's own image
 * scaling both want.
 */
import { chromium, type Browser, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";

const BASE = process.env["SHOOT_BASE"] ?? "http://localhost:5180";
const EMAIL = process.env["SHOOT_EMAIL"] ?? "avery@northwind.studio";
const PASSWORD = process.env["SHOOT_PASSWORD"] ?? "demo-password-8417";
const OUT = "../docs/screenshots";
const THEMES = ["dark", "light"] as const;

const WIDTH = 1440;
/** The sign-in panel is full-bleed, so it wants a normal 16:10 frame. */
const TALL = 900;
/** Breathing room below the board's last card. */
const PAD = 64;

/**
 * `networkidle` is unusable past sign-in: the app holds an SSE stream open for
 * realtime, so the network is never idle and the wait always times out. The
 * signed-out screens can still use it — there is no stream before sign-in,
 * which is the same reason the wordmark's beam does not sweep there.
 */
async function settle(page: Page, opts: { idle?: boolean } = {}) {
  // Webfonts land after first paint; shooting early captures the fallback face.
  await page.evaluate(() => document.fonts.ready);
  await page.waitForLoadState(opts.idle ? "networkidle" : "domcontentloaded");
  await page.waitForTimeout(600);
}

/**
 * Bottom edge of the real content, so the frame can end just below it. The
 * board is content-height, not viewport-height: shot at 900 it leaves a third
 * of the image empty, which reads as an app with nothing in it.
 */
async function contentBottom(page: Page): Promise<number> {
  return page.evaluate(() => {
    const main = document.querySelector("main") ?? document.body;
    const els = [...main.querySelectorAll("*")].filter((e) => {
      const r = e.getBoundingClientRect();
      return r.height > 0 && r.width > 0;
    });
    const bottom = Math.max(0, ...els.map((e) => e.getBoundingClientRect().bottom));
    return Math.ceil(bottom + window.scrollY);
  });
}

async function applyTheme(page: Page, theme: "dark" | "light") {
  await page.emulateMedia({ colorScheme: theme });
  await page.evaluate((t) => {
    localStorage.setItem("beacon-theme", t);
    document.documentElement.setAttribute("data-theme", t);
  }, theme);
  await page.waitForTimeout(400);
}

function context(browser: Browser, theme: "dark" | "light") {
  return browser.newContext({
    viewport: { width: WIDTH, height: TALL },
    deviceScaleFactor: 2,
    colorScheme: theme,
    reducedMotion: "reduce",
  });
}

async function signedOut(browser: Browser) {
  for (const theme of THEMES) {
    const ctx = await context(browser, theme);
    const page = await ctx.newPage();
    await page.addInitScript((t) => localStorage.setItem("beacon-theme", t), theme);
    await page.goto(`${BASE}/sign-in`);
    await settle(page, { idle: true });
    await page.screenshot({ path: `${OUT}/sign-in-${theme}.png` });
    console.log(`shot sign-in-${theme}`);
    await ctx.close();
  }
}

async function signedIn(browser: Browser) {
  for (const theme of THEMES) {
    const ctx = await context(browser, theme);
    const page = await ctx.newPage();

    await page.goto(`${BASE}/sign-in`);
    await page.getByPlaceholder("you@company.com").fill(EMAIL);
    await page.getByLabel("Password").fill(PASSWORD);
    await page.getByRole("button", { name: /^sign in$/i }).click();
    await page.waitForURL(`${BASE}/`, { timeout: 15_000 });
    await page.getByText("Projects").first().waitFor();
    await applyTheme(page, theme);
    await settle(page);

    await page.getByText("Platform", { exact: true }).click();
    await settle(page);

    // Crop the frame to the columns rather than shipping empty pixels.
    const h = Math.min((await contentBottom(page)) + PAD, TALL);
    await page.setViewportSize({ width: WIDTH, height: h });
    await page.waitForTimeout(400);
    await page.screenshot({ path: `${OUT}/board-${theme}.png` });
    console.log(`shot board-${theme} (${h}px)`);

    await page.keyboard.press("Meta+k");
    await page.waitForTimeout(300);
    await page.keyboard.type("index", { delay: 40 });
    await page.waitForTimeout(1000);
    await page.screenshot({ path: `${OUT}/search-${theme}.png` });
    console.log(`shot search-${theme}`);

    await ctx.close();
  }
}

async function main() {
  mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();
  await signedOut(browser);
  await signedIn(browser);
  await browser.close();
  console.log(`\nwrote ${OUT}/`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
