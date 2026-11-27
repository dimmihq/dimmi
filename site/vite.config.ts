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
 *
 * The build names its release, the commit it was built from (DIMMI_RELEASE,
 * or GITHUB_SHA in CI), so the site's error reports carry the same release
 * as the app's.
 */
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";
import { vitePrerenderPlugin } from "vite-prerender-plugin";

import { generateSitemap } from "../scripts/generate-sitemap";
import { fetchGitHubStars } from "./github-stars";

function githubStarsPlugin(): Plugin {
  let pending: Promise<number | null> | undefined;
  return {
    name: "dimmi-github-stars",
    async config() {
      pending ??= fetchGitHubStars();
      const count = await pending;
      return {
        define: {
          __GITHUB_STARS__: JSON.stringify(count),
        },
      };
    },
  };
}

const release = process.env.DIMMI_RELEASE || process.env.GITHUB_SHA || "";

export default defineConfig({
  root: fileURLToPath(new URL(".", import.meta.url)),
  // Its own dependency cache, so it never shares one with the app's dev server.
  cacheDir: fileURLToPath(new URL("../node_modules/.vite-site", import.meta.url)),
  plugins: [
    githubStarsPlugin(),
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
  define: { "import.meta.env.DIMMI_RELEASE": JSON.stringify(release) },
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
