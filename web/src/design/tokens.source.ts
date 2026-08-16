/**
 * The single source of truth for every color in Beacon.
 *
 * Both themes are REQUIRED on every role — a role that answers only one theme
 * is a compile error here, in the file you are already editing, rather than a
 * transparent background discovered in production. (production-frontend ch11-12)
 *
 * The palette is the VOLT system: the user's five poster colors mapped onto
 * semantic roles. Teal and orange sit exactly on status hues, so they ARE the
 * status system; volt yellow is reserved for "alive" and never means anything
 * else; purple owns "interactive"; red was added because destructive actions
 * need a color the palette did not have.
 */

type Hex = `#${string}`;

export interface Role {
  light: Hex;
  dark: Hex;
  /** Why this role exists — emitted into the styleguide. */
  why: string;
}

export const tokens = {
  // ---- surfaces -----------------------------------------------------------
  "bg-sunken": {
    light: "#F5F5F3",
    dark: "#09090B",
    why: "What the page sits in — board wells, code blocks.",
  },
  "bg-page": {
    light: "#FFFFFF",
    dark: "#0E0E11",
    why: "The page itself. Dark is near-black, never pure black (OLED smear).",
  },
  "bg-raised": {
    light: "#FFFFFF",
    dark: "#15151A",
    why: "Cards. Light distinguishes by border+shadow, dark by lightness.",
  },
  "bg-overlay": {
    light: "#FFFFFF",
    dark: "#1C1C22",
    why: "Dialogs, popovers, menus — the highest surface.",
  },
  "bg-well": {
    light: "#F0F0EE",
    dark: "#202027",
    why: "Sunken interactive zones: org pill, count chips, hover fills.",
  },

  // ---- ink ----------------------------------------------------------------
  ink: {
    light: "#16161A",
    dark: "#F1F1F3",
    why: "Primary text. Not pure black/white — both bloom.",
  },
  "ink-muted": {
    light: "#5F5F6B",
    dark: "#9C9CA8",
    why: "Secondary text: descriptions, timestamps, labels.",
  },
  "ink-faint": {
    light: "#8C8C99",
    dark: "#66666F",
    why: "Tertiary: placeholders, disabled labels.",
  },
  "ink-inverse": {
    light: "#FFFFFF",
    dark: "#0E0E11",
    why: "Text on a surface of the OPPOSITE theme — e.g. the dark tooltip in light mode. NOT for accent fills; see on-accent.",
  },
  "on-accent": {
    light: "#FFFFFF",
    dark: "#FFFFFF",
    why: "Text/icons ON an accent or danger fill. White in BOTH themes: accent stays a mid-dark purple in both, so ink-inverse would flip to near-black in dark and drop to 3.2:1.",
  },

  // ---- lines --------------------------------------------------------------
  line: {
    light: "#E8E8EB",
    dark: "#26262D",
    why: "Default hairline. 1px, everywhere, quiet.",
  },
  "line-strong": {
    light: "#D6D6DB",
    dark: "#33333B",
    why: "Inputs at rest, table header rules.",
  },

  // ---- accent (poster purple #92278F) -------------------------------------
  accent: {
    light: "#92278F",
    dark: "#A62CA3",
    why: "The brand interactive color. Dark lifts one step so white labels keep 4.5:1.",
  },
  "accent-hover": {
    light: "#7E2280",
    dark: "#B93AB6",
    why: "Hover: darker in light, lighter in dark — toward the page, never away.",
  },
  "accent-active": {
    light: "#6E1C70",
    dark: "#C94FC4",
    why: "Pressed.",
  },
  "accent-text": {
    light: "#8A2487",
    dark: "#DE8BDB",
    why: "Purple as text/links — lighter and calmer in dark so it doesn't vibrate.",
  },
  "accent-subtle": {
    light: "#F6EAF6",
    dark: "#2A1529",
    why: "Purple as background wash: selected rows, active nav, badges.",
  },

  // ---- volt (poster #E3FF00) — the signature ------------------------------
  volt: {
    light: "#C6DF00",
    dark: "#E3FF00",
    why: "ALIVE. Live presence, realtime pulses, the logo beam. Never semantic. Full power only on dark; light deepens it to survive white.",
  },
  "on-volt": {
    light: "#1F2400",
    dark: "#1F2400",
    why: "Text/icons ON a volt fill. Dark in BOTH themes — volt is a light color either way, so ink-inverse would be white-on-yellow and unreadable.",
  },
  "volt-text": {
    light: "#6B7A00",
    dark: "#E3FF00",
    why: "Volt as text — light needs a much deeper step to be readable.",
  },
  "volt-subtle": {
    light: "#F6FADC",
    dark: "#23260A",
    why: "Volt wash for realtime-updated rows, fading after a beat.",
  },

  // ---- status (poster teal + orange, plus the missing red) ----------------
  success: {
    light: "#01A79E",
    dark: "#01A79E",
    why: "Poster teal IS success. Done states, healthy, confirmations.",
  },
  "success-text": {
    light: "#067C76",
    dark: "#2BD9CE",
    why: "Teal as text.",
  },
  "success-subtle": {
    light: "#E3F6F4",
    dark: "#0D2523",
    why: "Teal wash for done badges.",
  },
  warning: {
    light: "#FF6A00",
    dark: "#FF6A00",
    why: "Poster orange IS warning: rate limits, expiring sessions, unsaved.",
  },
  "warning-text": {
    light: "#B34A00",
    dark: "#FF9A52",
    why: "Orange as text.",
  },
  "warning-subtle": {
    light: "#FFF0E4",
    dark: "#2B1707",
    why: "Orange wash.",
  },
  danger: {
    light: "#DC2626",
    dark: "#CE2C31",
    why: "Destructive only — the color the poster was missing; nothing else may use red. Dark is NOT lighter than light here: a solid red button carries white text, and lightening the red to suit the dark page drops that label to 3.9:1. Use danger-text when red needs to read as text on the page.",
  },
  "danger-text": {
    light: "#B91C1C",
    dark: "#F87171",
    why: "Red as text: error messages, destructive menu items.",
  },
  "danger-subtle": {
    light: "#FDECEC",
    dark: "#2A1214",
    why: "Red wash: invalid inputs, error banners.",
  },

  // ---- focus --------------------------------------------------------------
  ring: {
    light: "#92278F",
    dark: "#C94FC4",
    why: "The focus ring. Accent-family in both themes; volt tested badly on white.",
  },
} as const satisfies Record<string, Role>;

export type TokenName = keyof typeof tokens;
