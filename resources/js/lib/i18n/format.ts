/*
 * Dates, times, numbers and lists as the reader writes them: in the
 * language the app reads in and, for moments, in the time zone the
 * person chose, else their browser's. Every screen formats through these
 * so a change of language changes every date with it. Formatters are
 * costly to build, so each kind is built once per language and zone.
 */
import { activeLocale } from ".";

/*
 * FormatOptions overrides the active language or names the time zone a
 * moment shows in; left out, the zone is the browser's.
 */
export interface FormatOptions {
  locale?: string;
  timeZone?: string;
}

/*
 * Moment is a point in time as the API sends it, or as the browser holds
 * it.
 */
export type Moment = Date | string | number;

const formatters = new Map<
  string,
  Intl.DateTimeFormat | Intl.NumberFormat | Intl.RelativeTimeFormat | Intl.ListFormat
>();

/*
 * cached returns the formatter built for the same kind and options, or
 * builds and keeps it.
 */
function cached<T extends Intl.DateTimeFormat | Intl.NumberFormat | Intl.RelativeTimeFormat | Intl.ListFormat>(
  kind: string,
  options: object,
  build: () => T,
): T {
  const key = `${kind}:${JSON.stringify(options)}`;
  const found = formatters.get(key);
  if (found) {
    return found as T;
  }

  const made = build();
  formatters.set(key, made);

  return made;
}

function toDate(moment: Moment): Date {
  return moment instanceof Date ? moment : new Date(moment);
}

function moment(value: Moment, options: FormatOptions, style: Intl.DateTimeFormatOptions): string {
  const date = toDate(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const locale = options.locale ?? activeLocale();
  const settings = { ...style, timeZone: options.timeZone };

  return cached("date", { locale, ...settings }, () => new Intl.DateTimeFormat(locale, settings)).format(date);
}

/*
 * formatDate writes the day of a moment, such as "Nov 10, 2026", or ""
 * for a moment that is not one.
 */
export function formatDate(value: Moment, options: FormatOptions = {}): string {
  return moment(value, options, { dateStyle: "medium" });
}

/*
 * formatTime writes the time of day of a moment, such as "3:04 PM".
 */
export function formatTime(value: Moment, options: FormatOptions = {}): string {
  return moment(value, options, { timeStyle: "short" });
}

/*
 * formatDateTime writes the day and the time of a moment.
 */
export function formatDateTime(value: Moment, options: FormatOptions = {}): string {
  return moment(value, options, { dateStyle: "medium", timeStyle: "short" });
}

/*
 * formatNumber writes a number with the reader's separators, such as
 * "1,234.5", or in the style given, such as a percentage.
 */
export function formatNumber(value: number, options: { locale?: string } & Intl.NumberFormatOptions = {}): string {
  const { locale = activeLocale(), ...style } = options;

  return cached("number", { locale, ...style }, () => new Intl.NumberFormat(locale, style)).format(value);
}

/*
 * RelativeOptions sets the moment "now" is measured from and how long
 * the words are.
 */
export interface RelativeOptions {
  locale?: string;
  now?: number;
  style?: Intl.RelativeTimeFormatStyle;
}

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 31_536_000_000],
  ["month", 2_592_000_000],
  ["week", 604_800_000],
  ["day", 86_400_000],
  ["hour", 3_600_000],
  ["minute", 60_000],
];

/*
 * formatRelative says how long ago, or how far ahead, a moment is, in
 * the largest unit that fits, such as "3 hours ago" or "yesterday";
 * under a minute it says "now".
 */
export function formatRelative(value: Moment, options: RelativeOptions = {}): string {
  const { locale = activeLocale(), now = Date.now(), style = "long" } = options;
  const format = cached(
    "relative",
    { locale, style },
    () => new Intl.RelativeTimeFormat(locale, { numeric: "auto", style }),
  );
  const elapsed = toDate(value).getTime() - now;

  for (const [unit, size] of units) {
    if (Math.abs(elapsed) >= size) {
      return format.format(Math.trunc(elapsed / size), unit);
    }
  }

  return format.format(0, "second");
}

/*
 * formatList joins items the way the reader lists them, such as "Ada,
 * Grace, and Alan", or "Ada or Grace" as a disjunction.
 */
export function formatList(items: string[], options: { locale?: string; type?: Intl.ListFormatType } = {}): string {
  const { locale = activeLocale(), type = "conjunction" } = options;

  return cached("list", { locale, type }, () => new Intl.ListFormat(locale, { type })).format(items);
}
