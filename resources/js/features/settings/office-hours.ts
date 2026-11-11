/*
 * How office hours read and change on the settings page: whether the
 * team is in now and when that changes, in the schedule's own time
 * zone, one weekday's hours set or closed, and when a holiday falls.
 */
import type { Holiday, Schedule, Weekday, Weekly } from "@/lib/api/schedules";
import { formatDate, formatMonthDay } from "@/lib/i18n/format";

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

/*
 * holidayWhen says when a holiday falls, such as "Dec 25, every year"
 * or "Mar 3, 2027". The date has no time of day, so it is read in UTC
 * to keep the reader's own zone from moving it a day.
 */
export function holidayWhen(holiday: Holiday): string {
  if (holiday.yearly) {
    return `${formatMonthDay(holiday.date, { timeZone: "UTC" })}, every year`;
  }

  return formatDate(holiday.date, { timeZone: "UTC" });
}
