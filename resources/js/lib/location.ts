/*
 * Reading what the address carries. Secrets mailed in links sit after
 * the #, which browsers never send to a server or name in a referrer.
 */

/*
 * safeRedirect returns the path when it stays on this site, else the
 * fallback, so a crafted link cannot send a person elsewhere after they
 * sign in.
 */
export function safeRedirect(path: unknown, fallback = "/d"): string {
  if (typeof path !== "string" || !path.startsWith("/") || path.startsWith("//") || /[\\\r\n]/.test(path)) {
    return fallback;
  }

  return path;
}

/*
 * fragment returns the named value of the address's fragment, such as
 * the token in /reset-password#id=prs_1&token=abc, or "".
 */
export function fragment(name: string, hash: string = window.location.hash): string {
  return new URLSearchParams(hash.replace(/^#/, "")).get(name) ?? "";
}
