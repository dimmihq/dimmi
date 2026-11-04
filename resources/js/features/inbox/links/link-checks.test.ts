import { describe, expect, it } from "vitest";

import { check, hostOf, reasonsFor, reasonText, under } from "./link-checks";

const on = { enabled: true, allow_domains: [], block_domains: [], strip_inbound_email_links: false };

describe("link checks", () => {
  it("reads the host of web links only", () => {
    expect(hostOf("https://Example.com/path")).toBe("example.com");
    expect(hostOf("mailto:ann@example.com")).toBeNull();
    expect(hostOf("not a url")).toBeNull();
  });

  it("matches a domain and the names under it, not look-alikes", () => {
    expect(under("example.com", "example.com")).toBe(true);
    expect(under("docs.example.com", "example.com")).toBe(true);
    expect(under("badexample.com", "example.com")).toBe(false);
  });

  it("opens ordinary links and links whose text names the same site", () => {
    expect(check("https://example.com/a", "Read this", on)).toEqual({ kind: "open", host: "example.com" });
    expect(check("https://www.example.com", "example.com", on)?.kind).toBe("open");
    expect(check("mailto:x@example.com", "mail", on)).toBeNull();
  });

  it("warns when the text names another site, and for look-alikes, shorteners and addresses", () => {
    expect(reasonsFor("evil.example", "https://bank.example/login")).toEqual(["other_host"]);
    expect(reasonsFor("xn--pple-43d.com", "apple")).toEqual(["lookalike"]);
    expect(reasonsFor("bit.ly", "click")).toEqual(["shortener"]);
    expect(reasonsFor("10.0.0.1", "router")).toEqual(["address"]);
    expect(reasonsFor("[::1]", "here")).toEqual(["address"]);
    expect(check("https://bit.ly/x", "promo", on)).toEqual({ kind: "warn", host: "bit.ly", reasons: ["shortener"] });
    expect(reasonText("shortener")).toContain("shortener");
  });

  it("blocks listed domains always and skips warnings for allowed ones or when off", () => {
    const listed = { ...on, allow_domains: ["bit.ly"], block_domains: ["evil.example"] };
    expect(check("https://login.evil.example", "x", listed)?.kind).toBe("blocked");
    expect(check("https://login.evil.example", "x", { ...listed, enabled: false })?.kind).toBe("blocked");
    expect(check("https://bit.ly/x", "x", listed)?.kind).toBe("open");
    expect(check("https://bit.ly/x", "x", { ...on, enabled: false })?.kind).toBe("open");
  });
});
