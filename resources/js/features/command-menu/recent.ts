/*
 * The moves a person ran from the command menu lately, kept in their
 * browser so they come first next time. Storage may be missing, full or
 * blocked; the menu then simply keeps its usual order.
 */
import type { InboxAction } from "@/features/inbox/actions/registry";

const key = "dimmi:command-menu:recent";
const kept = 5;

function read(): string[] {
  try {
    const raw = window.localStorage.getItem(key);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === "string") : [];
  } catch {
    return [];
  }
}

/*
 * remember puts the move first among the recent ones.
 */
export function remember(id: string) {
  try {
    window.localStorage.setItem(key, JSON.stringify([id, ...read().filter((r) => r !== id)].slice(0, kept)));
  } catch {
    return;
  }
}

/*
 * recentFirst orders the moves with the recent ones first.
 */
export function recentFirst(actions: InboxAction[]): InboxAction[] {
  const recent = read();
  const rank = (a: InboxAction) => {
    const at = recent.indexOf(a.id);
    return at === -1 ? recent.length : at;
  };

  return [...actions].sort((a, b) => rank(a) - rank(b));
}
