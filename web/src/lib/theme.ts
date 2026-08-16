/** Three states, not a boolean: light, dark, and system — where system is the
 *  absence of the stored key. (production-frontend ch11) */
export type ThemePreference = "light" | "dark" | "system";

const KEY = "beacon-theme";
const media = window.matchMedia("(prefers-color-scheme: dark)");

export function getPreference(): ThemePreference {
  const v = localStorage.getItem(KEY);
  return v === "light" || v === "dark" ? v : "system";
}

export function resolvedTheme(pref: ThemePreference = getPreference()): "light" | "dark" {
  if (pref === "system") return media.matches ? "dark" : "light";
  return pref;
}

export function setPreference(pref: ThemePreference) {
  if (pref === "system") localStorage.removeItem(KEY);
  else localStorage.setItem(KEY, pref);
  apply();
}

export function apply() {
  document.documentElement.dataset.theme = resolvedTheme();
}

/** Follow OS changes while in "system", and other tabs' switches. */
export function watchTheme() {
  media.addEventListener("change", () => {
    if (getPreference() === "system") apply();
  });
  window.addEventListener("storage", (e) => {
    if (e.key === KEY) apply();
  });
}
