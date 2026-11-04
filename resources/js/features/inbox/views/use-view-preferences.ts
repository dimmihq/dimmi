/*
 * How the person likes each view of their inbox: its sort and status,
 * kept on the server so every device and tab shows the same. A change
 * shows at once and is saved a second after the last one, so clicking
 * through sorts sends one request. Other tabs hear about the save and
 * read the preferences again.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef } from "react";

import { type InboxPreferences, preferencesQuery, updatePreferences, type ViewPreference } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

const saveDelay = 1000;

export function useViewPreferences(workspace: string, view: string) {
  const client = useQueryClient();
  const mutation = useMutationId();
  const { data } = useQuery({ ...preferencesQuery(workspace), enabled: workspace !== "" });
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  const save = useCallback(
    (change: ViewPreference) => {
      const key = realtimeKeys.preferences(workspace);
      const current = client.getQueryData<InboxPreferences>(key);
      const views = { ...(current?.views ?? {}), [view]: { ...(current?.views[view] ?? {}), ...change } };
      client.setQueryData<InboxPreferences>(key, (old) => ({
        sidebar: {},
        pinned_attributes: [],
        updated_at: null,
        ...old,
        views,
      }));
      clearTimeout(timer.current);
      timer.current = setTimeout(() => {
        void updatePreferences({ workspace, mutation: mutation() }, { views }).catch(() =>
          client.invalidateQueries({ queryKey: key }),
        );
      }, saveDelay);
    },
    [client, mutation, view, workspace],
  );

  return { preference: data?.views[view] ?? {}, save };
}
