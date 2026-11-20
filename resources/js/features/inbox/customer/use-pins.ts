/*
 * The attribute keys a teammate pinned, saved with their inbox
 * preferences so the same keys come first on every customer.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { type InboxPreferences, preferencesQuery, updatePreferences } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

export function usePins(workspace: string) {
  const client = useQueryClient();
  const mutation = useMutationId();
  const { data } = useQuery({ ...preferencesQuery(workspace), enabled: workspace !== "" });
  const pinned = data?.pinned_attributes ?? [];
  const toggle = (key: string) => {
    const next = pinned.includes(key) ? pinned.filter((pin) => pin !== key) : [...pinned, key];
    const cacheKey = realtimeKeys.preferences(workspace);
    void client.cancelQueries({ queryKey: cacheKey });
    client.setQueryData<InboxPreferences>(cacheKey, (old) => ({
      sidebar: {},
      views: {},
      updated_at: null,
      ...old,
      pinned_attributes: next,
    }));
    void updatePreferences({ workspace, mutation: mutation() }, { pinned_attributes: next })
      .then((saved) => client.setQueryData(cacheKey, saved))
      .catch(() => client.invalidateQueries({ queryKey: cacheKey }));
  };

  return { pinned, toggle };
}
