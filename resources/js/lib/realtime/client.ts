/*
 * The one realtime connection of the browser app. It opens a WebSocket
 * to the page's own origin, so the session cookie authenticates it, and
 * reconnects on its own after a drop, asking every channel for what it
 * missed meanwhile. Screens share subscriptions: a channel is followed
 * once however many components listen to it, and left when the last
 * one stops.
 */
import { Centrifuge, type Subscription } from "centrifuge";

/*
 * ConnectionState is how the connection is doing, as screens show it.
 */
export type ConnectionState = "connecting" | "connected" | "disconnected";

/*
 * endpoint returns the address of the realtime endpoint on the page's
 * own origin.
 */
export function endpoint(location: Pick<Location, "protocol" | "host">): string {
  const scheme = location.protocol === "https:" ? "wss:" : "ws:";
  return `${scheme}//${location.host}/connection/websocket`;
}

/*
 * RealtimeClient wraps the connection and counts who listens to each
 * channel.
 */
export class RealtimeClient {
  readonly centrifuge: Centrifuge;
  private readonly listeners = new Map<string, number>();

  constructor(url: string) {
    this.centrifuge = new Centrifuge(url, { name: "dashboard", minReconnectDelay: 500, maxReconnectDelay: 20000 });
  }

  connect() {
    this.centrifuge.connect();
  }

  disconnect() {
    this.centrifuge.disconnect();
  }

  /*
   * acquire returns the channel's subscription, subscribing when this is
   * its first listener. Pair every acquire with a release.
   */
  acquire(channel: string): Subscription {
    const existing = this.centrifuge.getSubscription(channel);
    const sub = existing ?? this.centrifuge.newSubscription(channel);
    this.listeners.set(channel, (this.listeners.get(channel) ?? 0) + 1);
    if (sub.state === "unsubscribed") {
      sub.subscribe();
    }
    return sub;
  }

  /*
   * release lets go of a channel, leaving it once nobody listens.
   */
  release(channel: string) {
    const left = (this.listeners.get(channel) ?? 1) - 1;
    if (left > 0) {
      this.listeners.set(channel, left);
      return;
    }

    this.listeners.delete(channel);
    const sub = this.centrifuge.getSubscription(channel);
    if (sub) {
      sub.unsubscribe();
      this.centrifuge.removeSubscription(sub);
    }
  }
}
