/*
 * When a routine runs, as the builder shows it: a kind (daily, weekly,
 * monthly, once or a custom cron), a wall time, a weekday or a day of
 * the month, and a time zone. Daily, weekly and monthly routines are
 * stored as the cron expression the server reads, so a saved routine is
 * read back into the same picks. A monthly routine runs on a day from 1
 * to 28, or on the month's last day, since days 29 to 31 do not exist
 * every month.
 */
import type { RoutineCadence } from "@/lib/api/routines";

export interface Timing {
  kind: RoutineCadence;
  hour: number;
  minute: number;
  /* 0 is Sunday. */
  weekday: number;
  /* 1 to 28, or 0 for the month's last day. */
  monthDay: number;
  cron: string;
  /* A one-off run's local date and time, as YYYY-MM-DDTHH:MM. */
  runAt: string;
  timezone: string;
}

export const WEEKDAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

/*
 * cronOf is the cron expression the timing stores, empty for a one-off.
 */
export function cronOf(t: Timing): string {
  switch (t.kind) {
    case "daily":
      return `${t.minute} ${t.hour} * * *`;
    case "weekly":
      return `${t.minute} ${t.hour} * * ${t.weekday}`;
    case "monthly":
      return `${t.minute} ${t.hour} ${t.monthDay === 0 ? "L" : t.monthDay} * *`;
    case "cron":
      return t.cron.trim();
    default:
      return "";
  }
}

/*
 * timingOf reads a stored cadence back into the builder's picks. A cron
 * the presets cannot show stays a custom cron.
 */
export function timingOf(kind: RoutineCadence, cron: string | null, runAt: string | null, timezone: string): Timing {
  const t: Timing = { kind, hour: 9, minute: 0, weekday: 1, monthDay: 1, cron: cron ?? "", runAt: "", timezone };
  if (kind === "once") {
    t.runAt = runAt ? localInput(runAt, timezone) : "";
    return t;
  }
  const fields = (cron ?? "").trim().split(/\s+/);
  const [minute, hour, day, month, weekday] = fields;
  const num = (v: string | undefined, lo: number, hi: number) =>
    v !== undefined && /^\d+$/.test(v) && Number(v) >= lo && Number(v) <= hi ? Number(v) : null;
  const m = num(minute, 0, 59);
  const h = num(hour, 0, 23);
  if (fields.length !== 5 || m === null || h === null || month !== "*") {
    return { ...t, kind: "cron" };
  }
  t.minute = m;
  t.hour = h;
  if (kind === "daily" && day === "*" && weekday === "*") {
    return t;
  }
  const wd = num(weekday, 0, 6);
  if (kind === "weekly" && day === "*" && wd !== null) {
    return { ...t, weekday: wd };
  }
  const md = day === "L" ? 0 : num(day, 1, 28);
  if (kind === "monthly" && weekday === "*" && md !== null) {
    return { ...t, monthDay: md };
  }
  return { ...t, kind: "cron" };
}

/*
 * clock writes a wall time as HH:MM.
 */
export function clock(hour: number, minute: number): string {
  return `${String(hour).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
}

/*
 * ordinal writes a day of the month: 1st, 2nd, 23rd.
 */
export function ordinal(day: number): string {
  const tens = day % 100;
  if (tens >= 11 && tens <= 13) {
    return `${day}th`;
  }
  return `${day}${{ 1: "st", 2: "nd", 3: "rd" }[day % 10] ?? "th"}`;
}

/*
 * summarize says in a few words when the routine runs.
 */
export function summarize(t: Timing): string {
  const at = clock(t.hour, t.minute);
  switch (t.kind) {
    case "daily":
      return `Every day at ${at}`;
    case "weekly":
      return `Every ${WEEKDAYS[t.weekday]} at ${at}`;
    case "monthly":
      return t.monthDay === 0
        ? `On the last day of each month at ${at}`
        : `On the ${ordinal(t.monthDay)} of each month at ${at}`;
    case "once":
      return t.runAt ? `Once, on ${t.runAt.replace("T", " at ")}` : "Once";
    default:
      return t.cron.trim() ? `On the cron ${t.cron.trim()}` : "On a custom cron";
  }
}

/*
 * describeRoutine says when a saved routine runs, in its own zone.
 */
export function describeRoutine(r: {
  cadence: RoutineCadence;
  cron_expression: string | null;
  run_at: string | null;
  timezone: string;
}): string {
  return summarize(timingOf(r.cadence, r.cron_expression, r.run_at, r.timezone));
}

/*
 * localInput writes a moment as the YYYY-MM-DDTHH:MM a date-time input
 * shows, on the wall clock of the zone.
 */
export function localInput(iso: string, timezone: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).formatToParts(new Date(iso));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "00";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get("minute")}`;
}

/*
 * instantOf reads a wall time in the zone as a moment, for a one-off
 * run: the offset the zone has then is found by formatting a first
 * guess back on its wall clock.
 */
export function instantOf(local: string, timezone: string): string {
  const guess = new Date(`${local}:00Z`);
  const shown = new Date(`${localInput(guess.toISOString(), timezone)}:00Z`);
  return new Date(guess.getTime() - (shown.getTime() - guess.getTime())).toISOString();
}
