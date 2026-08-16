/**
 * Where a card lands.
 *
 * `position` is a float so a card can be inserted between two others without
 * rewriting the rest of the column. The whole scheme rests on one fact: there
 * is always a number between any two distinct floats — until there is not.
 *
 * Halving the gap every time means about 50 insertions into the SAME slot
 * before two positions become indistinguishable in float64. That is far more
 * than a person does by hand and far less than a script does, so
 * `needsRebalance` exists to say when the column has to be renumbered. Beacon
 * has no bulk-reorder endpoint yet (see DEVIATIONS.md), so today that is a
 * warning rather than a repair.
 */

/** The gap between freshly-appended cards. Wide, so there is room to insert. */
export const GAP = 1000;

/** Below this, two adjacent positions are close enough to worry about. */
const MIN_GAP = 1e-6;

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
    if (sorted[i]! - sorted[i - 1]! < MIN_GAP) return true;
  }
  return false;
}
