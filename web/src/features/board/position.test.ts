import { describe, expect, it } from "vitest";
import { GAP, needsRebalance, positionAt, positionAtEnd, positionAtStart } from "./position";

/** One ULP at x: the distance to the next representable double. */
function ulp(x: number): number {
  const n = Math.abs(x);
  if (n === 0) return Number.MIN_VALUE;
  const b = new DataView(new ArrayBuffer(8));
  b.setFloat64(0, n);
  let hi = b.getUint32(0);
  let lo = b.getUint32(4);
  if (lo === 0xffffffff) { hi += 1; lo = 0; } else { lo += 1; }
  b.setUint32(0, hi);
  b.setUint32(4, lo);
  return b.getFloat64(0) - n;
}

describe("positionAtEnd", () => {
  it("returns GAP for an empty column", () => {
    expect(positionAtEnd([])).toBe(GAP);
  });
  it("appends a full gap past the last card", () => {
    expect(positionAtEnd([1000, 2000])).toBe(3000);
  });
});

describe("positionAtStart", () => {
  it("returns GAP for an empty column", () => {
    expect(positionAtStart([])).toBe(GAP);
  });
  it("halves toward zero rather than going negative", () => {
    expect(positionAtStart([1000])).toBe(500);
    expect(positionAtStart([1])).toBeGreaterThan(0);
  });
});

describe("positionAt", () => {
  it("lands strictly between its neighbours", () => {
    const p = positionAt([1000, 2000], 1);
    expect(p).toBeGreaterThan(1000);
    expect(p).toBeLessThan(2000);
  });
  it("clamps an index past the end to an append", () => {
    expect(positionAt([1000], 99)).toBe(2000);
  });
  it("clamps a negative index to a prepend", () => {
    expect(positionAt([1000], -5)).toBe(500);
  });
  it("keeps a column strictly increasing after an insert at every slot", () => {
    const column = [1000, 2000, 3000];
    for (let slot = 0; slot <= column.length; slot++) {
      const p = positionAt(column, slot);
      const next = [...column, p].sort((a, b) => a - b);
      for (let i = 1; i < next.length; i++) {
        expect(next[i]!).toBeGreaterThan(next[i - 1]!);
      }
    }
    expect(column).toHaveLength(3); // positionAt must not mutate
  });
});

describe("needsRebalance", () => {
  it("is quiet on a healthy column", () => {
    expect(needsRebalance([1000, 2000, 3000, 4000])).toBe(false);
  });
  it("is quiet on a column with one card, or none", () => {
    expect(needsRebalance([])).toBe(false);
    expect(needsRebalance([1000])).toBe(false);
  });
  it("fires when two positions have actually collided", () => {
    expect(needsRebalance([1000, 1000])).toBe(true);
  });

  /**
   * The regression this file exists for. The threshold used to be an absolute
   * 1e-6, which is 8.8e15 ULPs of headroom near position 1e3 and SMALLER than
   * one ULP past 1e12 — so it cried wolf on small boards and was mathematically
   * dead on large ones. A relative threshold must behave the same at any scale.
   */
  it.each([1e3, 1e6, 1e9, 1e12])(
    "warns with usable float headroom left at position %s",
    (base) => {
      const lo = base;
      let hi = base + GAP;
      let inserts = 0;
      while (!needsRebalance([lo, hi]) && inserts < 5000) {
        hi = lo + (hi - lo) / 2;
        inserts += 1;
      }
      expect(inserts).toBeLessThan(5000); // it must fire eventually
      const headroom = (hi - lo) / ulp(hi);
      // Never so late that a collision is imminent...
      expect(headroom).toBeGreaterThan(100);
      // ...and never so early that a real board gets a false alarm.
      expect(headroom).toBeLessThan(1e6);
    },
  );

  /**
   * The header comment in position.ts quotes exact insert counts. A comment
   * with numbers in it rots silently, so the numbers are pinned here: if the
   * threshold or GAP changes, this fails and the comment gets updated with it.
   */
  it.each([
    { base: 1e3, warnsAfter: 40, collidesAfter: 53 },
    { base: 1e6, warnsAfter: 31, collidesAfter: 43 },
    { base: 1e9, warnsAfter: 21, collidesAfter: 33 },
    { base: 1e12, warnsAfter: 11, collidesAfter: 23 },
  ])("warns after $warnsAfter and collides after $collidesAfter at $base", (c) => {
    const lo = c.base;
    let hi = c.base + GAP;
    let warned = 0;
    while (!needsRebalance([lo, hi])) {
      hi = lo + (hi - lo) / 2;
      warned += 1;
    }
    expect(warned).toBe(c.warnsAfter);

    // Keep halving past the warning to find where float64 actually gives up.
    const l = c.base;
    let h = c.base + GAP;
    let collided = 0;
    for (;;) {
      const mid = l + (h - l) / 2;
      if (mid <= l || mid >= h) break;
      h = mid;
      collided += 1;
    }
    expect(collided).toBe(c.collidesAfter);
  });

  /** Prepending is the operation that really is unreachable. */
  it("survives 1,084 prepends before halving toward zero bottoms out", () => {
    let column = [GAP];
    let n = 0;
    for (;;) {
      const next = positionAtStart(column);
      if (next === 0 || next === column[0]) break;
      column = [next];
      n += 1;
    }
    expect(n).toBe(1084);
  });

  it("does not fire on positions produced by ordinary use", () => {
    // 200 appends, then an insert between each neighbouring pair.
    const column = Array.from({ length: 200 }, (_, i) => (i + 1) * GAP);
    for (let i = 1; i < column.length; i += 2) {
      column.push(positionAt(column, i));
      column.sort((a, b) => a - b);
    }
    expect(needsRebalance(column)).toBe(false);
  });
});
