import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": path.resolve(__dirname, "src") } },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    globals: true,
    include: ["src/**/*.test.{ts,tsx}", "scripts/**/*.test.ts"],
    coverage: {
      provider: "v8",
      // The layers where a bug is silent: maths, parsing, error mapping.
      include: ["src/api/**", "src/features/board/position.ts", "src/lib/**", "scripts/contrast.ts"],
      thresholds: { statements: 70, branches: 70, functions: 70, lines: 70 },
    },
  },
});
