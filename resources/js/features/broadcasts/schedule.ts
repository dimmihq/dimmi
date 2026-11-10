/*
 * When a scheduled broadcast goes out, as the date and time pickers
 * hold it and as the server stores it. A send at a set time is read in
 * the teammate's own zone. A send in each person's own zone is a wall
 * clock time the server reads in the workspace's zone and then moves to
 * each person's, so the composer writes it in the workspace's zone too.
 */
import {
  type CalendarDate,
  CalendarDateTime,
  getLocalTimeZone,
  parseAbsolute,
  Time,
  toCalendarDate,
  today,
  toTime,
  toZoned,
} from "@internationalized/date";

import type { ScheduleKind } from "@/lib/api/broadcasts";

export interface WallTime {
  day: CalendarDate;
  time: Time;
}

export function zoneFor(kind: ScheduleKind, workspaceZone: string | undefined): string {
  return kind === "local" ? (workspaceZone ?? getLocalTimeZone()) : getLocalTimeZone();
}

/*
 * wallOf reads a stored time as the day and time the pickers show,
 * tomorrow at nine when there is none yet.
 */
export function wallOf(at: string | null, zone: string): WallTime {
  if (!at) {
    return { day: today(zone).add({ days: 1 }), time: new Time(9, 0) };
  }
  const zoned = parseAbsolute(at, zone);

  return { day: toCalendarDate(zoned), time: toTime(zoned) };
}

/*
 * instantOf writes the day and time in the zone as the instant the
 * server stores.
 */
export function instantOf(wall: WallTime, zone: string): string {
  const { day, time } = wall;
  const local = new CalendarDateTime(day.year, day.month, day.day, time.hour, time.minute);

  return toZoned(local, zone).toAbsoluteString();
}
