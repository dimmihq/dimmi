/*
 * Vite builds the browser app from resources/js. In development the dev
 * server serves modules and hot updates while the Go server serves every
 * page, so cookies, cross-origin protection and the API behave as in
 * production. The dev server writes its address to public/hot, which
 * tells the Go server to load scripts from it; the file is removed when
 * the dev server stops. A production build lands in public/build with a
 * manifest and is embedded in the binary; fonts stay files, since the
 * content security policy loads fonts from this origin only. The public
 * portal (help center, changelog, feedback) is a second entry, so its
 * pages never load the dashboard, and its stylesheet is an entry of its
 * own, so the server links it in the page's head and pages are styled
 * from their first paint, in development too.
 *
 * When CI hands the build an error tracker token (SENTRY_AUTH_TOKEN), it
 * writes source maps, uploads them to the tracker (errors.trydimmi.com,
 * Sentry-compatible) and deletes them, so reports show the app's own code
 * while no map ships in the binary. The scripts carry debug ids that tie
 * a report to its map. A tracker that is down only warns: the build and
 * the release go on without maps.
 */
import fs from "node:fs";
import { fileURLToPath } from "node:url";

import { sentryVitePlugin } from "@sentry/vite-plugin";
import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

const devServer = "http://localhost:5173";
const goServer = "http://localhost:8080";
const hotFile = fileURLToPath(new URL("./public/hot", import.meta.url));
const errorsToken = process.env.SENTRY_AUTH_TOKEN ?? "";

function sourceMaps(): Plugin[] {
  if (errorsToken === "") {
    return [];
  }

  return sentryVitePlugin({
    url: process.env.SENTRY_URL ?? "https://errors.trydimmi.com",
    org: "_",
    project: "dashboard",
    authToken: errorsToken,
    telemetry: false,
    // The release is the commit, as the server names it. It is marked
    // released when a deploy goes live (deploy/release/tracker.sh), not here.
    release: { name: process.env.GITHUB_SHA, inject: false, finalize: false },
    sourcemaps: { filesToDeleteAfterUpload: ["public/build/**/*.map"] },
    errorHandler: (err) => {
      process.stderr.write(`warning: source maps were not uploaded: ${err.message}\n`);
    },
  });
}

function hot(): Plugin {
  const remove = () => fs.rmSync(hotFile, { force: true });

  return {
    name: "dimmi:hot-file",
    apply: "serve",
    configureServer(server) {
      server.httpServer?.once("listening", () => fs.writeFileSync(hotFile, devServer));
      server.httpServer?.once("close", remove);

      for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"] as const) {
        process.once(signal, () => {
          remove();
          process.exit();
        });
      }
    },
  };
}

export default defineConfig(({ command }) => ({
  base: command === "build" ? "/build/" : "/",
  publicDir: false,
  plugins: [
    tanstackRouter({
      target: "react",
      routesDirectory: "resources/js/routes",
      generatedRouteTree: "resources/js/route-tree.gen.ts",
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
    hot(),
    ...sourceMaps(),
  ],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./resources/js", import.meta.url)) },
  },
  server: {
    host: "localhost",
    port: 5173,
    strictPort: true,
    origin: devServer,
    cors: { origin: goServer },
  },
  build: {
    outDir: "public/build",
    emptyOutDir: true,
    manifest: true,
    sourcemap: errorsToken === "" ? false : "hidden",
    assetsInlineLimit: (file) => (file.endsWith(".woff2") ? false : undefined),
    rollupOptions: {
      input: ["resources/js/app.tsx", "resources/js/portal/portal.ts", "resources/js/portal/portal.css"],
    },
  },
  test: {
    projects: [
      {
        extends: true,
        test: { name: "app", environment: "jsdom", globals: true, include: ["resources/js/**/*.test.{ts,tsx}"] },
      },
      "./vite.widget.config.ts",
      {
        extends: true,
        resolve: {
          alias: {
            "@dimmi/server": fileURLToPath(new URL("./sdk/server/src/index.ts", import.meta.url)),
            "@dimmi/browser": fileURLToPath(new URL("./sdk/browser/src/index.ts", import.meta.url)),
            "@dimmi/react": fileURLToPath(new URL("./sdk/react/src/index.ts", import.meta.url)),
          },
        },
        test: { name: "sdk", environment: "node", globals: true, include: ["sdk/*/src/**/*.test.ts"] },
      },
    ],
  },
}));
