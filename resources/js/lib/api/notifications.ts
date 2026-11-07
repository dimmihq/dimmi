/*
 * A member's own notifications: the bell's list and counts, marking
 * them read or seen, the settings matrix and settings, the browsers
 * push is enabled on, and test sends. Live updates refetch the bell
 * through the notifications key.
 */
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { realtimeKeys } from "../realtime/keys";
import { request, requestBody } from "./client";

export type NotificationChannel = "in_app" | "email" | "push";

export interface MemberNotification {
  id: string;
  object: "notification";
  trigger: string;
  title: string;
  body: string;
  link: string;
  actor_name: string;
  conversation_id: string | null;
  conversation_number?: number;
  action_call_id: string | null;
  resolved?: string;
  count: number;
  read: boolean;
  seen: boolean;
  created_at: string;
  updated_at: string;
}

export interface NotificationCounts {
  unread_count: number;
  unseen_count: number;
}

export interface NotificationPage {
  data: MemberNotification[];
  meta: NotificationCounts & { next_cursor: string | null };
}

export interface NotificationTrigger {
  key: string;
  label: string;
  description: string;
}

export interface NotificationCell {
  trigger: string;
  channel: NotificationChannel;
  enabled: boolean;
  default: boolean;
  locked: boolean;
}

export interface NotificationSettings {
  timezone: string | null;
  quiet_hours_enabled: boolean;
  quiet_hours_start: string;
  quiet_hours_end: string;
  quiet_hours_days: number;
  quiet_hours_bypass: string[];
  paused_until: string | null;
  show_previews: boolean;
  include_ai_handled: boolean;
}

export interface NotificationPreferences {
  triggers: NotificationTrigger[];
  channels: NotificationChannel[];
  cells: NotificationCell[];
  settings: NotificationSettings;
}

export interface PushDevice {
  id: string;
  platform: string;
  device_label: string | null;
  endpoint: string;
  last_success_at: string | null;
  created_at: string;
}

export type NotificationFilter = "all" | "unread" | "mentioned" | "ai_action_approval_requested";

function listPath(filter: NotificationFilter, cursor: string | null, limit: number): string {
  const params = new URLSearchParams({ limit: String(limit) });
  if (filter === "unread") {
    params.set("filter[status]", "unread");
  } else if (filter !== "all") {
    params.set("filter[trigger]", filter);
  }
  if (cursor) {
    params.set("cursor", cursor);
  }
  return `/notifications?${params.toString()}`;
}

export function notificationsQuery(workspace: string, filter: NotificationFilter = "all") {
  return infiniteQueryOptions({
    queryKey: [...realtimeKeys.notifications(workspace), "list", filter],
    queryFn: ({ pageParam }) =>
      requestBody<NotificationPage>("GET", listPath(filter, pageParam, 20), undefined, { workspace }),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.meta.next_cursor,
  });
}

export function notificationCountsQuery(workspace: string) {
  return queryOptions({
    queryKey: [...realtimeKeys.notifications(workspace), "counts"],
    queryFn: async () =>
      (await requestBody<NotificationPage>("GET", listPath("all", null, 1), undefined, { workspace })).meta,
  });
}

export function markNotifications(
  workspace: string,
  change: { ids?: string[]; read?: boolean; seen?: boolean },
): Promise<{ meta: NotificationCounts }> {
  return requestBody<{ meta: NotificationCounts }>("PATCH", "/notifications", change, { workspace });
}

export function notificationPreferencesQuery(workspace: string) {
  return queryOptions({
    queryKey: [...realtimeKeys.notifications(workspace), "preferences"],
    queryFn: () => request<NotificationPreferences>("GET", "/notification-preferences", undefined, { workspace }),
  });
}

export function updateNotificationPreferences(
  workspace: string,
  preferences: { trigger: string; channel: NotificationChannel; enabled: boolean }[],
): Promise<NotificationPreferences> {
  return request<NotificationPreferences>("PATCH", "/notification-preferences", { preferences }, { workspace });
}

export function resetNotificationPreferences(workspace: string): Promise<NotificationPreferences> {
  return request<NotificationPreferences>("DELETE", "/notification-preferences", undefined, { workspace });
}

export function updateNotificationSettings(
  workspace: string,
  change: Partial<NotificationSettings> & { clear_pause?: boolean },
): Promise<NotificationSettings> {
  return request<NotificationSettings>("PATCH", "/notification-settings", change, { workspace });
}

export function sendTestNotification(workspace: string, channel: NotificationChannel): Promise<void> {
  return request<void>("POST", "/notification-tests", { channel }, { workspace });
}

export function focusNotifications(workspace: string, visible: boolean, conversation?: string): Promise<void> {
  return request<void>(
    "POST",
    "/notification-focus",
    { visible, ...(conversation ? { conversation_id: conversation } : {}) },
    { workspace },
  );
}

export function pushConfigQuery() {
  return queryOptions({
    queryKey: ["push-config"],
    queryFn: () => request<{ vapid_public_key: string; key_id: string }>("GET", "/push-config"),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function pushDevicesQuery() {
  return queryOptions({
    queryKey: ["push-subscriptions"],
    queryFn: () => request<PushDevice[]>("GET", "/push-subscriptions"),
  });
}

export function registerPushDevice(subscription: PushSubscriptionJSON, label: string): Promise<PushDevice> {
  return request<PushDevice>("POST", "/push-subscriptions", {
    endpoint: subscription.endpoint,
    keys: subscription.keys,
    device_label: label,
  });
}

export function removePushDevice(id: string): Promise<void> {
  return request<void>("DELETE", `/push-subscriptions/${encodeURIComponent(id)}`);
}
