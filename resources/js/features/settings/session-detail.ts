/*
 * The line under a session in the list: whether it is the one in use or
 * how long ago it was last used, and where from.
 */
import type { Session } from "@/lib/api/sessions";

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["day", 86_400_000],
  ["hour", 3_600_000],
  ["minute", 60_000],
];

const relative = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

/*
 * lastActive says how long ago the session was used, in the largest
 * unit that fits.
 */
export function lastActive(at: string, now = Date.now()): string {
  const elapsed = now - new Date(at).getTime();

  for (const [unit, size] of units) {
    if (elapsed >= size) {
      return `Last active ${relative.format(-Math.floor(elapsed / size), unit)}`;
    }
  }

  return "Active just now";
}

export function sessionDetail(session: Session, now = Date.now()): string {
  const seen = session.current ? "This session" : lastActive(session.last_activity_at, now);
  const where = [session.ip, session.country].filter(Boolean).join(" · ");

  return where ? `${seen} · ${where}` : seen;
}
