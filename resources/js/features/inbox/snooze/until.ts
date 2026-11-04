/*
 * How the snooze picker reads and writes moments: the exact time a
 * choice ends in the reader's words, and a picked day and time as the
 * moment the API expects.
 */
import { type CalendarDate, getLocalTimeZone, toZoned } from "@internationalized/date";

/*
 * untilLabel says when a snooze ends: the weekday and time, in the
 * reader's language and zone.
 */
export function untilLabel(iso: string, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: "short", hour: "numeric", minute: "2-digit" }).format(
    new Date(iso),
  );
}

/*
 * untilOf returns the picked day at the time ("09:30") in the reader's
 * zone, as an ISO moment, or null for a time that does not read.
 */
export function untilOf(day: CalendarDate, time: string, zone = getLocalTimeZone()): string | null {
  const match = /^(\d{2}):(\d{2})$/.exec(time);
  if (!match) {
    return null;
  }

  return toZoned(day, zone)
    .set({ hour: Number(match[1]), minute: Number(match[2]), second: 0 })
    .toDate()
    .toISOString();
}
