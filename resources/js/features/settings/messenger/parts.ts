/*
 * What every tab of the messenger settings is given: the settings as the
 * draft leaves them, a way to change them, and whether the person may.
 */
import type { WidgetChange, WidgetSettings } from "@/lib/api/widget-settings";

export interface TabProps {
  value: WidgetSettings;
  set: (change: WidgetChange) => void;
  locked: boolean;
}

/*
 * labelled maps between the values the server takes and the words the
 * page shows for them.
 */
export function labelled<T extends string>(labels: Record<T, string>) {
  const entries = Object.entries(labels) as [T, string][];

  return {
    options: entries.map(([, label]) => label),
    label: (value: T) => labels[value],
    value: (label: string) => entries.find(([, l]) => l === label)?.[0],
  };
}
