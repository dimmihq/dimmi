/*
 * What the realtime provider shares with the screens under it: the
 * connection, how it is doing, the workspace on screen and the bridge
 * into the query cache.
 */
import { createContext, useContext } from "react";

import type { Bridge } from "./bridge";
import type { ConnectionState, RealtimeClient } from "./client";

export interface Realtime {
  client: RealtimeClient | null;
  state: ConnectionState;
  workspace: string | null;
  bridge: Bridge | null;
}

export const RealtimeContext = createContext<Realtime>({
  client: null,
  state: "disconnected",
  workspace: null,
  bridge: null,
});

/*
 * useRealtime returns the realtime connection the app shell opened. Out
 * of the shell there is none, and hooks built on it do nothing.
 */
export function useRealtime(): Realtime {
  return useContext(RealtimeContext);
}

/*
 * useMutationId hands out an id for a change the screen is about to
 * make, to send in the Dimmi-Client-Mutation-Id header, so its own
 * broadcast is not applied a second time.
 */
export function useMutationId(): () => string {
  const { bridge } = useRealtime();
  return () => bridge?.mutations.next() ?? crypto.randomUUID();
}
