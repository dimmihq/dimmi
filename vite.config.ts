/*
 * Vite builds the browser app from resources/js. In development the dev
 * server serves modules and hot updates while the Go server serves every
 * page, so cookies, cross-origin protection and the API behave as in
 * production. The dev server writes its address to public/hot, which
 * tells the Go server to load scripts from it; the file is removed when
 * the dev server stops. A production build lands in public/build with a
 * manifest and is embedded in the binary; fonts stay files, since the
 * content security policy loads fonts from this origin only.
 */
import fs from "node:fs";
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

const devServer = "http://localhost:5173";
const goServer = "http://localhost:8080";
const hotFile = fileURLToPath(new URL("./public/hot", import.meta.url));

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
    assetsInlineLimit: (file) => (file.endsWith(".woff2") ? false : undefined),
    rollupOptions: { input: "resources/js/app.tsx" },
  },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["resources/js/**/*.test.{ts,tsx}"],
  },
}));
