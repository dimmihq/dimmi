/*
 * How the person likes to read threads: with events or without, and
 * times in their own zone or the customer's. Both are kept with their
 * inbox preferences, so every device reads threads the same way, and
 * change at once while the save goes out. A load of the preferences
 * still under way is stopped first, so its older answer cannot land
 * after the change and put the setting back.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import { type InboxPreferences, preferencesQuery, updatePreferences } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

type Setting = "hide_events" | "customer_time";

export function useThreadSettings(workspace: string) {
  const client = useQueryClient();
  const mutation = useMutationId();
  const { data } = useQuery({ ...preferencesQuery(workspace), enabled: workspace !== "" });
  const sidebar = data?.sidebar ?? {};

  const toggle = useCallback(
    async (setting: Setting) => {
      const key = realtimeKeys.preferences(workspace);
      await client.cancelQueries({ queryKey: key });
      const current = client.getQueryData<InboxPreferences>(key);
      const next = { ...(current?.sidebar ?? {}), [setting]: !current?.sidebar[setting] };
      client.setQueryData<InboxPreferences>(key, (old) => ({
        views: {},
        pinned_attributes: [],
        updated_at: null,
        ...old,
        sidebar: next,
      }));
      await updatePreferences({ workspace, mutation: mutation() }, { sidebar: next }).catch(() =>
        client.invalidateQueries({ queryKey: key }),
      );
    },
    [client, mutation, workspace],
  );

  return {
    hideEvents: sidebar.hide_events === true,
    customerTime: sidebar.customer_time === true,
    toggleEvents: useCallback(() => void toggle("hide_events"), [toggle]),
    toggleCustomerTime: useCallback(() => void toggle("customer_time"), [toggle]),
  };
}
