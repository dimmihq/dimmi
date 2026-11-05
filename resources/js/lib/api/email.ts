/*
 * The workspace's email addresses: the queries that read them and what
 * arrived at them, and the calls that connect one, change it, send a
 * forwarding test or disconnect it. Every call names the workspace in
 * the Dimmi-Workspace header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface SetupGuide {
  key: string;
  provider: string;
  name: string;
  recommended: boolean;
  deep_link: string | null;
  steps: string[];
  warnings: string[];
}

export interface ForwardingConfirmation {
  provider: string;
  code: string;
  confirm_url: string;
  received_at: string;
}

export interface EmailAddress {
  id: string;
  address: string;
  display_name: string;
  forwarding_address: string;
  is_default: boolean;
  inbox: { id: string; name: string };
  reply_to_mode: "token" | "none";
  from_name_style: "member_and_name" | "name_only";
  sending_identity: { from: string; uses_default_sender: boolean; provider: string };
  settings: { quote_previous: boolean; bulk_mail_handling: "close" | "open" | "drop" };
  forwarding_confirmation: ForwardingConfirmation | null;
  inbound_verified_at: string | null;
  last_inbound_at: string | null;
  setup_guides: SetupGuide[];
}

export interface InboundEmail {
  id: string;
  status: string;
  classification: string | null;
  ignore_reason: string | null;
  from: string | null;
  subject: string | null;
  received_at: string;
}

export interface EmailAddressChanges {
  display_name?: string;
  reply_to_mode?: EmailAddress["reply_to_mode"];
  from_name_style?: EmailAddress["from_name_style"];
  settings?: Partial<EmailAddress["settings"]>;
}

export function emailAddressesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "email-addresses"],
    queryFn: () => request<EmailAddress[]>("GET", "/email-addresses", undefined, { workspace }),
  });
}

export function inboundEmailsQuery(workspace: string, address: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "email-addresses", address, "inbound-emails"],
    queryFn: () =>
      request<InboundEmail[]>(
        "GET",
        `/email-addresses/${encodeURIComponent(address)}/inbound-emails?limit=5`,
        undefined,
        { workspace },
      ),
  });
}

export function createEmailAddress(workspace: string, address: string): Promise<EmailAddress> {
  return request<EmailAddress>("POST", "/email-addresses", { address }, { workspace });
}

export function updateEmailAddress(
  workspace: string,
  address: string,
  changes: EmailAddressChanges,
): Promise<EmailAddress> {
  return request<EmailAddress>("PATCH", `/email-addresses/${encodeURIComponent(address)}`, changes, { workspace });
}

export function deleteEmailAddress(workspace: string, address: string): Promise<void> {
  return request<void>("DELETE", `/email-addresses/${encodeURIComponent(address)}`, undefined, { workspace });
}

export interface TestFinding {
  code: string;
  severity: "error" | "warn" | "info";
  message: string;
}

export interface EmailAddressTest {
  id: string;
  email_address: string;
  status: "sending" | "waiting" | "received" | "failed" | "timed_out";
  sent_to: string;
  message_id: string;
  findings: TestFinding[];
  latency_ms: number | null;
  expires_at: string;
  completed_at: string | null;
  created_at: string;
}

export function sendEmailAddressTest(workspace: string, address: string): Promise<EmailAddressTest> {
  return request<EmailAddressTest>("POST", `/email-addresses/${encodeURIComponent(address)}/tests`, undefined, {
    workspace,
  });
}

export function emailAddressTestQuery(workspace: string, address: string, test: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "email-addresses", address, "tests", test],
    queryFn: () =>
      request<EmailAddressTest>(
        "GET",
        `/email-addresses/${encodeURIComponent(address)}/tests/${encodeURIComponent(test)}`,
        undefined,
        { workspace },
      ),
  });
}
