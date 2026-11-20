/*
 * The sidebar each header tab shows, and the pagination and eyebrow that
 * follow from it. Guide tabs keep only their own groups of Starlight's
 * sidebar; the API reference builds its sidebar from the contract. Its
 * categories hold hundreds of endpoints, so only the open resource lists
 * them: other resources are one link each, to their first endpoint.
 */
import { type ApiNavGroup, type ApiNavLink, apiNavigation } from "./api/spec";
import { stripBase, withBase } from "./base";
import type { SidebarEntry } from "./components/react/sidebar";
import { sectionFor } from "./nav";

type Link = Extract<SidebarEntry, { type: "link" }>;

const withSlash = (path: string) => (path.endsWith("/") ? path : `${path}/`);

function hasCurrent(entry: SidebarEntry): boolean {
  return entry.type === "link" ? entry.isCurrent : entry.entries.some(hasCurrent);
}

function flatten(entries: SidebarEntry[]): Link[] {
  return entries.flatMap((entry) => (entry.type === "link" ? [entry] : flatten(entry.entries)));
}

function fromApi(item: ApiNavGroup | ApiNavLink, path: string): SidebarEntry {
  if ("items" in item) {
    return {
      type: "group",
      label: item.label,
      entries: item.items.map((child) => fromApi(child, path)),
      collapsed: true,
      badge: undefined,
    };
  }
  return {
    type: "link",
    label: item.label,
    href: withBase(item.href),
    isCurrent: withSlash(item.href) === withSlash(path),
    badge: undefined,
    method: item.method,
  };
}

// Every entry of the current tab, in order.
// `path` is the page's URL; sections and API links use paths inside the docs.
function fullSidebar(url: string, starlight: SidebarEntry[]): SidebarEntry[] {
  const path = stripBase(url);
  const section = sectionFor(path);
  if (section.key === "api") {
    return apiNavigation().map((group) => fromApi(group, path));
  }
  return starlight.filter((entry) => entry.type === "group" && section.groups.includes(entry.label));
}

type Group = Extract<SidebarEntry, { type: "group" }>;

// A nested group away from the current page becomes one link to its first page.
function condense(group: Group): Group {
  return {
    ...group,
    entries: group.entries.map((child) => {
      if (child.type === "link") {
        return child;
      }
      if (hasCurrent(child)) {
        return condense(child);
      }
      const first = flatten(child.entries)[0];
      return { type: "link", label: child.label, href: first.href, isCurrent: false, badge: child.badge };
    }),
  };
}

export function sidebarFor(path: string, starlight: SidebarEntry[]): SidebarEntry[] {
  return fullSidebar(path, starlight).map((entry) => (entry.type === "group" ? condense(entry) : entry));
}

export function paginationFor(path: string, starlight: SidebarEntry[]) {
  const links = flatten(fullSidebar(path, starlight));
  const index = links.findIndex((link) => link.isCurrent);
  const pick = (link?: Link) => (link ? { href: link.href, label: link.label } : undefined);
  return index === -1 ? {} : { prev: pick(links[index - 1]), next: pick(links[index + 1]) };
}

// The top-level group holding the current page, shown above its title.
export function groupLabelFor(path: string, starlight: SidebarEntry[]): string | undefined {
  return fullSidebar(path, starlight).find((entry) => entry.type === "group" && hasCurrent(entry))?.label;
}
