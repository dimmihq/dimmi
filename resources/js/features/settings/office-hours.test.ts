import type { Schedule } from "@/lib/api/schedules";

import { holidayWhen, sameWeek, statusLine, withDay } from "./office-hours";

const schedule = (status: Schedule["status"]): Schedule => ({
  id: "sch_1",
  name: "Office hours",
  timezone: "Europe/Berlin",
  is_default: true,
  weekly: {},
  holidays: [],
  status,
});

describe("statusLine", () => {
  it("says when an open schedule closes, in its own time zone", () => {
    expect(statusLine(schedule({ open: true, next_open: null, next_close: "2026-11-04T16:30:00Z" }))).toBe(
      "Open now, closes Wed 17:30",
    );
  });

  it("says when a closed schedule opens, or that it never does", () => {
    expect(statusLine(schedule({ open: false, next_open: "2026-11-09T08:00:00Z", next_close: null }))).toBe(
      "Closed now, opens Mon 09:00",
    );
    expect(statusLine(schedule({ open: false, next_open: null, next_close: null }))).toBe("Never open");
  });
});

describe("withDay", () => {
  it("sets a day's first range and keeps the others", () => {
    expect(
      withDay(
        {
          mon: [
            ["09:00", "12:00"],
            ["13:00", "17:00"],
          ],
        },
        "mon",
        ["08:00", "12:00"],
      ),
    ).toEqual({
      mon: [
        ["08:00", "12:00"],
        ["13:00", "17:00"],
      ],
    });
  });

  it("closes a day", () => {
    expect(withDay({ mon: [["09:00", "17:00"]] }, "mon", null)).toEqual({});
  });
});

describe("sameWeek", () => {
  it("compares hours day by day, whatever order the days come in", () => {
    expect(
      sameWeek(
        { sat: [["09:00", "17:00"]], mon: [["09:00", "17:00"]] },
        { mon: [["09:00", "17:00"]], sat: [["09:00", "17:00"]] },
      ),
    ).toBe(true);
    expect(sameWeek({ mon: [["09:00", "17:00"]] }, {})).toBe(false);
  });
});

describe("holidayWhen", () => {
  it("says a yearly holiday comes back every year", () => {
    expect(holidayWhen({ date: "2026-12-25", name: "Christmas", yearly: true })).toBe("Dec 25, every year");
  });

  it("dates a one-off holiday, and one stored without the flag", () => {
    expect(holidayWhen({ date: "2027-03-03", name: "Offsite", yearly: false })).toBe("Mar 3, 2027");
    expect(holidayWhen({ date: "2026-01-01" })).toBe("Jan 1, 2026");
  });
});
