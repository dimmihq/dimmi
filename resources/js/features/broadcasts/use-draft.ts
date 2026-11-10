/*
 * The composer's draft, saved as it is written. A change shows at once
 * and is saved a moment later together with whatever else changed
 * meanwhile, so typing a subject is one save, not one per letter. Saves
 * go out one after another, never side by side, and whatever is still
 * waiting goes out before a send, a test or leaving the page. A refused
 * save says why and keeps what was typed.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import {
  type Broadcast,
  broadcastEstimateQuery,
  broadcastQuery,
  broadcastsKey,
  saveBroadcast,
} from "@/lib/api/broadcasts";

import { changesOf, type Draft, type DraftKey, draftOf } from "./draft";

const pause = 600;

/*
 * reach names the fields whose change alters who gets the broadcast, so
 * only those count the audience again.
 */
const reach = new Set<DraftKey>(["audience_id", "rules", "room_ids", "topic_id", "service", "skip_frequency_cap"]);

export interface DraftState {
  draft: Draft;
  saving: boolean;
  change: (change: Partial<Draft>) => void;
  flush: () => Promise<void>;
}

export function useDraft(workspace: string, broadcast: Broadcast): DraftState {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(() => draftOf(broadcast));
  const [saving, setSaving] = useState(false);
  const latest = useRef(draft);
  const pending = useRef(new Set<DraftKey>());
  const timer = useRef<number | undefined>(undefined);
  const flight = useRef<Promise<void>>(Promise.resolve());

  const flush = useCallback((): Promise<void> => {
    window.clearTimeout(timer.current);
    if (pending.current.size === 0) {
      return flight.current;
    }
    const keys = pending.current;
    const changes = changesOf(latest.current, keys);
    pending.current = new Set();
    setSaving(true);
    flight.current = flight.current
      .then(() => saveBroadcast(workspace, broadcast.id, changes))
      .then((saved) => {
        queryClient.setQueryData(broadcastQuery(workspace, broadcast.id).queryKey, saved);
        if ([...keys].some((key) => reach.has(key))) {
          void queryClient.invalidateQueries({ queryKey: broadcastEstimateQuery(workspace, broadcast.id).queryKey });
        }
        void queryClient.invalidateQueries({ queryKey: [...broadcastsKey(workspace), "list"] });
      })
      .catch((error: Error) => void toast.error(error.message))
      .finally(() => setSaving(false));

    return flight.current;
  }, [queryClient, workspace, broadcast.id]);

  const change = useCallback(
    (next: Partial<Draft>) => {
      latest.current = { ...latest.current, ...next };
      setDraft(latest.current);
      for (const key of Object.keys(next) as DraftKey[]) {
        pending.current.add(key);
      }
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => void flush(), pause);
    },
    [flush],
  );

  const flushRef = useRef(flush);
  flushRef.current = flush;
  useEffect(() => () => void flushRef.current(), []);

  return { draft, saving, change, flush };
}
