/*
 * Broadcasts fill in as they go out. The server says so on the
 * workspace's channel each time it counts a broadcast again, with its
 * status, why it paused and its counts, and every cached copy of that
 * broadcast takes them at once: the list's rows, the broadcast itself
 * and its report. The parts of a report the message does not carry,
 * such as links, accounts and recipients, are read again a few seconds
 * later, once for however many messages arrived meanwhile.
 */
import { type QueryClient, useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";

import {
  type Broadcast,
  type BroadcastProgress,
  type BroadcastReport,
  broadcastQuery,
  broadcastReportQuery,
  broadcastsKey,
} from "@/lib/api/broadcasts";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

const settle = 3000;

function isProgress(value: unknown): value is BroadcastProgress {
  return (
    typeof value === "object" &&
    value !== null &&
    (value as BroadcastProgress).object === "broadcast" &&
    typeof (value as BroadcastProgress).id === "string"
  );
}

/*
 * apply lays the progress over every cached copy of the broadcast.
 */
export function apply(queryClient: QueryClient, workspace: string, progress: BroadcastProgress) {
  const patch: Partial<Broadcast> = {
    status: progress.status,
    pause_reason: progress.pause_reason ?? null,
    ...(progress.counts ? { counts: progress.counts } : {}),
  };
  queryClient.setQueryData<Broadcast>(broadcastQuery(workspace, progress.id).queryKey, (old) =>
    old ? { ...old, ...patch } : old,
  );
  queryClient.setQueryData<BroadcastReport>(broadcastReportQuery(workspace, progress.id).queryKey, (old) =>
    old ? { ...old, broadcast: { ...old.broadcast, ...patch }, counts: progress.counts ?? old.counts } : old,
  );
  queryClient.setQueriesData<Broadcast[]>({ queryKey: [...broadcastsKey(workspace), "list"] }, (old) =>
    old?.map((row) => (row.id === progress.id ? { ...row, ...patch } : row)),
  );
}

/*
 * useLiveBroadcasts keeps the cached broadcasts current for as long as
 * the screen showing them is open.
 */
export function useLiveBroadcasts(workspace: string) {
  const queryClient = useQueryClient();
  const waiting = useRef(new Set<string>());

  useChannel(workspace ? channels.workspace(workspace) : null, {
    onEnvelope: (envelope) => {
      if (envelope.event !== "broadcast.updated" || !isProgress(envelope.data)) {
        return;
      }
      const id = envelope.data.id;
      apply(queryClient, workspace, envelope.data);
      if (waiting.current.has(id)) {
        return;
      }
      waiting.current.add(id);
      window.setTimeout(() => {
        waiting.current.delete(id);
        void queryClient.invalidateQueries({ queryKey: broadcastReportQuery(workspace, id).queryKey });
        void queryClient.invalidateQueries({ queryKey: [...broadcastsKey(workspace), "one", id, "recipients"] });
      }, settle);
    },
  });
}
