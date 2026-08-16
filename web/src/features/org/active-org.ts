/**
 * Which org the user is looking at.
 *
 * Kept in localStorage rather than only in the URL so that opening Beacon
 * fresh lands where they left off. The URL still wins when it names one — a
 * shared link must open the org in the link, not the reader's last one.
 */
const KEY = "beacon-active-org";

export function getActiveOrg(): string | null {
  return localStorage.getItem(KEY);
}

export function setActiveOrg(orgID: string) {
  localStorage.setItem(KEY, orgID);
}

export function clearActiveOrg() {
  localStorage.removeItem(KEY);
}
