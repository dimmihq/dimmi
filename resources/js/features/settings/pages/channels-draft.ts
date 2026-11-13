/*
 * The draft of an inbox's channels: the messengers and email addresses
 * picked to go to another inbox but not moved yet, held for the inbox
 * page's save.
 */
import { useState } from "react";

import { updateEmailAddress } from "@/lib/api/email";
import { moveWidget } from "@/lib/api/widget-settings";

/*
 * useChannelsDraft holds the channels sent to another inbox but not
 * saved yet, by messenger id and by email address. send moves each; the
 * inbox page sends them with the rest of its changes, and discard puts
 * them back.
 */
export function useChannelsDraft(workspace: string) {
  const [widgets, setWidgets] = useState<Record<string, string>>({});
  const [addresses, setAddresses] = useState<Record<string, string>>({});
  const moved = (current: string, to: string, id: string) => (pending: Record<string, string>) => {
    const { [id]: _, ...rest } = pending;

    return to === current ? rest : { ...rest, [id]: to };
  };

  return {
    widgets,
    addresses,
    dirty: Object.keys(widgets).length + Object.keys(addresses).length > 0,
    moveWidget: (id: string, current: string, to: string) => setWidgets(moved(current, to, id)),
    moveAddress: (address: string, current: string, to: string) => setAddresses(moved(current, to, address)),
    send: () =>
      Promise.all([
        ...Object.entries(widgets).map(([id, to]) => moveWidget(workspace, id, to)),
        ...Object.entries(addresses).map(([address, to]) => updateEmailAddress(workspace, address, { inbox_id: to })),
      ]),
    discard: () => {
      setWidgets({});
      setAddresses({});
    },
  };
}

export type ChannelsDraft = ReturnType<typeof useChannelsDraft>;
