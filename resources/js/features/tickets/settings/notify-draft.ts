/*
 * The draft of the states' Notify switches: those flipped but not saved
 * yet, held for the tickets settings page's save bar.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { type TicketState, ticketKeys, updateTicketState } from "@/lib/api/tickets";

/*
 * useNotifyDraft holds Notify switches flipped but not saved yet. Save
 * sends each, then reads the states again; Discard puts them back.
 */
export function useNotifyDraft(workspace: string) {
  const client = useQueryClient();
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const save = useMutation({
    mutationFn: () =>
      Promise.all(Object.entries(pending).map(([id, on]) => updateTicketState(workspace, id, { notify_customer: on }))),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      setPending({});
      toast.success("States saved");
    },
  });

  return {
    pending,
    dirty: Object.keys(pending).length > 0,
    save,
    flip: (state: TicketState, on: boolean) =>
      setPending(({ [state.id]: _, ...rest }) => (on === state.notify_customer ? rest : { ...rest, [state.id]: on })),
    discard: () => setPending({}),
  };
}

export type NotifyDraft = ReturnType<typeof useNotifyDraft>;
