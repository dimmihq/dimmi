/*
 * Opens the app's realtime connection once, for the whole signed-in
 * app, and keeps the query cache of the workspace on screen current:
 * the workspace's channel for its lists and teammates, and the
 * member's own channel for their counts and read state. When the
 * server cannot replay what the app missed while offline, the app
 * refetches instead; when it closes the connection because the
 * person's access changed, everything is fetched again under the new
 * rules.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { ConnectingContext, DisconnectedContext, StateContext } from "centrifuge";
import { type ReactNode, useEffect, useMemo, useState } from "react";

import { callerQuery } from "@/lib/api/workspace";

import { Bridge } from "./bridge";
import { type ConnectionState, endpoint, RealtimeClient } from "./client";
import { RealtimeContext } from "./context";
import { channels } from "./keys";
import { useChannel } from "./use-channel";

/*
 * accessChanged is the code the server closes connections with when the
 * person lost access to a workspace.
 */
const accessChanged = 4009;

function newClient(): RealtimeClient | null {
  if (typeof window === "undefined" || typeof WebSocket === "undefined") {
    return null;
  }
  return new RealtimeClient(endpoint(window.location));
}

export function RealtimeProvider({ workspace, children }: { workspace: string | null; children: ReactNode }) {
  const queryClient = useQueryClient();
  const [client] = useState(newClient);
  const [bridge] = useState(() => new Bridge(queryClient));
  const [state, setState] = useState<ConnectionState>("connecting");

  useEffect(() => {
    if (!client) {
      return;
    }

    const changed = (ctx: StateContext) => setState(ctx.newState as ConnectionState);
    const dropped = (ctx: ConnectingContext | DisconnectedContext) => {
      if (ctx.code === accessChanged) {
        void queryClient.invalidateQueries();
      }
    };

    client.centrifuge.on("state", changed);
    client.centrifuge.on("connecting", dropped);
    client.centrifuge.on("disconnected", dropped);
    client.connect();

    return () => {
      client.centrifuge.removeListener("state", changed);
      client.centrifuge.removeListener("connecting", dropped);
      client.centrifuge.removeListener("disconnected", dropped);
      client.disconnect();
    };
  }, [client, queryClient]);

  const value = useMemo(() => ({ client, state, workspace, bridge }), [client, state, workspace, bridge]);

  return (
    <RealtimeContext value={value}>
      {workspace ? <WorkspaceChannels workspace={workspace} bridge={bridge} /> : null}
      {children}
    </RealtimeContext>
  );
}

/*
 * WorkspaceChannels follows the workspace's channel and the member's
 * own, feeding both into the cache.
 */
function WorkspaceChannels({ workspace, bridge }: { workspace: string; bridge: Bridge }) {
  const { data: caller } = useQuery(callerQuery(workspace));
  const member = caller?.member?.id ?? null;
  const handlers = {
    onEnvelope: (envelope: Parameters<Bridge["apply"]>[2], channel: string) =>
      bridge.apply(workspace, channel, envelope),
    onSubscribed: (recovered: boolean, wasRecovering: boolean) => {
      if (wasRecovering && !recovered) {
        bridge.resync(workspace);
      }
    },
  };

  useChannel(channels.workspace(workspace), handlers);
  useChannel(member ? channels.member(member) : null, handlers);

  return null;
}
