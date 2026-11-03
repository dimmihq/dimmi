/*
 * Hooks that follow a channel for as long as a component is on screen:
 * every broadcast on it, and who else is listening. Both share the
 * app's one connection, and both do nothing outside the app shell or
 * without a channel.
 */
import type { JoinContext, LeaveContext, PublicationContext, SubscribedContext } from "centrifuge";
import { useEffect, useRef, useState } from "react";

import type { Envelope } from "./bridge";
import { useRealtime } from "./context";

/*
 * ChannelHandlers receive what happens on a channel: each broadcast,
 * and every (re)subscription with whether missed broadcasts were
 * replayed.
 */
export interface ChannelHandlers {
  onEnvelope?: (envelope: Envelope, channel: string) => void;
  onSubscribed?: (recovered: boolean, wasRecovering: boolean) => void;
}

/*
 * useChannel follows the channel and hands its broadcasts to the
 * handlers, which may change between renders without resubscribing.
 */
export function useChannel(channel: string | null, handlers: ChannelHandlers) {
  const { client } = useRealtime();
  const latest = useRef(handlers);
  latest.current = handlers;

  useEffect(() => {
    if (!client || !channel) {
      return;
    }

    const sub = client.acquire(channel);
    const publication = (ctx: PublicationContext) => latest.current.onEnvelope?.(ctx.data as Envelope, channel);
    const subscribed = (ctx: SubscribedContext) => latest.current.onSubscribed?.(ctx.recovered, ctx.wasRecovering);
    sub.on("publication", publication);
    sub.on("subscribed", subscribed);

    return () => {
      sub.removeListener("publication", publication);
      sub.removeListener("subscribed", subscribed);
      client.release(channel);
    };
  }, [client, channel]);
}

/*
 * Listener is one person listening on a channel, however many tabs
 * they have open.
 */
export interface Listener {
  user: string;
  name: string;
  avatarUrl: string | null;
}

interface Info {
  user: string;
  connInfo?: { name?: string; avatar_url?: string | null };
}

/*
 * listenersOf turns connections into people, one entry per user.
 */
export function listenersOf(connections: Iterable<Info>): Listener[] {
  const people = new Map<string, Listener>();
  for (const info of connections) {
    if (!people.has(info.user)) {
      people.set(info.user, {
        user: info.user,
        name: info.connInfo?.name ?? "",
        avatarUrl: info.connInfo?.avatar_url ?? null,
      });
    }
  }
  return [...people.values()];
}

/*
 * usePresence returns who listens on the channel, kept current as
 * people arrive and leave.
 */
export function usePresence(channel: string | null): Listener[] {
  const { client } = useRealtime();
  const [connections, setConnections] = useState<Map<string, Info>>(new Map());

  useEffect(() => {
    if (!client || !channel) {
      return;
    }

    const sub = client.acquire(channel);
    const refresh = () => {
      sub
        .presence()
        .then((result) => setConnections(new Map(Object.entries(result.clients))))
        .catch(() => setConnections(new Map()));
    };
    const join = (ctx: JoinContext) => setConnections((current) => new Map(current).set(ctx.info.client, ctx.info));
    const leave = (ctx: LeaveContext) =>
      setConnections((current) => {
        const next = new Map(current);
        next.delete(ctx.info.client);
        return next;
      });

    sub.on("subscribed", refresh);
    sub.on("join", join);
    sub.on("leave", leave);
    if (sub.state === "subscribed") {
      refresh();
    }

    return () => {
      sub.removeListener("subscribed", refresh);
      sub.removeListener("join", join);
      sub.removeListener("leave", leave);
      client.release(channel);
      setConnections(new Map());
    };
  }, [client, channel]);

  return listenersOf(connections.values());
}
