// Flat config (ESLint 9). `eslint` and `typescript-eslint` were already
// dependencies — package.json has had a `lint` script since scaffolding —
// but this file was never written, so every run failed with "couldn't find
// eslint.config.js" and the root Makefile's `lint` target quietly skipped
// straight to `tsc --noEmit`. Wiring this in is part of finishing CI: a
// pipeline that calls `npm run lint` needs it to actually run.
import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/**", "src/design/tokens.gen.ts", "src/generated/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      // Prefixing with `_` is this codebase's existing convention for an
      // intentionally-unused parameter (see catch blocks in api/sse.ts).
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      // any is not banned outright — SDK/DOM interop needs an escape hatch
      // occasionally — but it must be a decision, not a default.
      "@typescript-eslint/no-explicit-any": "warn",
    },
  },
);
