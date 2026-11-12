/*
 * Conversation ratings: whether and how the workspace asks customers to
 * rate a conversation once it closes, how a customer rated one, and the
 * page an email ask links to, which needs no sign-in. Scores run from 1
 * to 5, worst first, each with its face and word.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface RatingSettings {
  object: "conversation_rating_settings";
  enabled: boolean;
  delay_minutes: number;
  include_agent_closed: boolean;
  messenger: boolean;
  email: boolean;
}

export interface ConversationRating {
  id: string;
  object: "conversation_rating";
  conversation_id: string;
  score: number;
  face: string;
  label: string;
  comment: string | null;
  channel: "widget" | "email";
  member: { id: string; name: string } | null;
  asked_at: string | null;
  rated_at: string | null;
}

export interface RatingFace {
  score: number;
  face: string;
  label: string;
}

export interface CustomerRating {
  id: string;
  object: "conversation_rating";
  workspace?: string;
  score: number | null;
  comment: string | null;
  editable: boolean;
  faces: RatingFace[];
}

export const ratingSettingsKey = (workspace: string) => ["workspace", workspace, "rating-settings"] as const;

export function ratingSettingsQuery(workspace: string) {
  return queryOptions({
    queryKey: ratingSettingsKey(workspace),
    queryFn: () => request<RatingSettings>("GET", "/conversation-rating-settings", undefined, { workspace }),
  });
}

export function updateRatingSettings(workspace: string, change: Partial<RatingSettings>): Promise<RatingSettings> {
  return request<RatingSettings>("PATCH", "/conversation-rating-settings", change, { workspace });
}

export const conversationRatingsKey = (workspace: string, conversation: string) =>
  ["workspace", workspace, "conversation", conversation, "ratings"] as const;

export function conversationRatingsQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: conversationRatingsKey(workspace, conversation),
    queryFn: () =>
      request<ConversationRating[]>("GET", `/conversations/${encodeURIComponent(conversation)}/ratings`, undefined, {
        workspace,
      }),
  });
}

const pagePath = (token: string) => `/conversation-ratings/${encodeURIComponent(token)}`;

export function customerRatingQuery(token: string) {
  return queryOptions({
    queryKey: ["conversation-rating", token],
    queryFn: () => request<CustomerRating>("GET", pagePath(token)),
    retry: false,
  });
}

export function rateByToken(token: string, answer: { score?: number; comment?: string }): Promise<CustomerRating> {
  return request<CustomerRating>("PATCH", pagePath(token), answer);
}
