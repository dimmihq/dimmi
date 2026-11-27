/*
 * Public SDK packages published from sdk/. Keep in sync with docs/src/sdks.ts
 * (docs build and scripts/generate-llms.ts both verify the list matches).
 */
export const SDK_PACKAGES = [
  "browser",
  "react",
  "nextjs",
  "vue",
  "angular",
  "svelte",
  "server",
  "api",
  "go",
  "php",
  "laravel",
  "python",
] as const;

export const SDK_COUNT = SDK_PACKAGES.length;
