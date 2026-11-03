/*
 * The messenger widget's Vite config, apart from the dashboard's because
 * the widget is not React: its JSX compiles to the widget's own runtime,
 * which turns components into HTML strings. The build script and the
 * widget's tests both start from this file.
 */
import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

const runtime = fileURLToPath(new URL("./resources/widget/jsx/jsx-runtime.ts", import.meta.url));

export default defineConfig({
  publicDir: false,
  resolve: {
    alias: { "widget-jsx/jsx-runtime": runtime, "widget-jsx/jsx-dev-runtime": runtime },
  },
  oxc: {
    jsx: { runtime: "automatic", importSource: "widget-jsx" },
  },
  test: {
    name: "widget",
    environment: "jsdom",
    globals: true,
    include: ["resources/widget/**/*.test.{ts,tsx}"],
  },
});
