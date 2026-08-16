/**
 * Where a card lands.
 *
 * `position` is a float so a card can be inserted between two others without
 * rewriting the rest of the column. The whole scheme rests on one fact: there
 * is always a number between any two distinct floats — until there is not.
 *
 * Halving the gap every time means 1,084 insertions into the SAME slot before
 * two positions are genuinely indistinguishable in float64. `needsRebalance`
 * exists to warn well before that. Beacon has no bulk-reorder endpoint yet
 * (see DEVIATIONS.md), so today it is a warning rather than a repair.
 *
 * The threshold is RELATIVE, not absolute, and that matters more than it
 * looks. An absolute floor measures the wrong thing, because the spacing
 * between representable doubles grows with magnitude:
 *
 *   position 1e3   one ULP is 1.1e-13
 *   position 1e9   one ULP is 1.2e-07
 *   position 1e12  one ULP is 1.2e-04
 *
 * A fixed 1e-6 floor is 8.8e15 ULPs of headroom near 1e3 — it would warn
 * about a thousand inserts too early — and SMALLER than one ULP by 1e12,
 * where two positions collide before the check could ever fire. One constant
 * cannot be both. Scaling by the magnitude gives ~7,500 ULPs of headroom at
 * every size, which is the property actually wanted.
 */

/** The gap between freshly-appended cards. Wide, so there is room to insert. */
export const GAP = 1000;

/**
 * Warn when a gap shrinks to this fraction of the position's own magnitude.
 * 2^-40 leaves ~7,500 representable doubles between neighbours at any scale —
 * far too tight to have arrived at by hand, far too loose to be a real
 * collision.
 */
const MIN_RELATIVE_GAP = 2 ** -40;

/** Append: after everything currently in the column. */
export function positionAtEnd(sorted: number[]): number {
  const last = sorted[sorted.length - 1];
  return last === undefined ? GAP : last + GAP;
}

/** Prepend: before everything. Halves toward zero rather than going negative. */
export function positionAtStart(sorted: number[]): number {
  const first = sorted[0];
  return first === undefined ? GAP : first / 2;
}

/**
 * Drop at index `index` of a column whose current positions are `sorted`,
 * with the dragged card already removed from that array.
 */
export function positionAt(sorted: number[], index: number): number {
  if (sorted.length === 0) return GAP;
  if (index <= 0) return positionAtStart(sorted);
  if (index >= sorted.length) return positionAtEnd(sorted);
  const before = sorted[index - 1]!;
  const after = sorted[index]!;
  return before + (after - before) / 2;
}

/** True when the column's gaps have collapsed far enough to need renumbering. */
export function needsRebalance(sorted: number[]): boolean {
  for (let i = 1; i < sorted.length; i++) {
    const lower = sorted[i - 1]!;
    const upper = sorted[i]!;
    // Scale by the larger neighbour: that is where the doubles are sparsest,
    // so it is the side that runs out of room first. The floor of 1 keeps the
    // test meaningful for positions between 0 and 1, which prepending
    // produces by halving toward zero.
    const threshold = Math.max(Math.abs(upper), 1) * MIN_RELATIVE_GAP;
    if (upper - lower < threshold) return true;
  }
  return false;
}
