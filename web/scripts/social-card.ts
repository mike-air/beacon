/**
 * Build the GitHub social preview card — docs/screenshots/social-card.png.
 *
 * This is the image that appears when the repo link is pasted into Slack, a
 * tweet, or a DM. GitHub's pinned-repo cards do NOT render it: those show name,
 * description, language and stars only. It is the shared-link surface, set at
 * Settings -> Social preview, and GitHub wants 1280x640 under 1MB.
 *
 * It is composed rather than screenshotted, because the card has a job the app
 * does not: it must survive being scaled to a 400px-wide thumbnail in a chat
 * client. So the type is far larger than any UI type, and the board sits behind
 * it as texture, not as something anyone is expected to read.
 *
 * Colours are read from tokens.source.ts at build time, so the card cannot
 * drift from the app the way a hand-made image in Figma would.
 *
 *   npx tsx scripts/social-card.ts
 */
import { chromium } from "@playwright/test";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { tokens } from "../src/design/tokens.source";

const OUT = "../docs/screenshots";
const W = 1280;
const H = 640;

const c = (role: keyof typeof tokens) => tokens[role].dark;

const b64 = (p: string, mime: string) =>
  `data:${mime};base64,${readFileSync(p).toString("base64")}`;

const archivo = b64(
  "node_modules/@fontsource/archivo-black/files/archivo-black-latin-400-normal.woff2",
  "font/woff2",
);
const grotesk = b64(
  "node_modules/@fontsource/space-grotesk/files/space-grotesk-latin-400-normal.woff2",
  "font/woff2",
);
const groteskMed = b64(
  "node_modules/@fontsource/space-grotesk/files/space-grotesk-latin-500-normal.woff2",
  "font/woff2",
);
const board = b64(`${OUT}/board-dark.webp`, "image/webp");

const html = `<!doctype html>
<meta charset="utf-8">
<style>
  @font-face { font-family: Archivo; src: url(${archivo}) format('woff2'); font-weight: 400; }
  @font-face { font-family: Grotesk; src: url(${grotesk}) format('woff2'); font-weight: 400; }
  @font-face { font-family: Grotesk; src: url(${groteskMed}) format('woff2'); font-weight: 500; }

  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body { width: ${W}px; height: ${H}px; }
  body {
    background: ${c("bg-page")};
    color: ${c("ink")};
    font-family: Grotesk, sans-serif;
    position: relative;
    overflow: hidden;
  }

  /* The beam, echoed. The mark is a lighthouse; the card should feel lit from
     the same place the logo is lit from — so the glow starts AT the mark and
     runs right, rather than sitting under the headline where it turns white
     type olive. Volt only ever means "alive", so it stays a glow, never a fill. */
  .beam {
    position: absolute;
    left: -260px; top: -240px;
    width: 1500px; height: 1120px;
    background: radial-gradient(ellipse at 34% 42%,
      ${c("volt")}14 0%, ${c("volt")}07 22%, ${c("volt")}02 40%, transparent 60%);
  }

  /* A window onto the board, not the whole board. Cropping to the columns lets
     the frame run the full height of the card instead of leaving a third of it
     empty, and the part that survives the crop is the part worth seeing. */
  .card {
    position: absolute;
    right: -84px; top: 64px;
    width: 760px; height: 512px;
    border: 1px solid ${c("line")};
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 44px 90px -24px #000000D9;
  }
  .card img {
    display: block;
    width: 100%; height: 100%;
    object-fit: cover;
    object-position: left top;
    /* Held one stop back. At thumbnail size the headline has to win, and a
       board at full brightness pulls the eye before the words do. */
    filter: brightness(0.78) saturate(0.92);
  }
  /* Fade the board into the ground so it reads as texture, not as a second
     thing competing with the headline. */
  .card::after {
    content: "";
    position: absolute; inset: 0;
    background: linear-gradient(100deg, ${c("bg-page")} 0%, ${c("bg-page")}00 46%);
  }

  .left {
    position: absolute;
    left: 72px; top: 50%;
    transform: translateY(-50%);
    width: 660px;
  }

  .mark { display: flex; align-items: center; gap: 14px; margin-bottom: 40px; }
  .mark span {
    font-family: Archivo;
    font-size: 34px;
    letter-spacing: -0.02em;
  }

  h1 {
    font-family: Archivo;
    font-size: 62px;
    line-height: 1.04;
    letter-spacing: -0.032em;
    text-wrap: balance;
  }
  h1 em { font-style: normal; color: ${c("volt")}; }

  p {
    margin-top: 22px;
    font-size: 21px;
    line-height: 1.5;
    color: ${c("ink-muted")};
    max-width: 30ch;
  }

  .chips { display: flex; gap: 9px; margin-top: 38px; }
  .chip {
    font-size: 14px;
    font-weight: 500;
    letter-spacing: 0.01em;
    color: ${c("ink-muted")};
    background: ${c("bg-well")};
    border: 1px solid ${c("line")};
    border-radius: 999px;
    padding: 8px 15px;
    white-space: nowrap;
  }
</style>

<div class="beam"></div>

<div class="card"><img src="${board}" alt=""></div>

<div class="left">
  <div class="mark">
    <svg width="40" height="40" viewBox="0 0 32 32" fill="none">
      <path d="M9 16 L29 8.5 L29 23.5 Z" fill="${c("volt")}" opacity="0.35"/>
      <path d="M9 16 L24 11.5 L24 20.5 Z" fill="${c("volt")}" opacity="0.75"/>
      <rect x="4" y="6" width="6" height="20" rx="2.6" fill="${c("accent")}"/>
      <circle cx="7" cy="16" r="2" fill="${c("bg-page")}"/>
    </svg>
    <span>BEACON</span>
  </div>

  <h1>Built to be <em>read</em>,<br>not just run.</h1>

  <p>A multi-tenant task board. A Go API, a React client, and a contract
     generated between them.</p>

  <div class="chips">
    <div class="chip">Go + Postgres</div>
    <div class="chip">React + TypeScript</div>
    <div class="chip">Generated SDK</div>
  </div>
</div>
`;

async function main() {
  mkdirSync(OUT, { recursive: true });
  const tmp = "/tmp/beacon-social-card.html";
  writeFileSync(tmp, html);

  const browser = await chromium.launch();
  const page = await browser.newPage({
    viewport: { width: W, height: H },
    deviceScaleFactor: 2,
  });
  await page.goto(`file://${tmp}`);
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(300);
  await page.screenshot({ path: `${OUT}/social-card.png` });
  await browser.close();
  console.log(`wrote ${OUT}/social-card.png`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
