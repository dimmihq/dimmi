/*
 * Conversation attributes: fields a workspace defines once (a key, a
 * name, a type, what it means and, for a list, its values) that every
 * conversation can hold. Teammates fill them in the conversation's
 * details, the AI fills the ones it may as the customer writes, or both;
 * a value a teammate set is never replaced by the AI. One may be
 * required before a teammate closes a conversation, and shown only to
 * some teams. Workflows branch and start on them and inbox views filter
 * by them as conversation.ai.<key>; reports filter and split by them.
 * The details panel stays current through the
 * conversation.attribute_changed broadcast. A key and a type never
 * change; an attribute is archived rather than deleted.
 */
import { queryOptions } from "@tanstack/react-query";

import { realtimeKeys } from "@/lib/realtime/keys";

import { request } from "./client";
import type { Attachment } from "./inbox";

export type ConversationAttributeType =
  | "text"
  | "long_text"
  | "number"
  | "decimal"
  | "choice"
  | "multiple_choice"
  | "boolean"
  | "date"
  | "datetime"
  | "file"
  | "contact"
  | "company";

export type FilledBy = "ai" | "teammates" | "both";

export interface ConversationAttribute {
  id: string;
  object: "conversation_attribute";
  key: string;
  label: string;
  description: string;
  data_type: ConversationAttributeType;
  options: string[];
  option_descriptions: Record<string, string>;
  filled_by: FilledBy;
  required_to_close: boolean;
  team_ids: string[];
  status: "active" | "archived";
  created_at: string;
  updated_at: string;
}

export interface ConversationAttributeInput {
  key: string;
  label: string;
  description?: string;
  data_type: ConversationAttributeType;
  options?: string[];
  option_descriptions?: Record<string, string>;
  filled_by?: FilledBy;
  required_to_close?: boolean;
  team_ids?: string[];
}

export interface ConversationAttributeChange {
  label?: string;
  description?: string;
  options?: string[];
  option_descriptions?: Record<string, string>;
  status?: "active" | "archived";
  filled_by?: FilledBy;
  required_to_close?: boolean;
  team_ids?: string[];
}

export interface AttributeSource {
  by: "ai" | "member" | "workflow" | "import";
  id: string | null;
  name: string | null;
  at: string;
  /* Why the AI chose the value, and how sure it was from 0 to 1. */
  reason: string | null;
  sureness: number | null;
}

export interface ConversationAttributeValue {
  object: "conversation_attribute_value";
  attribute: ConversationAttribute;
  value: unknown;
  set_by: AttributeSource | null;
  files: Attachment[];
}

export interface ConversationAttributeValues {
  object: "conversation_attribute_values";
  values: ConversationAttributeValue[];
  checked_at: string | null;
}

export function conversationAttributesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversation-attributes"],
    queryFn: () => request<ConversationAttribute[]>("GET", "/conversation-attributes", undefined, { workspace }),
  });
}

export function conversationAttributeValuesQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: realtimeKeys.aiAttributes(workspace, conversation),
    queryFn: () =>
      request<ConversationAttributeValues>("GET", `/conversations/${conversation}/attributes`, undefined, {
        workspace,
      }),
  });
}

export function createConversationAttribute(
  workspace: string,
  body: ConversationAttributeInput,
): Promise<ConversationAttribute> {
  return request<ConversationAttribute>("POST", "/conversation-attributes", body, { workspace });
}

export function updateConversationAttribute(
  workspace: string,
  id: string,
  body: ConversationAttributeChange,
): Promise<ConversationAttribute> {
  return request<ConversationAttribute>("PATCH", `/conversation-attributes/${id}`, body, { workspace });
}

export function reorderConversationAttributes(workspace: string, ids: string[]): Promise<ConversationAttribute[]> {
  return request<ConversationAttribute[]>("PATCH", "/conversation-attributes/order", { ids }, { workspace });
}

export function setConversationAttributeValues(
  workspace: string,
  conversation: string,
  values: Record<string, unknown>,
): Promise<ConversationAttributeValues> {
  return request<ConversationAttributeValues>(
    "PATCH",
    `/conversations/${conversation}/attributes`,
    { values },
    { workspace },
  );
}

export const conversationAttributeTypes: Record<ConversationAttributeType, string> = {
  text: "Text",
  long_text: "Multi-line text",
  number: "Number",
  decimal: "Decimal",
  choice: "List",
  multiple_choice: "Multi-select",
  boolean: "Yes or no",
  date: "Date",
  datetime: "Date and time",
  file: "Files",
  contact: "Contact",
  company: "Company",
};

/*
 * aiFillable says whether the AI can fill an attribute of the type: text,
 * numbers, yes or no, a date and one of a list.
 */
export function aiFillable(type: ConversationAttributeType): boolean {
  return ["text", "long_text", "number", "decimal", "choice", "boolean", "date"].includes(type);
}

export const filledByLabels: Record<FilledBy, string> = {
  both: "AI and teammates",
  teammates: "Teammates only",
  ai: "AI only",
};

export interface ConversationAttributeUsage {
  object: "conversation_attribute_usage";
  conversations: number;
  workflows: { id: string; name: string }[];
  views: { id: string; name: string }[];
  deletable: boolean;
}

export function conversationAttributeUsageQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversation-attributes", id, "usage"],
    queryFn: () =>
      request<ConversationAttributeUsage>("GET", `/conversation-attributes/${id}/usage`, undefined, { workspace }),
  });
}

export function deleteConversationAttribute(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/conversation-attributes/${id}`, undefined, { workspace });
}
