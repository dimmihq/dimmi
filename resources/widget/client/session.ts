/*
 * The visitor's session with one widget. A page load with no stored token
 * makes no request at all: the session starts only when the visitor first
 * does something, such as writing. A stored token is resumed once; when
 * the server says it is unknown, expired or ended, it is dropped and the
 * visitor carries on as new, which is what the server expects. Each widget
 * keeps its own token under its app id, so two widgets on one site never
 * share a visitor.
 *
 * A signed-in visitor's page hands over an identity token instead. A
 * session signed in as the same customer is refreshed with it; any other
 * session is handed over to a new signed-in one, which the server makes
 * from the token. Only a hash of the customer's id is stored, to tell on
 * the next page whether the token names the same person. When the page
 * says nobody is signed in any more, a signed-in session is ended at
 * once, so the next person on a shared computer starts fresh. A token the
 * server refuses changes nothing: the visitor carries on as they were and
 * the console says why.
 */

import { report } from "../lib/report";
import type { Surface } from "../lib/rules";
import { ApiError, type Http } from "./http";
import { fingerprint, subjectOf } from "./identity";

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
  expires_at?: string;
  unread_count?: number;
  contact: SessionContact;
  surface?: Surface;
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

  private get subKey(): string {
    return `dimmi:${this.appId}:sub`;
  }

  /*
   * signedIn reports whether the stored session is a signed-in one.
   */
  get signedIn(): boolean {
    return this.storage.get(this.subKey) !== null;
  }

  /*
   * identify runs the page-load handshake for a page with an identity
   * source: a token signs the visitor in or refreshes their session,
   * null signs a signed-in visitor out. It answers the session, or
   * undefined when the visitor has none yet.
   */
  async identify(token: string | null, details: StartDetails = {}): Promise<SessionData | undefined> {
    if (token === null) {
      if (this.signedIn) {
        await this.signOut();
        return undefined;
      }
      return this.current ?? this.resume();
    }
    const sub = subjectOf(token);
    const same = sub !== undefined && this.storage.get(this.subKey) === (await fingerprint(sub));
    if (same && this.storage.get(this.key)) {
      const refreshed = await this.refresh(token);
      if (refreshed) {
        return refreshed;
      }
    }
    return this.signIn(token, sub, details);
  }

  /*
   * sameCustomer reports whether the token names the customer the
   * stored session is signed in as.
   */
  async sameCustomer(token: string): Promise<boolean> {
    const sub = subjectOf(token);
    return sub !== undefined && this.storage.get(this.subKey) === (await fingerprint(sub));
  }

  private async refresh(token: string): Promise<SessionData | undefined> {
    this.http.token = this.storage.get(this.key) ?? undefined;
    try {
      const res = await this.http.patch<{ data: SessionData }>("/session", { user_jwt: token });
      this.current = res.data;
      return res.data;
    } catch (error) {
      if (error instanceof ApiError && (ended.has(error.code) || error.code === "identity_mismatch")) {
        return undefined;
      }
      this.refused(error);
      return this.current;
    }
  }

  private async signIn(token: string, sub: string | undefined, details: StartDetails) {
    const previous = this.storage.get(this.key) ?? undefined;
    try {
      const res = await this.http.post<{ data: SessionData }>("/sessions", {
        app_id: this.appId,
        user_jwt: token,
        ...(previous ? { previous_token: previous } : {}),
        ...details,
      });
      const session = res.data;
      if (session.token) {
        this.http.token = session.token;
        this.storage.set(this.key, session.token);
      }
      if (sub !== undefined) {
        this.storage.set(this.subKey, await fingerprint(sub));
      }
      this.current = session;
      return session;
    } catch (error) {
      this.refused(error);
      if (this.signedIn) {
        await this.signOut();
        return undefined;
      }
      return this.current ?? this.resume();
    }
  }

  /*
   * refused tells the developer why a token was turned down, with the
   * server's hint.
   */
  private refused(error: unknown) {
    if (!(error instanceof ApiError)) {
      throw error;
    }
    const hint = typeof error.body.hint === "string" ? ` ${error.body.hint}` : "";
    report("error", error.code, `${error.message}${hint}`);
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
    this.http.token ??= this.storage.get(this.key) ?? undefined;
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
    this.storage.remove(this.subKey);
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
