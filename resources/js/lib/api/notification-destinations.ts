/*
 * The Slack places notifications go to: channels the team hears in,
 * which the workspace sets up, and each teammate's own direct messages.
 * A destination Slack refused stops with the reason and stays stopped
 * until someone turns it back on. A test posts a card now and says
 * whether Slack took it.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type DestinationDigest = "daily_standup" | "daily_summary" | "weekly_themes";

export interface NotificationDestination {
  id: string;
  object: "notification_destination";
  scope: "workspace" | "member";
  type: "slack_channel" | "slack_dm";
  name: string;
  integration_id: string;
  slack_channel_id: string | null;
  slack_channel_name: string | null;
  triggers: string[];
  inbox_ids: string[] | null;
  include_ai_handled: boolean;
  show_previews: boolean;
  digests: DestinationDigest[];
  digest_time: string;
  next_digest_at: string | null;
  status: "active" | "error" | "disabled";
  last_error_code: string | null;
  last_error: string | null;
  last_delivered_at: string | null;
  created_at: string;
}

export interface SlackPostable {
  id: string;
  object: "slack_channel";
  name: string;
  is_private: boolean;
}

export interface DestinationTest {
  object: "notification_destination_test";
  destination_id: string;
  status: "sent" | "failed";
  error_code: string | null;
  error: string | null;
}

/*
 * DestinationHearing is what a channel destination hears about. An
 * inbox list of null means every inbox.
 */
export interface DestinationHearing {
  triggers: string[];
  inbox_ids: string[] | null;
  include_ai_handled: boolean;
  show_previews: boolean;
  digests: DestinationDigest[];
  digest_time: string;
}

export function destinationsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "notification-destinations"],
    queryFn: () => request<NotificationDestination[]>("GET", "/notification-destinations", undefined, { workspace }),
  });
}

export function slackPostablesQuery(workspace: string, integration: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "integrations", integration, "slack-channels"],
    queryFn: () =>
      request<SlackPostable[]>("GET", `/integrations/${encodeURIComponent(integration)}/slack-channels`, undefined, {
        workspace,
      }),
  });
}

export function createChannelDestination(
  workspace: string,
  integration: string,
  channel: string,
  hearing: DestinationHearing,
): Promise<NotificationDestination> {
  const { inbox_ids, ...rest } = hearing;
  const body = { type: "slack_channel", integration_id: integration, channel_id: channel, ...rest };

  return request("POST", "/notification-destinations", inbox_ids ? { ...body, inbox_ids } : body, { workspace });
}

export function createDirectDestination(workspace: string, integration: string): Promise<NotificationDestination> {
  return request(
    "POST",
    "/notification-destinations",
    { type: "slack_dm", integration_id: integration },
    { workspace },
  );
}

/*
 * DestinationChange is what an edit sends. all_inboxes widens it back
 * to every inbox; enabled turns it off, or on again after a stop.
 */
export type DestinationChange = Partial<Omit<DestinationHearing, "inbox_ids">> & {
  inbox_ids?: string[];
  all_inboxes?: boolean;
  enabled?: boolean;
};

export function updateDestination(
  workspace: string,
  id: string,
  change: DestinationChange,
): Promise<NotificationDestination> {
  return request("PATCH", `/notification-destinations/${encodeURIComponent(id)}`, change, { workspace });
}

export function deleteDestination(workspace: string, id: string): Promise<unknown> {
  return request("DELETE", `/notification-destinations/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function testDestination(workspace: string, id: string): Promise<DestinationTest> {
  return request("POST", `/notification-destinations/${encodeURIComponent(id)}/tests`, undefined, { workspace });
}
