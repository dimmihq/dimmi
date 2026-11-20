/*
 * Facebook Pages and Instagram accounts: those the workspace connected,
 * what Meta's sign-in popup needs to open, the accounts a sign-in shared,
 * and connecting, moving and disconnecting one. In the inbox, where a
 * conversation stands on its platform and who its customer is there.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type SocialPlatform = "facebook" | "instagram";

export interface SocialChannel {
  id: string;
  platform: SocialPlatform;
  name: string;
  username: string;
  picture_url: string;
  status: string;
  inbox_id: string;
  account_id: string;
  last_error: string | null;
  created_at: string;
}

export interface SocialSignupConfig {
  available: boolean;
  app_id: string;
  config_id: string;
  graph_version: string;
}

export interface SocialAccount {
  id: string;
  name: string;
  username: string;
  picture_url: string;
  connected: boolean;
}

export interface SocialGrant {
  grant: string;
  accounts: SocialAccount[];
}

export interface SocialProfile {
  name: string;
  username: string;
  picture_url: string;
  followers: number;
  verified: boolean;
  follows: boolean;
}

/*
 * phase is "open" while anyone may reply, "human" while only a teammate
 * may, and "closed" until the customer writes again.
 */
export interface SocialConversation {
  channel_id: string;
  platform: SocialPlatform;
  phase: "open" | "human" | "closed";
  last_wrote_at: string | null;
  open_until: string | null;
  human_until: string | null;
  profile: SocialProfile;
}

export function socialChannelsQuery(workspace: string, platform: SocialPlatform) {
  return queryOptions({
    queryKey: ["workspace", workspace, "social", platform, "channels"],
    queryFn: () => request<SocialChannel[]>("GET", `/channels/${platform}`, undefined, { workspace }),
  });
}

export function socialSignupConfigQuery(workspace: string, platform: SocialPlatform) {
  return queryOptions({
    queryKey: ["workspace", workspace, "social", platform, "signup-config"],
    queryFn: () => request<SocialSignupConfig>("GET", `/channels/${platform}/signup-config`, undefined, { workspace }),
    staleTime: 5 * 60_000,
  });
}

export function socialConversationQuery(workspace: string, platform: SocialPlatform, conversation: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "social", "conversations", conversation],
    queryFn: () =>
      request<SocialConversation>("GET", `/conversations/${encodeURIComponent(conversation)}/${platform}`, undefined, {
        workspace,
      }),
  });
}

/*
 * startSocialSignup trades the code Meta's popup handed back for the
 * accounts it shared and a grant to connect one with.
 */
export function startSocialSignup(
  workspace: string,
  platform: SocialPlatform,
  result: { code: string; redirect_uri: string },
): Promise<SocialGrant> {
  return request<SocialGrant>("POST", `/channels/${platform}/signups`, result, { workspace });
}

/*
 * connectSocial connects the account picked from a sign-in.
 */
export function connectSocial(
  workspace: string,
  platform: SocialPlatform,
  body: { grant: string; account_id: string; inbox_id?: string },
): Promise<SocialChannel> {
  return request<SocialChannel>("POST", `/channels/${platform}`, body, { workspace });
}

/*
 * moveSocial sends the account's new conversations to another inbox.
 */
export function moveSocial(
  workspace: string,
  platform: SocialPlatform,
  channel: string,
  inbox: string,
): Promise<SocialChannel> {
  return request<SocialChannel>(
    "PATCH",
    `/channels/${platform}/${encodeURIComponent(channel)}`,
    { inbox_id: inbox },
    { workspace },
  );
}

/*
 * disconnectSocial stops the account's messages reaching the inbox.
 */
export function disconnectSocial(workspace: string, platform: SocialPlatform, channel: string): Promise<void> {
  return request<void>("DELETE", `/channels/${platform}/${encodeURIComponent(channel)}`, undefined, { workspace });
}
