import { describe, expect, it } from "vitest";
import { checkContrast, pairCount, ratio } from "./contrast";

describe("contrast ratio", () => {
  it("computes the WCAG extremes correctly", () => {
    expect(ratio("#FFFFFF", "#000000")).toBeCloseTo(21, 1);
    expect(ratio("#FFFFFF", "#FFFFFF")).toBeCloseTo(1, 5);
  });

  /**
   * The three real bugs this guard caught on its first three runs. If any of
   * them stops failing, the guard has stopped working.
   */
  it("fails white on volt — the bug that shipped first", () => {
    expect(ratio("#FFFFFF", "#E3FF00")).toBeLessThan(4.5);
  });
  it("fails a near-black label on dark-theme accent", () => {
    expect(ratio("#0E0E11", "#A62CA3")).toBeLessThan(4.5);
  });
  it("fails white on a red lightened for dark mode", () => {
    expect(ratio("#FFFFFF", "#E5484D")).toBeLessThan(4.5);
  });
});

describe("the shipped palette", () => {
  it("passes every text/surface pair in both themes", () => {
    const failures = checkContrast();
    expect(failures, failures.join("\n")).toEqual([]);
  });
  it("checks a meaningful number of pairs", () => {
    expect(pairCount).toBeGreaterThanOrEqual(40);
  });
});
