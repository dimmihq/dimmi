/*
 * The line under a session in the list: whether it is the one in use or
 * how long ago it was last used, and where from.
 */
import type { Session } from "@/lib/api/sessions";
import { i18n } from "@/lib/i18n";
import { formatRelative } from "@/lib/i18n/format";

/*
 * lastActive says how long ago the session was used, in the largest
 * unit that fits, or that it is in use right now.
 */
export function lastActive(at: string, now = Date.now()): string {
  if (now - new Date(at).getTime() < 60_000) {
    return i18n.t("settings:sessions.activeJustNow");
  }

  return i18n.t("settings:sessions.lastActive", { time: formatRelative(at, { now }) });
}

export function sessionDetail(session: Session, now = Date.now()): string {
  const seen = session.current ? i18n.t("settings:sessions.current") : lastActive(session.last_activity_at, now);
  const where = [session.ip, session.country].filter(Boolean).join(" · ");

  return where ? `${seen} · ${where}` : seen;
}
