/*
 * Where a sign-in through a company's identity provider starts.
 */
import type { SSOLookup } from "@/lib/api/auth";

/*
 * ssoStart is where the browser goes to sign in through the connection,
 * landing on the page asked for after.
 */
export function ssoStart(lookup: SSOLookup, redirect: string): string | null {
  if (!lookup.redirect_url) {
    return null;
  }

  return `${lookup.redirect_url}?${new URLSearchParams({ redirect }).toString()}`;
}
