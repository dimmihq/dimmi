/*
 * The workspace's email addresses: the queries that read them and what
 * arrived at them, and the calls that connect one, change it, send a
 * forwarding test or disconnect it. Every call names the workspace in
 * the Dimmi-Workspace header.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface SetupGuide {
  provider: string;
  name: string;
  steps: string[];
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

export function sendEmailAddressTest(workspace: string, address: string): Promise<{ sent_to: string }> {
  return request<{ sent_to: string }>("POST", `/email-addresses/${encodeURIComponent(address)}/tests`, undefined, {
    workspace,
  });
}
