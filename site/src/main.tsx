/*
 * The site's one entry. In the browser it hydrates the prerendered HTML
 * (or renders, in development). At build time vite-prerender-plugin
 * calls prerender() for every route and writes the HTML it returns.
 */
import "./site.css";

import { StrictMode } from "react";
import { createRoot, hydrateRoot } from "react-dom/client";

import { App } from "./app";
import { startMonitoring } from "./monitoring";
import { DEFAULT_TITLE, findRoute, routes } from "./routes";
import { headElements } from "./seo";
import { captureUtm } from "./track";

if (typeof window !== "undefined") {
  captureUtm();
  startMonitoring();
  const target = document.getElementById("root");
  if (target) {
    const app = (
      <StrictMode>
        <App path={window.location.pathname} />
      </StrictMode>
    );
    if (target.hasChildNodes()) {
      hydrateRoot(target, app);
    } else {
      createRoot(target).render(app);
    }
  }
}

export async function prerender(data: { url: string }) {
  // The edge build, because the browser build opens a MessageChannel when it
  // loads that keeps the build process from ever exiting.
  const { renderToString } = await import("react-dom/server.edge");
  const { parseLinks } = await import("vite-prerender-plugin/parse");

  const html = renderToString(
    <StrictMode>
      <App path={data.url} />
    </StrictMode>,
  );
  const route = findRoute(data.url);
  const known = new Set(routes.map((r) => r.path));
  const links = new Set(parseLinks(html).filter((link) => known.has(link)));

  return {
    html,
    links,
    head: {
      lang: "en",
      title: route?.title ?? DEFAULT_TITLE,
      elements: headElements(route),
    },
  };
}
