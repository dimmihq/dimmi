import { beforeEach, describe, expect, it } from "vitest";

import { isTrusted, trustDomain } from "./trusted-domains";

describe("trusted domains", () => {
  beforeEach(() => window.localStorage.clear());

  it("trusts a domain and the names under it, not look-alikes", () => {
    expect(isTrusted(["example.com"], "docs.example.com")).toBe(true);
    expect(isTrusted(["example.com"], "badexample.com")).toBe(false);
  });

  it("keeps the domains per workspace and ignores repeats", () => {
    trustDomain("w1", "example.com");
    trustDomain("w1", "example.com");
    trustDomain("w2", "other.com");
    expect(JSON.parse(window.localStorage.getItem("dimmi:links:trusted:w1") ?? "[]")).toEqual(["example.com"]);
    expect(JSON.parse(window.localStorage.getItem("dimmi:links:trusted:w2") ?? "[]")).toEqual(["other.com"]);
  });
});
