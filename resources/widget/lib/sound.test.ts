import { describe, expect, it } from "vitest";

import { notes, play, playing } from "./sound";

describe("sounds", () => {
  it("reads a setting as on unless it is none", () => {
    expect(playing("none")).toBe(false);
    expect(playing("pop")).toBe(true);
    expect(playing("chime")).toBe(true);
    expect(playing(undefined)).toBe(true);
  });

  it("keeps both sounds short and soft", () => {
    for (const sound of ["in", "out"] as const) {
      expect(notes[sound].length).toBeGreaterThan(0);
      for (const [, , , level, at, length] of notes[sound]) {
        expect(at + length).toBeLessThan(0.5);
        expect(level).toBeLessThanOrEqual(0.2);
      }
    }
  });

  it("stays silent without Web Audio", () => {
    expect(() => play("in")).not.toThrow();
  });
});
