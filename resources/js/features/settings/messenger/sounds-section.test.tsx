import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { WidgetAppearance, WidgetSettings } from "@/lib/api/widget-settings";

import { SoundsSection } from "./sounds-section";

const play = vi.fn();
vi.mock("../../../../widget/lib/sound", () => ({ play: (sound: string) => play(sound) }));

function settings(look: Partial<WidgetAppearance>): WidgetSettings {
  return {
    id: "wgt_1",
    appearance: { sound: true, sent_sound: "none", reply_sound: "pop", ...look },
  } as unknown as WidgetSettings;
}

describe("messenger sounds", () => {
  beforeEach(() => play.mockReset());

  it("plays the picked reply sound and cannot play none", () => {
    render(<SoundsSection value={settings({})} set={vi.fn()} locked={false} />);

    expect(screen.getByRole<HTMLButtonElement>("button", { name: "Play sent sound" }).disabled).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Play reply sound" }));
    expect(play).toHaveBeenCalledWith("pop");
  });

  it("waits on the sound pickers while sounds are off", () => {
    render(<SoundsSection value={settings({ sound: false, sent_sound: "tick" })} set={vi.fn()} locked={false} />);

    const pickers = screen
      .getAllByRole<HTMLButtonElement>("button")
      .filter((button) => !button.getAttribute("aria-label")?.startsWith("Play"));
    expect(pickers.length).toBeGreaterThanOrEqual(2);
    expect(pickers.filter((picker) => picker.disabled)).toHaveLength(2);
  });
});
