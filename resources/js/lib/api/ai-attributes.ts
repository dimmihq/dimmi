/*
 * AI conversation attributes: fields a workspace defines once (a key, a
 * name, a type, what it means and, for a choice, its values) that the
 * decision model fills in on every conversation and checks again as the
 * customer writes. Workflows branch and start on them and inbox views
 * filter by them as conversation.ai.<key>. A key and a type never change;
 * an attribute is archived rather than deleted. Reading takes
 * conversations:read; changing takes ai:write, which the server checks.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type AIAttributeType = "text" | "number" | "date" | "boolean" | "choice";

export interface AIAttribute {
  id: string;
  object: "ai_conversation_attribute";
  key: string;
  label: string;
  description: string;
  data_type: AIAttributeType;
  options: string[];
  status: "active" | "archived";
  created_at: string;
  updated_at: string;
}

export interface AIAttributeInput {
  key: string;
  label: string;
  description?: string;
  data_type: AIAttributeType;
  options?: string[];
}

export interface AIAttributeChange {
  label?: string;
  description?: string;
  options?: string[];
  status?: "active" | "archived";
}

export interface ConversationAIAttributes {
  object: "conversation_ai_attributes";
  values: { object: "ai_attribute_value"; attribute: AIAttribute; value: unknown }[];
  checked_at: string | null;
}

export function aiAttributesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "ai-conversation-attributes"],
    queryFn: () => request<AIAttribute[]>("GET", "/ai/conversation-attributes", undefined, { workspace }),
  });
}

export function conversationAIAttributesQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "conversations", conversation, "ai-attributes"],
    queryFn: () =>
      request<ConversationAIAttributes>("GET", `/conversations/${conversation}/ai-attributes`, undefined, {
        workspace,
      }),
  });
}

export function createAIAttribute(workspace: string, body: AIAttributeInput): Promise<AIAttribute> {
  return request<AIAttribute>("POST", "/ai/conversation-attributes", body, { workspace });
}

export function updateAIAttribute(workspace: string, id: string, body: AIAttributeChange): Promise<AIAttribute> {
  return request<AIAttribute>("PATCH", `/ai/conversation-attributes/${id}`, body, { workspace });
}

export const aiAttributeTypes: Record<AIAttributeType, string> = {
  text: "Text",
  number: "Number",
  date: "Date",
  boolean: "Yes or no",
  choice: "One of a list",
};
