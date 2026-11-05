/*
 * The checks a link in a customer's message goes through before a
 * teammate opens it. They run on the link as shown, never on the stored
 * text, and look only at the link itself: no third party ever learns
 * which links customers send. A blocked domain cannot be opened. An
 * allowed one opens without a word. Otherwise a link earns a warning when
 * its text names another host, its host mixes scripts or is spelled in
 * punycode, it goes through a URL shortener, or it points at a bare
 * address.
 */
import { createContext } from "react";

export interface LinkSafety {
  enabled: boolean;
  allow_domains: string[];
  block_domains: string[];
  strip_inbound_email_links: boolean;
}

/*
 * LinkPolicy is how links in a thread's customer messages behave: the
 * workspace's settings, and whether they show as plain text.
 */
export interface LinkPolicy {
  settings: LinkSafety;
  plain: boolean;
}

export const LinkPolicyContext = createContext<LinkPolicy | null>(null);

export type Reason = "other_host" | "lookalike" | "shortener" | "address";

export type Verdict =
  | { kind: "open"; host: string }
  | { kind: "warn"; host: string; reasons: Reason[] }
  | { kind: "blocked"; host: string };

const shorteners = [
  "bit.ly",
  "buff.ly",
  "cutt.ly",
  "goo.gl",
  "is.gd",
  "lnkd.in",
  "ow.ly",
  "rb.gy",
  "rebrand.ly",
  "shorturl.at",
  "t.co",
  "t.ly",
  "tiny.cc",
  "tinyurl.com",
];

/*
 * hostOf returns the lower-case host of a web link, or null for anything
 * that is not http or https.
 */
export function hostOf(href: string): string | null {
  try {
    const url = new URL(href);

    return url.protocol === "http:" || url.protocol === "https:" ? url.hostname.toLowerCase() : null;
  } catch {
    return null;
  }
}

/*
 * under reports whether the host is the domain or a name under it.
 */
export function under(host: string, domain: string): boolean {
  return host === domain || host.endsWith(`.${domain}`);
}

function isAddress(host: string): boolean {
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(host) || host.startsWith("[");
}

/*
 * looksAlike reports a host spelled in punycode, or written with letters
 * outside ASCII, the way look-alike domains imitate familiar ones.
 */
function looksAlike(host: string): boolean {
  const foreign = [...host].some((letter) => (letter.codePointAt(0) ?? 0) > 127);

  return foreign || host.split(".").some((label) => label.startsWith("xn--"));
}

/*
 * namedHost returns the host the link's text names, when the text looks
 * like a web address or a bare domain.
 */
function namedHost(text: string): string | null {
  const trimmed = text.trim().toLowerCase();
  if (!/^(https?:\/\/)?[a-z0-9.-]+\.[a-z]{2,}(\/\S*)?$/.test(trimmed)) {
    return null;
  }

  return hostOf(trimmed.startsWith("http") ? trimmed : `https://${trimmed}`);
}

function sameSite(a: string, b: string): boolean {
  const strip = (host: string) => host.replace(/^www\./, "");

  return strip(a) === strip(b);
}

/*
 * reasonsFor lists why a link deserves a second look.
 */
export function reasonsFor(host: string, text: string): Reason[] {
  const reasons: Reason[] = [];
  const named = namedHost(text);
  if (named && !sameSite(named, host)) {
    reasons.push("other_host");
  }
  if (looksAlike(host)) {
    reasons.push("lookalike");
  }
  if (shorteners.some((domain) => under(host, domain))) {
    reasons.push("shortener");
  }
  if (isAddress(host)) {
    reasons.push("address");
  }

  return reasons;
}

/*
 * check decides how a link opens under the workspace's settings.
 */
export function check(href: string, text: string, settings: LinkSafety): Verdict | null {
  const host = hostOf(href);
  if (!host) {
    return null;
  }
  if (settings.block_domains.some((domain) => under(host, domain))) {
    return { kind: "blocked", host };
  }
  if (!settings.enabled || settings.allow_domains.some((domain) => under(host, domain))) {
    return { kind: "open", host };
  }

  const reasons = reasonsFor(host, text);

  return reasons.length === 0 ? { kind: "open", host } : { kind: "warn", host, reasons };
}

const reasonWords: Record<Reason, string> = {
  other_host: "Its text names a different site.",
  lookalike: "Its address uses look-alike letters.",
  shortener: "It goes through a link shortener, which hides where it ends up.",
  address: "It points at a bare server address instead of a site name.",
};

/*
 * reasonText says in words why a link was flagged.
 */
export function reasonText(reason: Reason): string {
  return reasonWords[reason];
}
