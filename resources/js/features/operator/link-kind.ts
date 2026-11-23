/*
 * What a link in one of Operator's messages opens, read from its
 * address: a page inside dimmi, by kind, or a page outside it. The kind
 * picks the link's glyph, and a link written as a bare address gets a
 * name from it, so no message shows a raw address.
 */
export type LinkKind =
  | "conversation"
  | "article"
  | "procedure"
  | "routine"
  | "changelog"
  | "request"
  | "page"
  | "public"
  | "outside";

export interface ReadLink {
  kind: LinkKind;
  /* The path to open inside dimmi, or null for a link that leaves it. */
  inApp: string | null;
  /* A name for the link when it was written as a bare address. */
  name: string;
}

const kinds: [RegExp, LinkKind, string][] = [
  [/^\/d\/[^/]+\/inbox\//, "conversation", "Open conversation"],
  [/^\/d\/[^/]+\/help-center\/art_/, "article", "Open article"],
  [/^\/d\/[^/]+\/procedures\//, "procedure", "Open procedure"],
  [/^\/d\/[^/]+\/routines\//, "routine", "Open routine"],
  [/^\/d\/[^/]+\/changelog\//, "changelog", "Open changelog entry"],
  [/^\/d\/[^/]+\/feedback\//, "request", "Open feature request"],
  [/^\/d\//, "page", "Open in dimmi"],
];

/*
 * titled turns the last part of an address into words: a help center
 * slug such as plans-and-label-allowances reads as its title.
 */
function titled(path: string, fallback: string): string {
  const last = path.split("/").filter(Boolean).at(-1) ?? "";
  const words = decodeURIComponent(last).replace(/^\d+-/, "").replace(/[-_]+/g, " ").trim();

  return words ? words.charAt(0).toUpperCase() + words.slice(1) : fallback;
}

/*
 * readLink says what an address opens. Pages under /d/ are dimmi's own,
 * whichever host the address names, so they open in this window.
 */
export function readLink(href: string, origin: string): ReadLink {
  let url: URL;
  try {
    url = new URL(href, origin);
  } catch {
    return { kind: "outside", inApp: null, name: href };
  }
  for (const [pattern, kind, name] of kinds) {
    if (pattern.test(url.pathname)) {
      return { kind, inApp: url.pathname + url.search + url.hash, name };
    }
  }
  const helpCenter = /^\/hc\//.test(url.pathname) || /\/(articles|changelog|feedback)\//.test(url.pathname);

  return helpCenter
    ? { kind: "public", inApp: null, name: titled(url.pathname, url.hostname) }
    : { kind: "outside", inApp: null, name: url.hostname.replace(/^www\./, "") };
}
