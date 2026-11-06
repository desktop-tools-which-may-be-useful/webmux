import { defineConfig } from "vitest/config";

export const sharedCoverage = {
  provider: "v8" as const,
  reportOnFailure: true,
  include: ["src/**/*.{ts,svelte}"],
  exclude: [
    "src/**/*.d.ts",
    "src/main.ts",
    "src/icons/**",
    "src/lib/components/ui/**",
  ],
};

export default defineConfig({
  test: {
    environment: "node",
    include: ["test/*.test.ts"],
    coverage: {
      ...sharedCoverage,
      include: ["src/**/*.ts"],
      exclude: [...sharedCoverage.exclude, "src/**/*.svelte.ts"],
      reportsDirectory: ".vitest/coverage/unit",
      reporter: ["text-summary", "json", "json-summary", "html"],
    },
  },
});
