/*
 * validLink reports whether a button may link somewhere, the way the help
 * center checks it: a web or mail address, or a path on the same site.
 */
export function validLink(href: string): boolean {
  const text = href.trim();
  if (/^\/(?!\/)/.test(text) || text.startsWith("#")) {
    return true;
  }
  if (/^mailto:\S+$/i.test(text)) {
    return true;
  }
  try {
    const url = new URL(text);
    return (url.protocol === "https:" || url.protocol === "http:") && url.host !== "";
  } catch {
    return false;
  }
}
