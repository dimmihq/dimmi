/*
 * Where a signed-in visitor's identity token comes from. The host app
 * mints a short-lived token on its own server for whoever is signed in,
 * and the messenger asks for one whenever it needs to know: an identity
 * URL on the same site, which answers {"token": "..."} or {"token":
 * null}, or a function the page hands over. The token stays in memory
 * only, never in storage, a URL or the console. Anything other than a
 * clear answer counts as signed out, so a shared computer never keeps the
 * previous person signed in.
 */
import { report } from "../lib/report";

/*
 * Source answers the visitor's current token, null when nobody is signed
 * in.
 */
export type Source = () => Promise<string | null>;

/*
 * fromUrl asks the host app's identity endpoint, with its cookies.
 */
export function fromUrl(fetcher: typeof fetch, url: string): Source {
  return async () => {
    let res: Response;
    try {
      res = await fetcher(url, {
        credentials: "same-origin",
        cache: "no-store",
        headers: { Accept: "application/json" },
      });
    } catch {
      report("warn", "identity_source_failed", `${url} could not be reached; the visitor stays signed out.`);
      return null;
    }
    const type = res.headers.get("Content-Type") ?? "";
    const body = res.ok && type.includes("json") ? ((await res.json().catch(() => null)) as unknown) : null;
    if (!body || typeof body !== "object" || !("token" in body)) {
      report(
        "error",
        "identity_endpoint_invalid",
        `${url} answered ${res.status} ${type || "without a content type"}; it must answer 200 with {"token": "<jwt>"} or {"token": null}.`,
      );
      return null;
    }
    const token = (body as { token: unknown }).token;
    return typeof token === "string" && token !== "" ? token : null;
  };
}

/*
 * fromFunction wraps a function the page passed, so a throw or a wrong
 * answer counts as signed out.
 */
export function fromFunction(fn: () => unknown): Source {
  return async () => {
    try {
      const token = await fn();
      return typeof token === "string" && token !== "" ? token : null;
    } catch {
      report("warn", "identity_source_failed", "the identity function threw; the visitor stays signed out.");
      return null;
    }
  };
}

/*
 * subjectOf reads the sub claim of a token without verifying it. It only
 * picks which request to send; the server checks the token itself.
 */
export function subjectOf(token: string): string | undefined {
  const part = token.split(".")[1];
  if (!part) {
    return undefined;
  }
  try {
    const json = atob(part.replace(/-/g, "+").replace(/_/g, "/"));
    const sub = (JSON.parse(json) as { sub?: unknown }).sub;
    return typeof sub === "string" ? sub.trim() : undefined;
  } catch {
    return undefined;
  }
}

/*
 * fingerprint is the hash stored to recognize the signed-in customer on
 * the next page, so their id itself is never written to storage.
 */
export async function fingerprint(sub: string): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest("SHA-256", new TextEncoder().encode(sub));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
