import { i18n } from ".";
import {
  formatDate,
  formatDateTime,
  formatList,
  formatMonthDay,
  formatNumber,
  formatRelative,
  formatTime,
} from "./format";

const moment = "2026-11-10T15:04:00Z";
const now = Date.parse("2026-11-03T12:00:00Z");

afterEach(async () => {
  await i18n.changeLanguage("en");
});

describe("formatDate", () => {
  it("writes the day in the language and zone given", () => {
    expect(formatDate(moment, { locale: "en-US", timeZone: "UTC" })).toBe("Nov 10, 2026");
    expect(formatDate(moment, { locale: "de", timeZone: "UTC" })).toBe("10.11.2026");
    expect(formatDate("2026-11-10T23:30:00Z", { locale: "en-US", timeZone: "Asia/Tokyo" })).toBe("Nov 11, 2026");
  });

  it("follows the language the app reads in", async () => {
    await i18n.changeLanguage("fr");

    expect(formatDate(moment, { timeZone: "UTC" })).toBe("10 nov. 2026");
  });

  it("writes nothing for a moment that is not one", () => {
    expect(formatDate("soon", { locale: "en-US" })).toBe("");
  });
});

describe("formatMonthDay", () => {
  it("writes the month and day without the year", () => {
    expect(formatMonthDay("2026-12-25", { locale: "en-US", timeZone: "UTC" })).toBe("Dec 25");
    expect(formatMonthDay("2026-12-25", { locale: "de", timeZone: "UTC" })).toBe("25. Dez.");
  });
});

describe("formatTime and formatDateTime", () => {
  it("write the time of day, alone and with its date", () => {
    expect(formatTime(moment, { locale: "en-US", timeZone: "UTC" })).toBe("3:04 PM");
    expect(formatTime(moment, { locale: "en-GB", timeZone: "Europe/Lisbon" })).toBe("15:04");
    expect(formatDateTime(moment, { locale: "en-US", timeZone: "UTC" })).toBe("Nov 10, 2026, 3:04 PM");
  });
});

describe("formatNumber", () => {
  it("uses the reader's separators and the style asked for", () => {
    expect(formatNumber(1234.5, { locale: "en-US" })).toBe("1,234.5");
    expect(formatNumber(1234.5, { locale: "de" })).toBe("1.234,5");
    expect(formatNumber(0.25, { locale: "en-US", style: "percent" })).toBe("25%");
  });
});

describe("formatRelative", () => {
  it("says how long ago in the largest unit that fits", () => {
    expect(formatRelative("2026-11-03T11:45:00Z", { locale: "en", now })).toBe("15 minutes ago");
    expect(formatRelative("2026-11-03T09:00:00Z", { locale: "en", now })).toBe("3 hours ago");
    expect(formatRelative("2026-11-02T12:00:00Z", { locale: "en", now })).toBe("yesterday");
    expect(formatRelative("2026-10-20T12:00:00Z", { locale: "en", now })).toBe("2 weeks ago");
  });

  it("says now under a minute and looks ahead too", () => {
    expect(formatRelative("2026-11-03T11:59:30Z", { locale: "en", now })).toBe("now");
    expect(formatRelative("2026-11-05T12:00:00Z", { locale: "en", now })).toBe("in 2 days");
  });

  it("writes short units when asked", () => {
    expect(formatRelative("2026-11-03T09:00:00Z", { locale: "en", now, style: "short" })).toBe("3 hr. ago");
  });
});

describe("formatList", () => {
  it("joins items the way the reader lists them", () => {
    expect(formatList(["Ada", "Grace", "Alan"], { locale: "en" })).toBe("Ada, Grace, and Alan");
    expect(formatList(["Ada", "Grace"], { locale: "en", type: "disjunction" })).toBe("Ada or Grace");
    expect(formatList(["Ada", "Grace"], { locale: "de" })).toBe("Ada und Grace");
  });
});
