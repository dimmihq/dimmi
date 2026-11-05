/*
 * The browser's own copy of each draft, so typing survives a reload even
 * before the server has it. Storage may be blocked or full; then the copy
 * is simply missing and the server's draft is all there is.
 */
import type { Draft, DraftMode } from "@/lib/api/drafts";

/*
 * Copy is a draft as the browser keeps it: the text and when it was last
 * changed, as an ISO time.
 */
export interface Copy {
  body: string;
  at: string;
}

export function copyKey(workspace: string, conversation: string | null, mode: DraftMode): string {
  return `dimmi:draft:${workspace}:${conversation ?? "new"}:${mode}`;
}

export function readCopy(key: string): Copy | null {
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<Copy>;

    return typeof parsed.body === "string" && typeof parsed.at === "string"
      ? { body: parsed.body, at: parsed.at }
      : null;
  } catch {
    return null;
  }
}

export function writeCopy(key: string, copy: Copy | null) {
  try {
    if (copy === null || copy.body === "") {
      window.localStorage.removeItem(key);
    } else {
      window.localStorage.setItem(key, JSON.stringify(copy));
    }
  } catch {
    return;
  }
}

/*
 * newest picks the later of the browser's copy and the server's draft,
 * the copy when both were changed at the same moment.
 */
export function newest(copy: Copy | null, stored: Draft | undefined): Copy {
  const fromServer = stored ? { body: stored.body, at: stored.client_updated_at } : null;
  if (!copy) {
    return fromServer ?? { body: "", at: new Date(0).toISOString() };
  }
  if (!fromServer) {
    return copy;
  }

  return Date.parse(fromServer.at) > Date.parse(copy.at) ? fromServer : copy;
}

/*
 * findDraft returns the server's draft of the conversation and mode.
 */
export function findDraft(drafts: Draft[] | undefined, conversation: string | null, mode: DraftMode) {
  return drafts?.find((d) => d.conversation_id === conversation && d.mode === mode);
}
