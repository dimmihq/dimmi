/*
 * The rating card of a closed conversation as a connected messenger
 * runs it: the latest ask of each conversation, loaded the first time
 * the visitor opens it closed, the face and comment they send, and the
 * card's own stylesheet. It is a chunk of its own, fetched only once a
 * visitor opens a closed conversation, so it costs the core nothing.
 */
import type { Http } from "../client/http";
import { ApiError } from "../client/http";
import { RatingCard, type RatingTexts, type WidgetRating } from "../components/rating";
import styles from "../styles/rating.css?inline";
import { report } from "./report";

const styled = new WeakSet<ShadowRoot>();

export class Ratings {
  private readonly asks = new Map<string, WidgetRating | null>();
  private readonly commented = new Set<string>();

  constructor(
    private readonly http: Http,
    root: ShadowRoot,
    private readonly redraw: (conversation: string) => void,
  ) {
    if (!styled.has(root)) {
      const sheet = new CSSStyleSheet();
      sheet.replaceSync(styles);
      root.adoptedStyleSheets = [...root.adoptedStyleSheets, sheet];
      styled.add(root);
    }
  }

  /*
   * forget drops what is known of a conversation's ask, after a new ask
   * went out or the conversation opened again.
   */
  forget(conversation: string) {
    this.asks.delete(conversation);
  }

  /*
   * card is the conversation's rating card, empty while its ask loads or
   * when none went out.
   */
  card(conversation: string, texts: RatingTexts): string {
    const ask = this.asks.get(conversation);
    if (ask === undefined) {
      this.load(conversation);
      return "";
    }
    if (!ask) {
      return "";
    }

    return String(RatingCard({ rating: ask, texts, commented: this.commented.has(ask.id) }));
  }

  /*
   * rate sends the face or the comment. The card changes at once; a
   * refusal puts it back.
   */
  async rate(conversation: string, answer: { score?: number; comment?: string }) {
    const before = this.asks.get(conversation);
    if (!before) {
      return;
    }
    if (answer.score !== undefined) {
      this.asks.set(conversation, { ...before, score: answer.score });
    }
    if (answer.comment !== undefined) {
      this.commented.add(before.id);
    }
    this.redraw(conversation);
    try {
      const res = await this.http.patch<{ data: WidgetRating }>(`/conversations/${conversation}/rating`, answer);
      this.asks.set(conversation, res.data);
    } catch (error) {
      this.asks.set(conversation, before);
      this.commented.delete(before.id);
      report("warn", error instanceof ApiError ? error.code : "network", "the rating could not be saved.");
    }
    this.redraw(conversation);
  }

  private load(conversation: string) {
    this.asks.set(conversation, null);
    this.http
      .get<{ data: WidgetRating | null }>(`/conversations/${conversation}/rating`)
      .then((res) => {
        this.asks.set(conversation, res.data);
        if (res.data) {
          this.redraw(conversation);
        }
      })
      .catch(() => this.asks.delete(conversation));
  }
}
