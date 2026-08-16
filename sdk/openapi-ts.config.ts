import { defineConfig } from "@hey-api/openapi-ts";

/**
 * The SDK is generated from the document the Go handlers emit.
 *
 * `input` deliberately points at ../api/openapi.json rather than at a running
 * server. Generating from a live process means the SDK depends on somebody
 * having started one, and on that one being current — the committed file is
 * the artefact CI can diff, which is what makes drift a red build.
 *
 * The output is COMMITTED. A consumer of this package should not need a Go
 * toolchain to typecheck their own code, and a reviewer should be able to see
 * a contract change as a diff in a pull request rather than infer it.
 */
export default defineConfig({
  input: "../api/openapi.json",
  output: {
    path: "src/generated",
    // Formatted so a contract change reads as a reviewable diff rather than
    // a single reflowed line.
    postProcess: ["prettier"],
  },
  plugins: [
    // Named methods from operationId, with JSDoc taken from each operation's
    // summary and description. `getMe()` and `listTasks()` rather than a
    // stringly-typed path — which is the difference between a client and an
    // SDK.
    { name: "@hey-api/sdk", operations: { strategy: "single" } },
    { name: "@hey-api/typescript", enums: "javascript" },
    // fetch, because this runs in a browser and adding an HTTP library to do
    // what the platform already does is weight with no payload.
    { name: "@hey-api/client-fetch" },
  ],
});
