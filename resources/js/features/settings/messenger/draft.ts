/*
 * The unsaved changes to a messenger: only what someone changed, so a
 * save sends just that, and the page reads the saved settings with the
 * changes laid over them. The look's launcher and background merge one
 * level deeper, the same way the server merges them.
 */
import type { WidgetAppearance, WidgetChange, WidgetSettings } from "@/lib/api/widget-settings";

/*
 * withDraft returns the settings as the draft would leave them.
 */
export function withDraft(saved: WidgetSettings, draft: WidgetChange): WidgetSettings {
  const look = draft.appearance ?? {};

  return {
    ...saved,
    ...draft,
    appearance: {
      ...saved.appearance,
      ...look,
      launcher: { ...saved.appearance.launcher, ...look.launcher },
      background: { ...saved.appearance.background, ...look.background },
    } as WidgetAppearance,
    texts: { ...saved.texts, ...draft.texts },
  } as WidgetSettings;
}

/*
 * withChange lays a change over the draft, merging the look and the words.
 */
export function withChange(draft: WidgetChange, change: WidgetChange): WidgetChange {
  const next: WidgetChange = { ...draft, ...change };
  if (change.appearance) {
    const before = draft.appearance ?? {};
    next.appearance = {
      ...before,
      ...change.appearance,
      ...(change.appearance.launcher ? { launcher: { ...before.launcher, ...change.appearance.launcher } } : {}),
      ...(change.appearance.background
        ? { background: { ...before.background, ...change.appearance.background } }
        : {}),
    };
  }
  if (change.texts) {
    next.texts = { ...draft.texts, ...change.texts };
  }

  return next;
}

/*
 * changed reports whether the draft holds any change.
 */
export function changed(draft: WidgetChange): boolean {
  return Object.keys(draft).length > 0;
}
