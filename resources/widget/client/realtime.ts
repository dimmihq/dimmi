/*
 * How news reaches the messenger. A plain event stream carries what the
 * team does live; each publication has a position, and a stream that
 * drops reconnects from where it was, so nothing in between is lost. When
 * the stream cannot work at all, such as behind a proxy that holds it
 * back, the messenger asks the server for news every few seconds instead,
 * and the visitor never sees the difference beyond a short delay.
 *
 * Every event is handed on once, even when the stream and a poll both
 * bring it. One visitor's widgets share a channel, so an event about a
 * conversation on another widget is dropped here.
 */
import { ApiError, type Http } from "./http";

export interface Envelope {
  id?: string;
  event: string;
  data: Record<string, unknown>;
}

export interface Source {
  onmessage: ((event: { data: string }) => void) | null;
  onerror: ((event: unknown) => void) | null;
  close(): void;
}

export interface RealtimeDeps {
  eventSource: (url: string) => Source;
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (handle: unknown) => void;
  now: () => number;
}

export interface Listener {
  event(envelope: Envelope): void;
  resync(): void;
  ended(): void;
  mode(mode: "stream" | "polling" | "off"): void;
}

interface Push {
  connect?: { subs?: Record<string, { recovered?: boolean; epoch?: string; offset?: number; publications?: Pub[] }> };
  pub?: Pub;
  channel?: string;
  disconnect?: { code: number };
}

interface Pub {
  data: Envelope;
  offset?: number;
}

interface Token {
  token: string;
  url: string;
  channel: string;
}

/*
 * Close codes: 4009 means the session ended, 4004 that a newer tab took
 * this one's place.
 */
const sessionEnded = 4009;
const replaced = 4004;
const firstFrameWait = 10_000;
const failureWindow = 30_000;
const maxBackoff = 30_000;
const seenKept = 300;

export class Realtime {
  private source: Source | undefined;
  private channel = "";
  private epoch = "";
  private offset = 0;
  private cursor = "";
  private seen: string[] = [];
  private failures: number[] = [];
  private waiting: unknown;
  private polling = false;
  private stopped = true;

  constructor(
    private readonly http: Http,
    private readonly deps: RealtimeDeps,
    private readonly widgetId: string,
    private readonly listener: Listener,
  ) {}

  /*
   * start opens the stream, or polls when streams already failed here.
   */
  start() {
    this.stopped = false;
    if (this.polling) {
      void this.poll();
      return;
    }
    void this.open();
  }

  stop() {
    this.stopped = true;
    this.source?.close();
    this.source = undefined;
    this.deps.clearTimeout(this.waiting);
    this.listener.mode("off");
  }

  private async open() {
    let token: Token;
    try {
      token = (await this.http.post<{ data: Token }>("/realtime-tokens")).data;
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        this.end();
        return;
      }
      this.failed();
      return;
    }
    if (this.stopped) {
      return;
    }

    this.channel = token.channel;
    const connect: Record<string, unknown> = { token: token.token };
    if (this.epoch) {
      connect.subs = { [this.channel]: { recover: true, epoch: this.epoch, offset: this.offset } };
    }

    const source = this.deps.eventSource(`${token.url}?cf_connect=${encodeURIComponent(JSON.stringify(connect))}`);
    this.source = source;
    this.waiting = this.deps.setTimeout(() => this.fallBack(), firstFrameWait);
    source.onmessage = (message) => this.receive(message.data);
    source.onerror = () => {
      if (this.source === source) {
        source.close();
        this.source = undefined;
        this.failed();
      }
    };
  }

  private receive(raw: string) {
    let push: Push;
    try {
      push = JSON.parse(raw) as Push;
    } catch {
      return;
    }

    if (push.connect) {
      this.connected(push.connect.subs?.[this.channel]);
    } else if (push.pub) {
      this.publication(push.pub);
    } else if (push.disconnect) {
      this.disconnected(push.disconnect.code);
    }
  }

  private connected(sub: { recovered?: boolean; epoch?: string; offset?: number; publications?: Pub[] } = {}) {
    this.deps.clearTimeout(this.waiting);
    const hadPosition = this.epoch !== "";
    this.listener.mode("stream");

    // A first connect has nothing to recover from: what changed between
    // the last load and this subscription, such as a picture checked
    // just after it was sent, is read again.
    if (!hadPosition || !sub.recovered) {
      this.listener.resync();
    }
    for (const pub of sub.publications ?? []) {
      this.publication(pub);
    }
    this.epoch = sub.epoch ?? this.epoch;
    this.offset = sub.offset ?? this.offset;
  }

  private publication(pub: Pub) {
    if (pub.offset !== undefined) {
      this.offset = Math.max(this.offset, pub.offset);
    }
    this.deliver(pub.data);
  }

  private disconnected(code: number) {
    this.source?.close();
    this.source = undefined;
    if (code === sessionEnded) {
      this.end();
      return;
    }
    if (code === replaced) {
      this.fallBack();
      return;
    }
    this.failed();
  }

  /*
   * failed retries the stream after a pause, and gives up on streams for
   * this page once two attempts failed within half a minute.
   */
  private failed() {
    if (this.stopped) {
      return;
    }
    const now = this.deps.now();
    this.failures = [...this.failures.filter((at) => now - at < failureWindow), now];
    if (this.failures.length >= 2) {
      this.fallBack();
      return;
    }
    this.deps.setTimeout(() => void this.open(), Math.min(maxBackoff, 1000 * this.failures.length));
  }

  private fallBack() {
    if (this.stopped || this.polling) {
      return;
    }
    this.source?.close();
    this.source = undefined;
    this.polling = true;
    this.listener.mode("polling");
    void this.poll();
  }

  private async poll() {
    if (this.stopped) {
      return;
    }

    let wait = 5000;
    try {
      const since = this.cursor ? `?since=${encodeURIComponent(this.cursor)}` : "";
      const res = await this.http.get<{
        data: { event: string; id: string; payload: Record<string, unknown> }[];
        meta: { cursor: string; poll_after_ms: number; has_more: boolean };
      }>(`/updates${since}`);
      for (const item of res.data) {
        this.deliver({ id: item.id, event: item.event, data: item.payload });
      }
      this.cursor = res.meta.cursor;
      wait = res.meta.has_more ? 0 : res.meta.poll_after_ms;
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        this.end();
        return;
      }
      if (error instanceof ApiError && error.status === 400) {
        this.cursor = "";
        this.listener.resync();
      }
    }

    this.waiting = this.deps.setTimeout(() => void this.poll(), wait);
  }

  private deliver(envelope: Envelope) {
    const widget = envelope.data?.widget_id;
    if (typeof widget === "string" && widget !== this.widgetId) {
      return;
    }
    if (envelope.id) {
      if (this.seen.includes(envelope.id)) {
        return;
      }
      this.seen = [...this.seen.slice(-(seenKept - 1)), envelope.id];
    }
    this.listener.event(envelope);
  }

  private end() {
    this.stop();
    this.listener.ended();
  }
}
