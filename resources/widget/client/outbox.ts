/*
 * Messages the visitor sent, until the server has them. Each carries its
 * own client id from the start, so a retry after a lost answer lands once
 * however often it is sent. The outbox is kept in storage, so a message
 * written just before the tab closed or the network dropped goes out on
 * the next page load. A refusal that will not change on retry, such as a
 * message too long, is handed back to the view; one that might, such as
 * no network or a busy server, waits and tries again, longer each time.
 */
import { ApiError, type Http } from "./http";
import type { Storage } from "./session";

export interface Pending {
  clientId: string;
  conversation?: string;
  body: string;
  attachmentIds: string[];
  /* The path of the button the visitor tapped, for an answer to a question. */
  choice?: string;
  createdAt: number;
}

export interface Sent {
  pending: Pending;
  message: Record<string, unknown>;
  conversation: Record<string, unknown> | undefined;
  conversationId: string;
}

export interface Hooks {
  sent(sent: Sent): void;
  refused(pending: Pending, error: ApiError): void;
  ready(): Promise<void>;
}

export interface Timers {
  setTimeout(fn: () => void, ms: number): unknown;
  uuid(): string;
  now(): number;
}

/*
 * Retryable codes come back by themselves once whatever held them clears.
 */
const retryable = new Set(["network", "attachment_not_uploaded", "rate_limited", "maintenance"]);
const maxDelay = 30_000;

export class Outbox {
  private queue: Pending[];
  private sending = false;
  private attempts = 0;

  constructor(
    private readonly http: Http,
    private readonly storage: Storage,
    private readonly appId: string,
    private readonly timers: Timers,
    private readonly hooks: Hooks,
  ) {
    this.queue = this.load();
  }

  get pending(): readonly Pending[] {
    return this.queue;
  }

  /*
   * send queues a message and tries it at once.
   */
  send(draft: Omit<Pending, "clientId" | "createdAt">): Pending {
    const pending = { ...draft, clientId: this.timers.uuid(), createdAt: this.timers.now() };
    this.queue.push(pending);
    this.save();
    void this.flush();

    return pending;
  }

  /*
   * flush sends what is queued, oldest first, stopping at the first one
   * that has to wait.
   */
  async flush(): Promise<void> {
    if (this.sending) {
      return;
    }
    this.sending = true;

    try {
      while (this.queue.length > 0) {
        const head = this.queue[0] as Pending;
        if (!(await this.attempt(head))) {
          return;
        }
        this.queue.shift();
        this.save();
      }
    } finally {
      this.sending = false;
    }
  }

  /*
   * attempt sends one message and says whether it is done with, sent or
   * refused for good.
   */
  private async attempt(pending: Pending): Promise<boolean> {
    try {
      await this.hooks.ready();
      const sent = await this.post(pending);
      this.attempts = 0;
      this.hooks.sent(sent);
      return true;
    } catch (error) {
      const failure = error instanceof ApiError ? error : new ApiError(0, "network", String(error));
      if (retryable.has(failure.code) || failure.status >= 500) {
        this.retry(failure.retryAfter);
        return false;
      }
      this.hooks.refused(pending, failure);
      return true;
    }
  }

  private async post(pending: Pending): Promise<Sent> {
    const body = {
      client_id: pending.clientId,
      body: pending.body,
      attachment_ids: pending.attachmentIds,
      ...(pending.choice ? { choice: pending.choice } : {}),
    };

    if (pending.conversation) {
      const res = await this.http.post<{ data: Record<string, unknown>; meta?: Record<string, unknown> }>(
        `/conversations/${pending.conversation}/messages`,
        body,
      );
      const moved = typeof res.data.conversation_id === "string" ? res.data.conversation_id : pending.conversation;

      return { pending, message: res.data, conversation: undefined, conversationId: moved };
    }

    const res = await this.http.post<{ data: Record<string, unknown>; meta: { message: Record<string, unknown> } }>(
      "/conversations",
      body,
    );

    return { pending, message: res.meta.message, conversation: res.data, conversationId: String(res.data.id) };
  }

  private retry(after?: number) {
    this.attempts += 1;
    const delay = after ? after * 1000 : Math.min(maxDelay, 500 * 2 ** (this.attempts - 1));
    this.timers.setTimeout(() => void this.flush(), delay);
  }

  private get key(): string {
    return `dimmi:${this.appId}:outbox`;
  }

  private load(): Pending[] {
    try {
      const stored = JSON.parse(this.storage.get(this.key) ?? "[]");
      return Array.isArray(stored) ? (stored as Pending[]) : [];
    } catch {
      return [];
    }
  }

  private save() {
    if (this.queue.length === 0) {
      this.storage.remove(this.key);
      return;
    }
    this.storage.set(this.key, JSON.stringify(this.queue));
  }
}
