import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  server: { port: 5180 },
  build: {
    // Split the vendor code by how often it changes. React and the router are
    // stable across releases, so a one-line app change should not invalidate
    // 150KB of somebody's cache.
    rollupOptions: {
      output: {
        manualChunks: {
          react: ["react", "react-dom"],
          router: ["@tanstack/react-router"],
          query: ["@tanstack/react-query"],
          forms: ["react-hook-form", "@hookform/resolvers", "zod"],
        },
      },
    },
    // Every remaining chunk should be small enough that a slow connection is
    // not waiting on one file.
    chunkSizeWarningLimit: 250,
  },
});
