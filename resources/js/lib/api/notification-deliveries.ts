/*
 * The delivery log of teammate notifications: how each one reached a
 * member on each channel, or why it did not; and the signed unsubscribe
 * link in teammate emails, read and changed without signing in.
 */
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { request, requestBody } from "./client";
import type { NotificationChannel } from "./notifications";

export type DeliveryStatus = "sent" | "failed" | "skipped";

export interface NotificationDelivery {
  id: string;
  object: "notification_delivery";
  delivery_id: string;
  member_id: string | null;
  trigger: string;
  channel: NotificationChannel;
  status: DeliveryStatus;
  reason: string | null;
  detail: string | null;
  title: string;
  link: string;
  attempts: number;
  occurred_at: string;
  created_at: string;
  updated_at: string;
}

export interface DeliveryPage {
  data: NotificationDelivery[];
  meta: { next_cursor: string | null };
}

export interface DeliveryFilter {
  member?: string;
  status?: DeliveryStatus;
}

function deliveriesPath(filter: DeliveryFilter, cursor: string | null): string {
  const params = new URLSearchParams({ limit: "20" });
  if (filter.member) {
    params.set("filter[member]", filter.member);
  }
  if (filter.status) {
    params.set("filter[status]", filter.status);
  }
  if (cursor) {
    params.set("cursor", cursor);
  }
  return `/notification-deliveries?${params.toString()}`;
}

export function notificationDeliveriesQuery(workspace: string, filter: DeliveryFilter = {}) {
  return infiniteQueryOptions({
    queryKey: ["workspace", workspace, "notification-deliveries", filter],
    queryFn: ({ pageParam }) =>
      requestBody<DeliveryPage>("GET", deliveriesPath(filter, pageParam), undefined, { workspace }),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.meta.next_cursor,
  });
}

export interface NotificationUnsubscribe {
  object: "notification_unsubscribe";
  workspace: string;
  email: string;
  trigger: string;
  label: string;
  subscribed: boolean;
  settings_url: string;
}

function unsubscribePath(token: string): string {
  return `/notification-unsubscribes/${encodeURIComponent(token)}`;
}

export function notificationUnsubscribeQuery(token: string) {
  return queryOptions({
    queryKey: ["notification-unsubscribes", token],
    queryFn: () => request<NotificationUnsubscribe>("GET", unsubscribePath(token)),
    retry: false,
  });
}

export function unsubscribeNotifications(token: string): Promise<NotificationUnsubscribe> {
  return request<NotificationUnsubscribe>("PATCH", unsubscribePath(token));
}

export function resubscribeNotifications(token: string): Promise<NotificationUnsubscribe> {
  return request<NotificationUnsubscribe>("DELETE", unsubscribePath(token));
}
