import { describe, expect, it } from "vitest";

import { decide, facts, matches } from "./rules";

const pricing = { url: "https://acme.com/Pricing/?plan=pro", width: 400, language: "de-DE" };

describe("facts", () => {
  it("reads the page and device from a visit", () => {
    expect(facts(pricing)).toMatchObject({
      "page.path": "/Pricing",
      "page.query": "plan=pro",
      "page.host": "acme.com",
      "visitor.device": "mobile",
      "visitor.locale": "de-DE",
    });
  });
});

describe("matches", () => {
  it("compares text without regard to case and ignores a trailing slash", () => {
    expect(matches({ field: "page.path", op: "eq", value: "/pricing/" }, pricing)).toBe(true);
    expect(matches({ field: "page.path", op: "starts_with", value: "/PRICING" }, pricing)).toBe(true);
    expect(matches({ field: "visitor.device", op: "in", value: ["tablet", "mobile"] }, pricing)).toBe(true);
  });

  it("matches globs, one star within a segment and two across", () => {
    const docs = { url: "https://acme.com/docs/guides/start" };
    expect(matches({ field: "page.path", op: "matches", value: "/docs/*" }, docs)).toBe(false);
    expect(matches({ field: "page.path", op: "matches", value: "/docs/**" }, docs)).toBe(true);
    expect(matches({ field: "page.path", op: "matches", value: "/docs/*/start" }, docs)).toBe(true);
  });

  it("joins conditions with all, any and not", () => {
    const rule = {
      all: [
        {
          any: [
            { field: "page.path", op: "eq", value: "/a" },
            { field: "page.host", op: "eq", value: "acme.com" },
          ],
        },
        { not: { field: "visitor.device", op: "eq", value: "desktop" } },
      ],
    };
    expect(matches(rule, pricing)).toBe(true);
  });

  it("treats a missing value as matching only negative conditions, and unknown operators as failing", () => {
    const noReferrer = { url: "https://acme.com/" };
    expect(matches({ field: "page.referrer", op: "neq", value: "x" }, noReferrer)).toBe(true);
    expect(matches({ field: "page.referrer", op: "contains", value: "x" }, noReferrer)).toBe(false);
    expect(matches({ field: "page.path", op: "sounds_like", value: "/" }, noReferrer)).toBe(false);
  });
});

describe("decide", () => {
  it("shows when there is no rule and follows the server's yes or no", () => {
    expect(decide(undefined, pricing)).toBe(true);
    expect(decide(false, pricing)).toBe(false);
    expect(decide({ field: "page.path", op: "eq", value: "/pricing" }, pricing)).toBe(true);
  });
});
