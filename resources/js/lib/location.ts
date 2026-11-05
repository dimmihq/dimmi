/*
 * Reading what the address carries, and where the person is. Secrets
 * mailed in links sit after the #, which browsers never send to a server or
 * name in a referrer. Times show in the zone the person chose, else their
 * browser's.
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

/*
 * browserZone returns the time zone the browser runs in.
 */
export function browserZone(): string {
  return new Intl.DateTimeFormat().resolvedOptions().timeZone;
}

/*
 * displayZone returns the zone times show in for a person: their own
 * choice from /me, else undefined, which leaves it to the browser.
 */
export function displayZone(me: { user: { timezone: string | null } } | null | undefined): string | undefined {
  return me?.user.timezone ?? undefined;
}

/*
 * shouldOfferZone reports whether to offer the browser's zone: only to a
 * person following the workspace from somewhere else.
 */
export function shouldOfferZone(own: string | null, workspace: string, browser: string): boolean {
  return own === null && browser !== "" && browser !== workspace;
}

/*
 * zoneChoices lists the zones to pick from, always with UTC, which some
 * browsers leave out, and the person's current one.
 */
export function zoneChoices(current: string | null, supported: string[]): string[] {
  const zones = new Set(["UTC", ...supported]);
  if (current) {
    zones.add(current);
  }

  return [...zones];
}
