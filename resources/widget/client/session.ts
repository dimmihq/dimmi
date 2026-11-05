/*
 * The visitor's session with one widget. A page load with no stored token
 * makes no request at all: the session starts only when the visitor first
 * does something, such as writing. A stored token is resumed once; when
 * the server says it is unknown, expired or ended, it is dropped and the
 * visitor carries on as new, which is what the server expects. Each widget
 * keeps its own token under its app id, so two widgets on one site never
 * share a visitor.
 */
import { ApiError, type Http } from "./http";

export interface Storage {
  get(key: string): string | null;
  set(key: string, value: string): void;
  remove(key: string): void;
}

export interface SessionContact {
  id: string;
  name: string | null;
  email: string | null;
}

export interface SessionData {
  id: string;
  token?: string;
  kind: string;
  unread_count?: number;
  contact: SessionContact;
}

export interface StartDetails {
  locale?: string;
  timezone?: string;
  page?: { url: string; title?: string; referrer?: string };
  website?: string;
}

/*
 * Ended is every code that means the stored token is no use any more.
 */
const ended = new Set(["session_invalid", "session_expired", "session_revoked", "identity_proof_required"]);

export class Session {
  current: SessionData | undefined;
  private starting: Promise<SessionData> | undefined;

  constructor(
    private readonly http: Http,
    private readonly storage: Storage,
    private readonly appId: string,
  ) {}

  private get key(): string {
    return `dimmi:${this.appId}:token`;
  }

  /*
   * resume picks up a stored session, or answers undefined when there is
   * none or it ended.
   */
  async resume(): Promise<SessionData | undefined> {
    const token = this.storage.get(this.key);
    if (!token) {
      return undefined;
    }

    this.http.token = token;
    try {
      const res = await this.http.get<{ data: SessionData }>("/session");
      this.current = res.data;
      return res.data;
    } catch (error) {
      if (error instanceof ApiError && ended.has(error.code)) {
        this.forget();
        return undefined;
      }
      throw error;
    }
  }

  /*
   * ensure returns the session, starting one the first time the visitor
   * needs it. Several callers at once share one start.
   */
  ensure(details: StartDetails = {}): Promise<SessionData> {
    if (this.current) {
      return Promise.resolve(this.current);
    }

    this.starting ??= this.start(details).finally(() => {
      this.starting = undefined;
    });

    return this.starting;
  }

  /*
   * signOut ends the session on the server and forgets it here.
   */
  async signOut(): Promise<void> {
    if (this.http.token) {
      await this.http.delete("/session").catch(() => undefined);
    }
    this.forget();
  }

  /*
   * forget drops the session without asking the server, for when the
   * server already said it ended.
   */
  forget() {
    this.current = undefined;
    this.http.token = undefined;
    this.storage.remove(this.key);
  }

  private async start(details: StartDetails): Promise<SessionData> {
    const res = await this.http.post<{ data: SessionData }>("/sessions", { app_id: this.appId, ...details });
    const session = res.data;
    if (session.token) {
      this.http.token = session.token;
      this.storage.set(this.key, session.token);
    }
    this.current = session;

    return session;
  }
}
