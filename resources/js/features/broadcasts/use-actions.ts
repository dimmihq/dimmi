/*
 * What a teammate can do to a broadcast once it is written: send or
 * schedule it, pause, resume or stop it while it goes out, unschedule
 * it back to a draft, duplicate it or delete a draft. Each answer goes
 * into the cache at once so the page moves on without waiting for a
 * read; a duplicate opens the copy. A refusal comes back to the caller,
 * which decides where to say why.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import {
  actOnBroadcast,
  type Broadcast,
  type BroadcastAction,
  broadcastQuery,
  broadcastReportQuery,
  broadcastsKey,
  deleteBroadcast,
} from "@/lib/api/broadcasts";

export function useBroadcastActions(workspace: string, id: string) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const refreshList = () => queryClient.invalidateQueries({ queryKey: [...broadcastsKey(workspace), "list"] });

  const act = useMutation({
    mutationFn: (action: BroadcastAction) => actOnBroadcast(workspace, id, action),
    onSuccess: (broadcast: Broadcast, action) => {
      void refreshList();
      if (action === "duplicate") {
        queryClient.setQueryData(broadcastQuery(workspace, broadcast.id).queryKey, broadcast);
        void navigate({ to: "/d/broadcasts/$broadcast", params: { broadcast: broadcast.id } });
        return;
      }
      queryClient.setQueryData(broadcastQuery(workspace, id).queryKey, broadcast);
      void queryClient.invalidateQueries({ queryKey: broadcastReportQuery(workspace, id).queryKey });
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteBroadcast(workspace, id),
    onSuccess: () => {
      void refreshList();
      void navigate({ to: "/d/broadcasts" }).then(() =>
        queryClient.removeQueries({ queryKey: broadcastQuery(workspace, id).queryKey }),
      );
    },
  });

  return { act, remove };
}
