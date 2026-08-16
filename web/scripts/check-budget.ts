/**
 * The performance budget, enforced at build time.
 *
 * A budget that lives in a document is a budget nobody checks. This one fails
 * the build, which is the only version that survives a deadline.
 *
 * The number that matters is not "total bytes shipped" — it is what a visitor
 * must download before the first screen renders. Lazily-loaded screens are
 * counted separately, because they arrive after paint and while the user is
 * already reading something.
 */
import { readdirSync, statSync } from "node:fs";
import { gzipSync } from "node:zlib";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const DIST = "dist/assets";

/** Gzipped KB. Servers compress; measuring raw bytes measures a fiction. */
const BUDGETS = {
  /** JS needed before anything paints: entry + its static imports. */
  entryJs: 200,
  /** The stylesheet — one file, loaded render-blocking. */
  css: 30,
  /** Any single lazily-loaded screen. */
  lazyChunk: 60,
};

const kb = (b: number) => Math.round((b / 1024) * 10) / 10;
const gz = (p: string) => gzipSync(readFileSync(p)).length;

const files = readdirSync(DIST).filter((f) => !f.endsWith(".map"));
const js = files.filter((f) => f.endsWith(".js"));
const css = files.filter((f) => f.endsWith(".css"));

// Vite names the entry "index-<hash>.js"; vendor chunks get their manualChunks
// key. Everything else is a route chunk, which arrives after first paint.
const VENDOR = ["react", "router", "query", "forms"];
const isEntry = (f: string) => f.startsWith("index-");
const isVendor = (f: string) => VENDOR.some((v) => f.startsWith(`${v}-`));

let entryBytes = 0;
const lazy: [string, number][] = [];

for (const f of js) {
  const size = gz(join(DIST, f));
  if (isEntry(f) || isVendor(f)) entryBytes += size;
  else lazy.push([f, size]);
}

const cssBytes = css.reduce((n, f) => n + gz(join(DIST, f)), 0);
const rawTotal = files.reduce((n, f) => n + statSync(join(DIST, f)).size, 0);

const failures: string[] = [];
if (kb(entryBytes) > BUDGETS.entryJs)
  failures.push(`entry JS ${kb(entryBytes)}KB > ${BUDGETS.entryJs}KB budget`);
if (kb(cssBytes) > BUDGETS.css)
  failures.push(`CSS ${kb(cssBytes)}KB > ${BUDGETS.css}KB budget`);
for (const [f, size] of lazy) {
  if (kb(size) > BUDGETS.lazyChunk)
    failures.push(`${f} is ${kb(size)}KB > ${BUDGETS.lazyChunk}KB per-screen budget`);
}

console.log("bundle (gzipped)");
console.log(`  entry + vendor : ${kb(entryBytes)} KB   budget ${BUDGETS.entryJs} KB`);
console.log(`  css            : ${kb(cssBytes)} KB   budget ${BUDGETS.css} KB`);
console.log(`  lazy screens   : ${lazy.length} chunks, largest ${kb(Math.max(0, ...lazy.map((l) => l[1])))} KB`);
console.log(`  raw total      : ${kb(rawTotal)} KB uncompressed`);

if (failures.length > 0) {
  console.error("\nperformance budget exceeded:");
  for (const f of failures) console.error("  " + f);
  console.error("\nSplit the screen, drop the dependency, or raise the budget deliberately.");
  process.exit(1);
}
console.log("\nbudget OK");
