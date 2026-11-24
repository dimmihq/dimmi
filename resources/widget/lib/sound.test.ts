import { describe, expect, it } from "vitest";

import { notes, play, soundOf, sounds } from "./sound";

describe("sounds", () => {
  it("reads a setting as a known sound or the fallback", () => {
    expect(soundOf("chime", "pop")).toBe("chime");
    expect(soundOf("none", "pop")).toBe("none");
    expect(soundOf("klaxon", "pop")).toBe("pop");
    expect(soundOf(undefined, "none")).toBe("none");
  });

  it("keeps every sound short", () => {
    for (const sound of sounds) {
      if (sound === "none") {
        continue;
      }
      for (const [, , , level, at, length] of notes[sound]) {
        expect(at + length).toBeLessThan(1);
        expect(level).toBeLessThanOrEqual(0.2);
      }
    }
  });

  it("stays silent without Web Audio", () => {
    expect(() => play("pop")).not.toThrow();
  });
});
