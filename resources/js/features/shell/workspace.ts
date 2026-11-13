/*
 * Which of the person's workspaces the app shows. The address says: every
 * dashboard address starts with the workspace's slug, /d/acme/inbox/…, so
 * a link someone shares opens the same workspace for whoever follows it.
 * The routes themselves never see the slug; the router takes it off the
 * address on the way in and puts the current one back on the way out.
 * A slug the workspace gave up and still holds finds it too, and the
 * address then moves to the slug it has now. An address without a slug,
 * such as an old link, opens the workspace this browser used last.
 */
import type { LocationRewrite } from "@tanstack/react-router";

import type { WorkspaceSummary } from "@/lib/api/auth";

const key = "dimmi:workspace";

/*
 * pages are the first parts of dashboard addresses that are pages, not
 * slugs; the server never hands these out as slugs.
 */
export const pages = new Set([
  "inbox",
  "tickets",
  "reports",
  "settings",
  "customers",
  "notifications",
  "agent",
  "help-center",
  "feedback",
  "announcements",
  "broadcasts",
  "series",
  "changelog",
  "shortcuts",
  "procedures",
  "workflows",
  "operator",
]);

/*
 * address is what the browser's address says about the workspace: the
 * slug it names, if any, and the slug of the workspace being shown,
 * which every address the app writes carries.
 */
const address: { named: string | null; shown: string | null } = { named: null, shown: null };

function remembered(): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

/*
 * keep writes the browser's choice of workspace.
 */
function keep(id: string): boolean {
  try {
    window.localStorage.setItem(key, id);

    return true;
  } catch {
    return false;
  }
}

/*
 * rememberWorkspace makes the workspace the one this browser opens, such
 * as one the person just joined, so the next address names it rather
 * than the workspace shown until now. Blocked storage keeps the choice
 * for the current page only, and the answer says whether it was kept.
 */
export function rememberWorkspace(id: string): boolean {
  address.named = null;
  address.shown = null;

  return keep(id);
}

/*
 * holds reports whether the workspace answers to the slug: its own, or
 * one it gave up and still holds.
 */
function holds(workspace: WorkspaceSummary, slug: string, now: number): boolean {
  if (workspace.slug.toLowerCase() === slug.toLowerCase()) {
    return true;
  }

  return (workspace.previous_slugs ?? []).some(
    (held) => held.slug.toLowerCase() === slug.toLowerCase() && Date.parse(held.until) > now,
  );
}

/*
 * Addressed is the workspace the address leads to: the one it names, or
 * when it names none the one this browser opens. missing is the slug of
 * a workspace the person is not in, and stale says the address names the
 * workspace by an old slug or by none, so it should be written again.
 */
export interface Addressed {
  workspace: WorkspaceSummary | null;
  missing: string | null;
  stale: boolean;
}

/*
 * addressed works out which workspace the address leads to, and makes it
 * the one every address the app writes carries.
 */
export function addressed(workspaces: WorkspaceSummary[]): Addressed {
  const named = address.named;
  if (named) {
    const workspace = workspaces.find((w) => holds(w, named, Date.now())) ?? null;
    if (!workspace) {
      return { workspace: null, missing: named, stale: false };
    }
    keep(workspace.id);
    address.shown = workspace.slug;

    return { workspace, missing: null, stale: workspace.slug !== named };
  }

  const chosen = remembered();
  const workspace = workspaces.find((w) => w.id === chosen) ?? workspaces[0] ?? null;
  address.shown = workspace?.slug ?? null;

  return { workspace, missing: null, stale: workspace !== null };
}

/*
 * currentWorkspace is the workspace the app shows, as routes need to know
 * before they render.
 */
export function currentWorkspace(workspaces: WorkspaceSummary[]): WorkspaceSummary | null {
  return addressed(workspaces).workspace;
}

/*
 * useCurrentWorkspace returns the workspace the app shows, and a way to
 * switch to another, which the next address carries.
 */
export function useCurrentWorkspace(workspaces: WorkspaceSummary[]) {
  const current = currentWorkspace(workspaces) ?? workspaces[0] ?? null;
  const choose = (id: string) => {
    const next = workspaces.find((w) => w.id === id);
    if (!next) {
      return;
    }
    keep(id);
    address.named = next.slug;
    address.shown = next.slug;
  };

  return { current, choose };
}

/*
 * showSlug writes the workspace's slug into the address bar in place of
 * an old one or none, without a new history entry, so the address can be
 * shared as it stands.
 */
export function showSlug(slug: string) {
  const parts = window.location.pathname.split("/");
  if (parts[1] !== "d") {
    return;
  }
  const rest = parts[2] && !pages.has(parts[2]) ? parts.slice(3) : parts.slice(2);
  const path = ["", "d", encodeURIComponent(slug), ...rest].join("/").replace(/\/$/, "");
  window.history.replaceState(window.history.state, "", path + window.location.search + window.location.hash);
}

/*
 * workspaceAddress takes the slug off dashboard addresses for the routes,
 * noting which workspace they name, and puts the shown workspace's slug
 * back on every dashboard address the app writes.
 */
export const workspaceAddress: LocationRewrite = {
  input: ({ url }) => {
    const parts = url.pathname.split("/");
    if (parts[1] !== "d") {
      return undefined;
    }
    const first = parts[2] ?? "";
    if (first === "" || pages.has(first)) {
      address.named = null;
      return undefined;
    }
    address.named = decodeURIComponent(first);
    const next = new URL(url);
    next.pathname = ["", "d", ...parts.slice(3)].join("/").replace(/\/$/, "") || "/d";

    return next;
  },
  output: ({ url }) => {
    const parts = url.pathname.split("/");
    if (parts[1] !== "d" || !address.shown || (parts[2] && !pages.has(parts[2]))) {
      return undefined;
    }
    const next = new URL(url);
    next.pathname = ["", "d", encodeURIComponent(address.shown), ...parts.slice(2)].join("/").replace(/\/$/, "");

    return next;
  },
};
