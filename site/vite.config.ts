/*
 * Vite builds the marketing site from site/. It shares the app's
 * BoardUI components, tokens and fonts through the @/ alias, so the site
 * and the product look the same. Every page is prerendered at build time
 * into its own static HTML file: vite-prerender-plugin calls the
 * prerender() export in src/main.tsx for each route, and the browser
 * then hydrates that HTML. There is no server; the build is plain files.
 *
 * The docs (docs/, Astro) are part of the site under /docs: `pnpm
 * site:build` builds this site, then the docs into dist/docs. In
 * development /docs is proxied to the docs' own dev server (pnpm docs:dev).
 */
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { vitePrerenderPlugin } from "vite-prerender-plugin";

import { generateSitemap } from "../scripts/generate-sitemap";

export default defineConfig({
  root: fileURLToPath(new URL(".", import.meta.url)),
  // Its own dependency cache, so it never shares one with the app's dev server.
  cacheDir: fileURLToPath(new URL("../node_modules/.vite-site", import.meta.url)),
  plugins: [
    react(),
    tailwindcss(),
    vitePrerenderPlugin({ renderTarget: "#root" }),
    {
      name: "dimmi-sitemap",
      closeBundle() {
        generateSitemap(fileURLToPath(new URL("./dist", import.meta.url)));
      },
    },
  ],
  resolve: {
    dedupe: ["react", "react-dom"],
    alias: {
      "@": fileURLToPath(new URL("../resources/js", import.meta.url)),
      "~site": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  optimizeDeps: {
    include: ["react", "react-dom", "react-dom/client", "motion/react", "three", "@remixicon/react", "tailwind-merge"],
  },
  server: {
    host: "localhost",
    port: 5180,
    strictPort: true,
    proxy: { "/docs": { target: "http://localhost:5190", ws: true } },
  },
  preview: { host: "localhost", port: 5181, strictPort: true },
  build: { outDir: "dist", emptyOutDir: true },
});
