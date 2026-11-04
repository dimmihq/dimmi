import { matchLocale, resolveLocale } from "./locale-sync";

const enabled = ["en", "fr", "pt-BR"];

describe("matchLocale", () => {
  it("finds the same language, the plain one, or a regional one", () => {
    expect(matchLocale("FR", enabled)).toBe("fr");
    expect(matchLocale("fr-CA", enabled)).toBe("fr");
    expect(matchLocale("pt", enabled)).toBe("pt-BR");
    expect(matchLocale("de", enabled)).toBeNull();
    expect(matchLocale(null, enabled)).toBeNull();
  });
});

describe("resolveLocale", () => {
  it("prefers the person, then the workspace, then the browser", () => {
    expect(resolveLocale({ enabled, user: "fr", workspace: "pt-BR", browser: ["en-US"] })).toBe("fr");
    expect(resolveLocale({ enabled, user: null, workspace: "pt-BR", browser: ["fr"] })).toBe("pt-BR");
    expect(resolveLocale({ enabled, user: null, workspace: null, browser: ["de-DE", "fr-CA"] })).toBe("fr");
  });

  it("skips a choice the server no longer serves and ends in English", () => {
    expect(resolveLocale({ enabled, user: "de", workspace: "es", browser: ["ja"] })).toBe("en");
    expect(resolveLocale({ enabled: ["de"], browser: [] })).toBe("de");
  });
});
