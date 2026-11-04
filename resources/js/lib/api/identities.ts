/*
 * The provider accounts linked to the person, such as GitHub or Google,
 * and the calls that read the recent proof of who they are, which linking
 * one needs, and unlink one. Linking itself happens in the browser, on
 * the server's redirect to the provider.
 */
import { queryOptions } from "@tanstack/react-query";

import type { Provider } from "./auth";
import { ApiError, request } from "./client";

export interface Identity {
  id: string;
  object: "oauth_identity";
  provider: string;
  email: string | null;
  username: string | null;
  avatar_url: string | null;
  last_login_at: string | null;
  created_at: string;
}

export const identitiesQuery = queryOptions({
  queryKey: ["identities"],
  queryFn: () => request<Identity[]>("GET", "/user/identities"),
});

export function unlinkIdentity(id: string): Promise<void> {
  return request<void>("DELETE", `/user/identities/${encodeURIComponent(id)}`);
}

/*
 * recentlyConfirmed reports whether the person proved who they are
 * recently enough to link a provider. The server answers not found when
 * no proof holds.
 */
export async function recentlyConfirmed(): Promise<boolean> {
  try {
    await request<unknown>("GET", "/user/confirmation");

    return true;
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return false;
    }

    throw error;
  }
}

/*
 * linkAddress is the server's redirect that links the provider to the
 * signed-in person.
 */
export function linkAddress(provider: Provider): string {
  return `${provider.redirect_url}?${new URLSearchParams({ intent: "link" }).toString()}`;
}
