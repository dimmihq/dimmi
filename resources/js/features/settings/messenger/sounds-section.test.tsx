import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { WidgetAppearance, WidgetSettings } from "@/lib/api/widget-settings";

import { SoundsSection } from "./sounds-section";

const play = vi.fn();
vi.mock("../../../../widget/lib/sound", () => ({ play: (sound: string) => play(sound) }));

function settings(look: Partial<WidgetAppearance>): WidgetSettings {
  return {
    id: "wgt_1",
    appearance: { sound: true, sent_sound: "pop", reply_sound: "pop", ...look },
  } as unknown as WidgetSettings;
}

describe("messenger sounds", () => {
  beforeEach(() => play.mockReset());

  it("plays each sound from the button beside it", () => {
    render(<SoundsSection value={settings({})} set={vi.fn()} locked={false} />);

    fireEvent.click(screen.getByRole("button", { name: "Play incoming sound" }));
    fireEvent.click(screen.getByRole("button", { name: "Play outgoing sound" }));
    expect(play.mock.calls).toEqual([["in"], ["out"]]);
  });

  it("turns the outgoing sound back on and plays it", () => {
    const set = vi.fn();
    render(<SoundsSection value={settings({ sent_sound: "none" })} set={set} locked={false} />);

    const outgoing = screen.getByRole<HTMLInputElement>("switch", { name: "Outgoing sound" });
    expect(outgoing.checked).toBe(false);
    fireEvent.click(outgoing);
    expect(set).toHaveBeenCalledWith({ appearance: { sent_sound: "pop" } });
    expect(play).toHaveBeenCalledWith("out");
  });

  it("waits on each sound while sounds are off", () => {
    render(<SoundsSection value={settings({ sound: false })} set={vi.fn()} locked={false} />);

    expect(screen.getByRole<HTMLInputElement>("switch", { name: "Incoming sound" }).disabled).toBe(true);
    expect(screen.getByRole<HTMLInputElement>("switch", { name: "Outgoing sound" }).disabled).toBe(true);
  });
});
