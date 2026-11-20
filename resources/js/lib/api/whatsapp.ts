/*
 * WhatsApp numbers: those the workspace connected, what Meta's signup
 * popup needs to open, an account read with a token before a number is
 * picked, and connecting, moving and disconnecting a number. In the
 * inbox, where a conversation stands on WhatsApp, its templates, and a
 * template sent as a reply.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { Message } from "./inbox";

export interface WhatsAppChannel {
  id: string;
  name: string;
  status: string;
  mode: "shared" | "own";
  inbox_id: string;
  display_phone_number: string;
  verified_name: string;
  name_status: string;
  quality_rating: string;
  messaging_limit: string;
  waba_id: string;
  phone_number_id: string;
  coexistence: boolean;
  webhook: { callback_url: string; verify_token: string } | null;
  last_error: string | null;
  created_at: string;
}

export interface WhatsAppSignupConfig {
  available: boolean;
  app_id: string;
  config_id: string;
  graph_version: string;
}

export interface WhatsAppNumber {
  id: string;
  display_phone_number: string;
  verified_name: string;
  quality_rating: string;
  connected: boolean;
}

export interface WhatsAppAccount {
  id: string;
  name: string;
  business: string;
  numbers: WhatsAppNumber[];
}

export interface WhatsAppVariable {
  part: "header" | "body";
  key: string;
}

export interface WhatsAppTemplate {
  id: string;
  name: string;
  language: string;
  category: "marketing" | "utility" | "authentication";
  status: string;
  sendable: boolean;
  rejected_reason: string | null;
  variables: WhatsAppVariable[];
  preview: string;
  content: WhatsAppTemplateParts;
  /* The detail each variable stands for, keyed "body.1"; empty for templates written in WhatsApp Manager. */
  bindings: Record<string, WhatsAppBinding>;
  synced_at: string;
}

export interface WhatsAppBinding {
  key: string;
  fallback?: string;
}

export type WhatsAppButtonType = "quick_reply" | "url" | "phone_number";

export interface WhatsAppButton {
  type: WhatsAppButtonType;
  text: string;
  url?: string;
  phone_number?: string;
}

/* A template's parts, and the samples Meta reviews its variables with. */
export interface WhatsAppTemplateContent {
  header: string;
  body: string;
  footer: string;
  buttons: WhatsAppButton[];
  samples: Record<string, string>;
}

/* A template's parts as the API returns them. */
export interface WhatsAppTemplateParts extends WhatsAppTemplateContent {
  editable: boolean;
}

export interface WhatsAppTemplateDraft extends WhatsAppTemplateContent {
  name: string;
  language: string;
  category: "utility" | "marketing";
  bindings: Record<string, WhatsAppBinding>;
}

export interface WhatsAppConversation {
  channel_id: string;
  window: { open: boolean; expires_at: string | null };
  contact: { name: string; email: string; phone: string };
  templates: WhatsAppTemplate[];
}

export interface WhatsAppToken {
  access_token: string;
  waba_id: string;
  app_secret?: string;
}

export function whatsappChannelsQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "whatsapp"],
    queryFn: () => request<WhatsAppChannel[]>("GET", "/channels/whatsapp", undefined, { workspace }),
  });
}

export function whatsappSignupConfigQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "whatsapp", "signup-config"],
    queryFn: () => request<WhatsAppSignupConfig>("GET", "/channels/whatsapp/signup-config", undefined, { workspace }),
    staleTime: 5 * 60_000,
  });
}

export function whatsappTemplatesQuery(workspace: string, channel: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "whatsapp", channel, "templates"],
    queryFn: () =>
      request<WhatsAppTemplate[]>("GET", `/channels/whatsapp/${encodeURIComponent(channel)}/templates`, undefined, {
        workspace,
      }),
  });
}

export function whatsappConversationQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "whatsapp", "conversations", conversation],
    queryFn: () =>
      request<WhatsAppConversation>("GET", `/conversations/${encodeURIComponent(conversation)}/whatsapp`, undefined, {
        workspace,
      }),
    refetchInterval: 60_000,
  });
}

/*
 * readWhatsAppAccount reads the account a token can see, with its
 * numbers, before one is picked.
 */
export function readWhatsAppAccount(workspace: string, token: WhatsAppToken): Promise<WhatsAppAccount> {
  return request<WhatsAppAccount>("POST", "/channels/whatsapp/accounts", token, { workspace });
}

/*
 * connectWhatsApp connects the number picked from an account read with
 * a token.
 */
export function connectWhatsApp(
  workspace: string,
  body: WhatsAppToken & { phone_number_id: string; inbox_id?: string },
): Promise<WhatsAppChannel> {
  return request<WhatsAppChannel>("POST", "/channels/whatsapp", body, { workspace });
}

/*
 * finishWhatsAppSignup connects the number Meta's popup handed back.
 */
export function finishWhatsAppSignup(
  workspace: string,
  body: {
    code: string;
    redirect_uri?: string;
    waba_id?: string;
    phone_number_id?: string;
    business_id?: string;
    coexistence?: boolean;
    inbox_id?: string;
  },
): Promise<WhatsAppChannel> {
  return request<WhatsAppChannel>("POST", "/channels/whatsapp/signups", body, { workspace });
}

/*
 * moveWhatsApp sends the number's new conversations to another inbox.
 */
export function moveWhatsApp(workspace: string, channel: string, inbox: string): Promise<WhatsAppChannel> {
  return request<WhatsAppChannel>(
    "PATCH",
    `/channels/whatsapp/${encodeURIComponent(channel)}`,
    { inbox_id: inbox },
    { workspace },
  );
}

/*
 * disconnectWhatsApp stops the number feeding the workspace.
 */
export function disconnectWhatsApp(workspace: string, channel: string): Promise<void> {
  return request<void>("DELETE", `/channels/whatsapp/${encodeURIComponent(channel)}`, undefined, { workspace });
}

/*
 * refreshWhatsAppTemplates takes a new copy of the number's templates
 * from Meta.
 */
export function refreshWhatsAppTemplates(workspace: string, channel: string): Promise<WhatsAppTemplate[]> {
  return request<WhatsAppTemplate[]>(
    "GET",
    `/channels/whatsapp/${encodeURIComponent(channel)}/templates?refresh=true`,
    undefined,
    { workspace },
  );
}

/*
 * sendWhatsAppTemplate sends a template as a reply to the conversation.
 */
export function sendWhatsAppTemplate(
  workspace: string,
  conversation: string,
  body: { name: string; language: string; values: Record<string, string>; client_message_id?: string },
): Promise<Message> {
  return request<Message>("POST", `/conversations/${encodeURIComponent(conversation)}/whatsapp-templates`, body, {
    workspace,
  });
}

/*
 * createWhatsAppTemplate sends a new template to Meta for review.
 */
export function createWhatsAppTemplate(
  workspace: string,
  channel: string,
  draft: WhatsAppTemplateDraft,
): Promise<WhatsAppTemplate> {
  return request<WhatsAppTemplate>("POST", `/channels/whatsapp/${encodeURIComponent(channel)}/templates`, draft, {
    workspace,
  });
}

/*
 * updateWhatsAppTemplate sends a template's new content back to Meta's
 * review; its name and language stay.
 */
export function updateWhatsAppTemplate(
  workspace: string,
  channel: string,
  template: string,
  draft: WhatsAppTemplateDraft,
): Promise<WhatsAppTemplate> {
  const { name: _name, language: _language, ...content } = draft;
  return request<WhatsAppTemplate>(
    "PUT",
    `/channels/whatsapp/${encodeURIComponent(channel)}/templates/${encodeURIComponent(template)}`,
    content,
    { workspace },
  );
}

/*
 * deleteWhatsAppTemplate removes a template from Meta.
 */
export function deleteWhatsAppTemplate(workspace: string, channel: string, template: string): Promise<void> {
  return request<void>(
    "DELETE",
    `/channels/whatsapp/${encodeURIComponent(channel)}/templates/${encodeURIComponent(template)}`,
    undefined,
    { workspace },
  );
}
