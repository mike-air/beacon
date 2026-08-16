/**
 * The guard that would have caught white-on-volt.
 *
 * Every text role is checked against every surface it is allowed to sit on,
 * in BOTH themes, and the generator refuses to write files if any pair drops
 * below its threshold. A contrast bug is a build failure, not a screenshot
 * somebody notices later. (production-frontend ch10)
 */
import { tokens } from "../src/design/tokens.source";

type Theme = "light" | "dark";

function srgbToLinear(c: number): number {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

function luminance(hex: string): number {
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return 0.2126 * srgbToLinear(r) + 0.7152 * srgbToLinear(g) + 0.0722 * srgbToLinear(b);
}

export function ratio(a: string, b: string): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

type Name = keyof typeof tokens;

/** [foreground, background, minimum, why it is that minimum] */
const PAIRS: [Name, Name, number, string][] = [
  ["ink", "bg-page", 7, "body text — AAA, it is read all day"],
  ["ink", "bg-raised", 7, "body text on cards"],
  ["ink", "bg-overlay", 7, "body text in dialogs"],
  ["ink-muted", "bg-page", 4.5, "secondary text — AA"],
  ["ink-muted", "bg-raised", 4.5, "secondary text on cards"],
  ["ink-muted", "bg-well", 4.5, "labels inside wells"],
  ["ink-faint", "bg-page", 3, "placeholders — AA large / non-essential"],
  ["on-accent", "accent", 4.5, "button labels on accent"],
  ["on-accent", "accent-hover", 4.5, "button labels while hovered"],
  ["on-accent", "danger", 4.5, "button labels on danger"],
  ["ink-inverse", "ink", 7, "the inverted tooltip"],
  ["on-volt", "volt", 4.5, "labels on a volt fill — the one that broke"],
  ["accent-text", "bg-page", 4.5, "links"],
  ["accent-text", "accent-subtle", 4.5, "badge text on its own wash"],
  ["success-text", "bg-page", 4.5, "success text"],
  ["success-text", "success-subtle", 4.5, "success badge"],
  ["warning-text", "bg-page", 4.5, "warning text"],
  ["warning-text", "warning-subtle", 4.5, "warning badge"],
  ["danger-text", "bg-page", 4.5, "error messages"],
  ["danger-text", "danger-subtle", 4.5, "error banner"],
  ["volt-text", "bg-page", 4.5, "volt as text"],
  ["ring", "bg-page", 3, "focus ring against the page — AA non-text"],
  ["line-strong", "bg-page", 1.4, "input borders must be visible at all"],
];

export function checkContrast(): string[] {
  const failures: string[] = [];
  for (const theme of ["light", "dark"] as Theme[]) {
    for (const [fg, bg, min, why] of PAIRS) {
      const r = ratio(tokens[fg][theme], tokens[bg][theme]);
      if (r < min) {
        failures.push(
          `${theme}: ${fg} on ${bg} is ${r.toFixed(2)}:1, needs ${min}:1 — ${why}` +
            ` (${tokens[fg][theme]} on ${tokens[bg][theme]})`,
        );
      }
    }
  }
  return failures;
}

export const pairCount = PAIRS.length * 2;
