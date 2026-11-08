/*
 * How office hours read and change on the settings page: whether the
 * team is in now and when that changes, in the schedule's own time
 * zone, and one weekday's hours set or closed.
 */
import type { Schedule, Weekday, Weekly } from "@/lib/api/schedules";

/*
 * statusLine says whether the team is in now and when that changes, in
 * the schedule's own time zone.
 */
export function statusLine(schedule: Schedule): string {
  const time = (iso: string) =>
    new Intl.DateTimeFormat("en", {
      weekday: "short",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
      timeZone: schedule.timezone,
    }).format(new Date(iso));
  const { open, next_open: nextOpen, next_close: nextClose } = schedule.status;
  if (open) {
    return nextClose ? `Open now, closes ${time(nextClose)}` : "Open now";
  }

  return nextOpen ? `Closed now, opens ${time(nextOpen)}` : "Never open";
}

/*
 * withDay sets one weekday's hours: one range, or closed when null.
 */
export function withDay(weekly: Weekly, day: Weekday, range: [string, string] | null): Weekly {
  const next = { ...weekly };
  if (range) {
    next[day] = [range, ...(weekly[day] ?? []).slice(1)];
  } else {
    delete next[day];
  }

  return next;
}

/*
 * sameWeek reports whether two weeks have the same hours, whatever order
 * their days are listed in.
 */
export function sameWeek(a: Weekly, b: Weekly): boolean {
  const days: Weekday[] = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];

  return days.every((day) => JSON.stringify(a[day] ?? []) === JSON.stringify(b[day] ?? []));
}
