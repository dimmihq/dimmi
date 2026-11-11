/*
 * The tools a workspace connects, such as Slack: which ones this install
 * offers and what each feature asks for, the workspace's installations
 * with their health, and each teammate's own grant. Installing and
 * granting answer the provider address to send the browser to; the
 * provider sends it back to the page it came from. A Slack installation
 * also feeds the channels it watches, each a room with its own settings.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface IntegrationProviderFeature {
  name: string;
  bot_scopes: string[];
  user_scopes: string[];
}

export interface IntegrationProvider {
  object: "integration_provider";
  name: string;
  label: string;
  installable: boolean;
  features: IntegrationProviderFeature[];
}

export interface IntegrationMemberGrant {
  id: string;
  object: "integration_member_grant";
  member_id: string;
  external_user_id: string;
  scopes: string[];
  status: string;
  created_at: string;
  revoked_at: string | null;
}

export interface IntegrationFeature {
  name: string;
  status: "ready" | "blocked_missing_scope";
  missing_scopes: string[];
}

export type IntegrationStatus = "active" | "error" | "disconnected" | "revoked";

export interface Integration {
  id: string;
  object: "integration";
  provider: string;
  name: string;
  external_account_id: string | null;
  source: string;
  status: IntegrationStatus;
  scopes: string[];
  last_error_code: string | null;
  last_error: string | null;
  metadata: Record<string, unknown>;
  features: IntegrationFeature[];
  member_grant_count: number;
  channel_ids: string[];
  own_grant: IntegrationMemberGrant | null;
  installed_at: string;
  last_checked_at: string | null;
}

export type RoomMode = "shared" | "community" | "internal" | "helpdesk";

export type RoomTrigger = "author_window" | "every_message" | "emoji" | "mention" | "ai" | "auto" | "off";

/*
 * RoomSettings are the settings a room's page changes. Every one is
 * optional: a room without one follows the default.
 */
export interface RoomSettings {
  agent_mode?: "auto" | "draft";
  agent_delay_seconds?: number;
  agent_delay_when?: "always" | "inside_hours";
  triage_channel_id?: string;
  auto_reply_after_seconds?: number;
  auto_join?: boolean;
  request_sensitivity?: "conservative" | "aggressive";
}

export interface SlackRoom {
  id: string;
  object: "channel_room";
  channel_id: string;
  external_id: string;
  name: string;
  status: "active" | "paused" | "removed" | "error";
  mode: RoomMode;
  trigger: RoomTrigger;
  is_shared_external: boolean;
  settings: RoomSettings;
  last_error: string | null;
  removed_at: string | null;
}

/*
 * RoomChange is what a save sends: only what changed. A setting sent as
 * null goes back to its default.
 */
export interface RoomChange {
  mode?: RoomMode;
  trigger?: RoomTrigger;
  settings?: { [K in keyof RoomSettings]?: RoomSettings[K] | null };
}

export function integrationProvidersQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "integration-providers"],
    queryFn: () => request<IntegrationProvider[]>("GET", "/integration-providers", undefined, { workspace }),
  });
}

export function integrationsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "integrations"],
    queryFn: () => request<Integration[]>("GET", "/integrations", undefined, { workspace }),
  });
}

export function integrationQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "integrations", id],
    queryFn: () => request<Integration>("GET", `/integrations/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function startInstallation(
  workspace: string,
  provider: string,
  body: { features?: string[]; return_to: string; account?: string },
): Promise<{ authorize_url: string }> {
  return request("POST", `/integrations/${encodeURIComponent(provider)}/installations`, body, { workspace });
}

export function disconnectIntegration(workspace: string, id: string): Promise<unknown> {
  return request("DELETE", `/integrations/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function startMemberGrant(workspace: string, id: string, returnTo: string): Promise<{ authorize_url: string }> {
  return request(
    "POST",
    `/integrations/${encodeURIComponent(id)}/member-grants`,
    { return_to: returnTo },
    { workspace },
  );
}

export function revokeMemberGrant(workspace: string, id: string, grant: string): Promise<unknown> {
  return request(
    "DELETE",
    `/integrations/${encodeURIComponent(id)}/member-grants/${encodeURIComponent(grant)}`,
    undefined,
    { workspace },
  );
}

export function slackRoomsQuery(workspace: string, channel: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "channels", channel, "rooms"],
    queryFn: () =>
      request<SlackRoom[]>("GET", `/channels/${encodeURIComponent(channel)}/rooms`, undefined, { workspace }),
  });
}

export function updateSlackRoom(workspace: string, room: SlackRoom, change: RoomChange): Promise<SlackRoom> {
  return request<SlackRoom>(
    "PATCH",
    `/channels/${encodeURIComponent(room.channel_id)}/rooms/${encodeURIComponent(room.id)}`,
    change,
    { workspace },
  );
}
