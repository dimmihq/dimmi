/*
 * The person's drafts, kept on the server so a half written reply
 * follows them to another device. Each save sends the draft whole with
 * the moment it was last changed; the server keeps the newest and
 * answers a stale save with what it holds.
 */
import { queryOptions } from "@tanstack/react-query";

import { realtimeKeys } from "@/lib/realtime/keys";

import { request } from "./client";

export type DraftMode = "reply" | "note" | "new";

export interface Draft {
  id: string;
  conversation_id: string | null;
  mode: DraftMode;
  body: string;
  attachment_ids: string[];
  client_updated_at: string;
}

export interface DraftSave {
  conversation_id: string | null;
  mode: DraftMode;
  body: string;
  client_updated_at: string;
}

export function draftsQuery(workspace: string) {
  return queryOptions({
    queryKey: realtimeKeys.drafts(workspace),
    queryFn: () => request<Draft[]>("GET", "/drafts", undefined, { workspace }),
    staleTime: 30_000,
  });
}

/*
 * saveDraft writes the draft, starting it or updating the one there.
 */
export function saveDraft(
  workspace: string,
  draft: DraftSave,
  options: { keepalive?: boolean; signal?: AbortSignal } = {},
): Promise<Draft> {
  const body = { ...draft, conversation_id: draft.conversation_id ?? undefined };

  return request<Draft>("POST", "/drafts?upsert=true", body, { workspace, ...options });
}

export function deleteDraft(workspace: string, id: string, options: { keepalive?: boolean } = {}): Promise<void> {
  return request<void>("DELETE", `/drafts/${encodeURIComponent(id)}`, undefined, { workspace, ...options });
}
