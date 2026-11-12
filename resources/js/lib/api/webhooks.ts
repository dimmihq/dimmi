/*
 * The workspace's outbound webhooks on screen: the catalog of events an
 * endpoint can subscribe to, the endpoints and their numbers, an
 * endpoint's delivery log, and the calls that add, change, test and
 * remove an endpoint, reveal or rotate its secret and send a delivery
 * again. A new endpoint's secret comes back once, beside it.
 */
import { queryOptions } from "@tanstack/react-query";

import { request, requestBody, requestPage } from "./client";

export interface WebhookEventType {
  object: "webhook_event_type";
  type: string;
  object_type: string;
  group: string;
  description: string;
  notes_opt_in: boolean;
  named_only: boolean;
}

export type WebhookHealth = "healthy" | "failing" | "disabled" | string;

export interface WebhookEndpoint {
  id: string;
  object: "webhook_endpoint";
  url: string;
  description: string;
  event_types: string[];
  include_notes: boolean;
  inbox_ids: string[] | null;
  api_version: string;
  metadata: Record<string, string>;
  enabled: boolean;
  disabled_reason: string | null;
  disabled_at: string | null;
  health: WebhookHealth;
  failing_since: string | null;
  last_success_at: string | null;
  last_failure_at: string | null;
  previous_secret_expires_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface WebhookEndpointInput {
  url?: string;
  description?: string;
  event_types?: string[];
  include_notes?: boolean;
  inbox_ids?: string[];
  enabled?: boolean;
}

export interface CreatedWebhookEndpoint {
  endpoint: WebhookEndpoint;
  secret: string;
}

export interface WebhookSecret {
  object: "webhook_secret";
  endpoint_id: string;
  secret: string;
  previous_secret: string | null;
  previous_expires_at: string | null;
}

export interface WebhookStats {
  object: "webhook_endpoint_stats";
  attempts: number;
  succeeded: number;
  p50_ms: number;
  p95_ms: number;
}

export type WebhookDeliveryStatus = "pending" | "retrying" | "succeeded" | "failed" | "canceled";

export interface WebhookDelivery {
  id: string;
  object: "webhook_delivery";
  endpoint_id: string;
  event_id: string;
  event_type: string;
  sequence: number;
  payload_bytes: number;
  thin: boolean;
  test: boolean;
  status: WebhookDeliveryStatus;
  attempt_count: number;
  next_attempt_at: string | null;
  last_attempt_at: string | null;
  succeeded_at: string | null;
  last_response_status: number | null;
  last_error_code: string | null;
  created_at: string;
}

export interface WebhookAttempt {
  id: string;
  object: "webhook_delivery_attempt";
  number: number;
  trigger: string;
  requested_at: string;
  duration_ms: number | null;
  response_status: number | null;
  response_body: string | null;
  error_code: string | null;
  error_message: string | null;
  hint?: string;
}

export interface WebhookDeliveryDetail extends WebhookDelivery {
  payload: unknown;
  attempts: WebhookAttempt[];
}

export interface WebhookTestResult {
  object: "webhook_test";
  delivery: WebhookDelivery;
  attempt: WebhookAttempt;
}

const key = (workspace: string) => ["workspace", workspace, "webhooks"] as const;

/*
 * webhooksKey is the root of every webhook query of the workspace, so
 * one change can refresh them all.
 */
export function webhooksKey(workspace: string) {
  return key(workspace);
}

export function webhookEventTypesQuery(workspace: string) {
  return queryOptions({
    queryKey: [...key(workspace), "event-types"],
    queryFn: () => request<WebhookEventType[]>("GET", "/webhook-event-types", undefined, { workspace }),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function webhookEndpointsQuery(workspace: string) {
  return queryOptions({
    queryKey: [...key(workspace), "endpoints"],
    queryFn: () => request<WebhookEndpoint[]>("GET", "/webhook-endpoints", undefined, { workspace }),
  });
}

export function webhookEndpointQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...key(workspace), "endpoints", id],
    queryFn: () =>
      request<WebhookEndpoint>("GET", `/webhook-endpoints/${encodeURIComponent(id)}`, undefined, { workspace }),
  });
}

export function webhookStatsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...key(workspace), "endpoints", id, "stats"],
    queryFn: () =>
      request<WebhookStats>("GET", `/webhook-endpoints/${encodeURIComponent(id)}/stats`, undefined, { workspace }),
  });
}

export function webhookDeliveriesQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...key(workspace), "endpoints", id, "deliveries"],
    queryFn: () =>
      requestPage<WebhookDelivery>(`/webhook-endpoints/${encodeURIComponent(id)}/deliveries?limit=25`, {
        workspace,
      }),
  });
}

export function webhookDeliveryQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...key(workspace), "deliveries", id],
    queryFn: () =>
      request<WebhookDeliveryDetail>("GET", `/webhook-deliveries/${encodeURIComponent(id)}`, undefined, {
        workspace,
      }),
  });
}

export async function createWebhookEndpoint(
  workspace: string,
  input: WebhookEndpointInput,
): Promise<CreatedWebhookEndpoint> {
  const body = await requestBody<{ data: WebhookEndpoint; meta: { secret: string } }>(
    "POST",
    "/webhook-endpoints",
    input,
    { workspace },
  );

  return { endpoint: body.data, secret: body.meta.secret };
}

export function updateWebhookEndpoint(
  workspace: string,
  id: string,
  input: WebhookEndpointInput,
): Promise<WebhookEndpoint> {
  return request<WebhookEndpoint>("PATCH", `/webhook-endpoints/${encodeURIComponent(id)}`, input, { workspace });
}

export function deleteWebhookEndpoint(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/webhook-endpoints/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function revealWebhookSecret(workspace: string, id: string): Promise<WebhookSecret> {
  return request<WebhookSecret>(
    "POST",
    `/webhook-endpoints/${encodeURIComponent(id)}/secret-reveals`,
    {},
    {
      workspace,
    },
  );
}

export function rotateWebhookSecret(workspace: string, id: string, overlapSeconds: number): Promise<WebhookSecret> {
  return request<WebhookSecret>(
    "POST",
    `/webhook-endpoints/${encodeURIComponent(id)}/secret-rotations`,
    { overlap_seconds: overlapSeconds },
    { workspace },
  );
}

export function testWebhookEndpoint(workspace: string, id: string, type?: string): Promise<WebhookTestResult> {
  return request<WebhookTestResult>(
    "POST",
    `/webhook-endpoints/${encodeURIComponent(id)}/tests`,
    type ? { type } : {},
    { workspace },
  );
}

export function redeliverWebhook(workspace: string, id: string): Promise<WebhookDeliveryDetail> {
  return request<WebhookDeliveryDetail>(
    "POST",
    `/webhook-deliveries/${encodeURIComponent(id)}/redeliveries`,
    {},
    {
      workspace,
    },
  );
}
