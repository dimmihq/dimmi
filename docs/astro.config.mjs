/*
 * The docs site: Starlight for routing, search, the sidebar model and
 * code blocks, with every piece of its UI swapped for our own built on
 * the app's BoardUI components and tokens, so docs, marketing site and
 * product look the same.
 *
 * The docs are part of the marketing site, served under /docs on the
 * same domain: `astro build` writes plain static files into the site's
 * build, site/dist/docs, and `pnpm site:build` builds both (see
 * site/Dockerfile). Pages link to each other by their path without the
 * /docs prefix; src/base.ts adds it, and rehype-base.mjs does the same
 * for links written in page content.
 */
import { fileURLToPath } from "node:url";

import react from "@astrojs/react";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

import { sidebar } from "./src/nav.ts";
import { rehypeBase } from "./src/rehype-base.mjs";

const BASE = "/docs";

const here = (path) => fileURLToPath(new URL(path, import.meta.url));

// Every Starlight UI component, replaced by ours in src/overrides.
const overridden = [
  "Banner",
  "ContentPanel",
  "DraftContentNotice",
  "EditLink",
  "FallbackContentNotice",
  "Footer",
  "Header",
  "Hero",
  "LanguageSelect",
  "LastUpdated",
  "MarkdownContent",
  "MobileMenuFooter",
  "MobileMenuToggle",
  "MobileTableOfContents",
  "PageFrame",
  "PageSidebar",
  "PageTitle",
  "Pagination",
  "Search",
  "Sidebar",
  "SiteTitle",
  "SkipLink",
  "SocialIcons",
  "TableOfContents",
  "ThemeProvider",
  "ThemeSelect",
  "TwoColumnContent",
];

export default defineConfig({
  site: "https://trydimmi.com",
  base: BASE,
  outDir: here("../site/dist/docs"),
  cacheDir: here("../node_modules/.astro-docs"),
  server: { host: "localhost", port: 5190 },
  markdown: { rehypePlugins: [[rehypeBase, { base: BASE }]] },
  integrations: [
    starlight({
      title: "dimmi docs",
      description: "Set up, run and extend dimmi, the open-source customer support platform.",
      favicon: "/favicon.svg",
      credits: false,
      pagefind: true,
      lastUpdated: true,
      editLink: { baseUrl: "https://github.com/dimmihq/dimmi/edit/main/docs/" },
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      customCss: ["./src/styles/docs.css"],
      sidebar,
      components: Object.fromEntries(overridden.map((name) => [name, `./src/overrides/${name}.astro`])),
      expressiveCode: {
        themes: ["github-dark-default", "github-light-default"],
        useStarlightUiThemeColors: false,
        // Above BoardUI's prose layer, so prose never restyles a code block.
        cascadeLayer: "code",
        styleOverrides: {
          borderRadius: "0.75rem",
          borderColor: "var(--color-separator-border)",
          codeFontFamily: "var(--font-mono-source), ui-monospace, monospace",
          codeFontSize: "0.8125rem",
          codeLineHeight: "1.6",
          textMarkers: {
            markBackground: "color-mix(in oklab, var(--color-accent-500) 16%, transparent)",
            markBorderColor: "var(--color-accent-500)",
          },
          uiFontFamily: "var(--font-inter), ui-sans-serif, system-ui, sans-serif",
          frames: {
            shadowColor: "transparent",
            editorTabBarBackground: "var(--color-background-secondary-default)",
            editorActiveTabBackground: "var(--color-background-full)",
            editorActiveTabIndicatorTopColor: "var(--color-accent-500)",
            terminalTitlebarBackground: "var(--color-background-secondary-default)",
            terminalTitlebarBorderBottomColor: "var(--color-separator-border)",
          },
        },
      },
    }),
    react(),
  ],
  vite: {
    plugins: [tailwindcss()],
    resolve: {
      dedupe: ["react", "react-dom"],
      alias: {
        "@": here("../resources/js"),
        "~docs": here("./src"),
      },
    },
    ssr: { noExternal: ["@remixicon/react"] },
  },
});
