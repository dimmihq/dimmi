import { describe, expect, it } from "vitest";

import type { WidgetSettings } from "@/lib/api/widget-settings";

import { changed, hasOwnWords, withChange, withDraft } from "./draft";

const saved = {
  id: "wgt_1",
  name: "Website",
  enabled: true,
  inbox: { id: "inbox_1", name: "Support" },
  appearance: {
    primary_color: "#10b981",
    header_color: null,
    button_color: null,
    background: { type: "solid", fade: false },
    theme: "auto",
    position: "right",
    padding_x: 20,
    padding_y: 20,
    font: "system",
    sound: true,
    launcher: { style: "bubble", shape: "circle", label: "", icon: "chat" },
  },
  texts: { greeting: "Hi" },
} as unknown as WidgetSettings;

describe("messenger drafts", () => {
  it("lays the changes over the saved settings, one level deep for the look", () => {
    const draft = withChange(withChange({}, { appearance: { launcher: { shape: "squircle" } } }), {
      appearance: { primary_color: "#3b82f6" },
      texts: { chat_title: "Talk to us" },
    });
    const shown = withDraft(saved, draft);

    expect(shown.appearance.primary_color).toBe("#3b82f6");
    expect(shown.appearance.launcher).toEqual({ style: "bubble", shape: "squircle", label: "", icon: "chat" });
    expect(shown.texts).toEqual({ greeting: "Hi", chat_title: "Talk to us" });
    expect(shown.name).toBe("Website");
  });

  it("keeps only what changed, so a save sends just that", () => {
    const draft = withChange({}, { name: "App", appearance: { launcher: { label: "Help" } } });

    expect(draft).toEqual({ name: "App", appearance: { launcher: { label: "Help" } } });
    expect(changed(draft)).toBe(true);
    expect(changed({})).toBe(false);
  });

  it("merges each language's words one by one and marks the languages that have any", () => {
    const withFrench = { ...saved, texts_by_locale: { fr: { greeting_intro: "Salut", delay_notice: "Absents" } } };
    const draft = withChange(withChange({}, { texts_by_locale: { fr: { greeting_intro: "" } } }), {
      texts_by_locale: { de: { greeting_intro: "Hallo" } },
    });
    const shown = withDraft(withFrench, draft);

    expect(shown.texts_by_locale).toEqual({
      fr: { greeting_intro: "", delay_notice: "Absents" },
      de: { greeting_intro: "Hallo" },
    });
    expect(hasOwnWords(shown.texts_by_locale, "fr")).toBe(true);
    expect(hasOwnWords({ fr: { greeting_intro: "" } }, "fr")).toBe(false);
    expect(hasOwnWords(shown.texts_by_locale, "es")).toBe(false);
  });
});
