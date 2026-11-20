/*
 * The docs live under /docs on the marketing site (astro.config.mjs).
 * Code names pages by their path inside the docs, such as /quickstart/;
 * withBase turns that into the URL to link to, and stripBase turns the
 * URL of the page being built back into that path.
 */

export const BASE = import.meta.env.BASE_URL.replace(/\/+$/, "");

export function withBase(path: string): string {
  if (!path.startsWith("/") || path.startsWith("//") || path === BASE || path.startsWith(`${BASE}/`)) {
    return path;
  }
  return `${BASE}${path}`;
}

export function stripBase(path: string): string {
  if (!BASE || !(path === BASE || path.startsWith(`${BASE}/`))) {
    return path;
  }
  return path.slice(BASE.length) || "/";
}
