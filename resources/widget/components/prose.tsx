/*
 * Long-form text for articles and changelog entries. Bodies arrive as
 * blocks rather than HTML, so every piece of text is escaped and no
 * customer markup ever reaches the page. Code in a known language comes
 * as tokens marked keyword, string, number or comment, drawn as spans
 * the stylesheet colors. A video or demo opens on its own site rather
 * than playing inside the host page, a button is a link drawn as one,
 * and a collapsible section is the browser's own disclosure. Each page ends by asking what
 * the visitor thought, with a way into a conversation. An article asks
 * whether it helped, and the answer shows as chosen with a thank-you,
 * changeable by picking the other face.
 */
import type { Block as Content, Token } from "../content";
import type { Options } from "../options";
import { ChevronRightIcon, ExternalIcon, ImageIcon, SparkleIcon, VideoIcon } from "./icons";

/*
 * webLink is a link the messenger may open: a web or mail address. The
 * server sends nothing else; this keeps a stray one inert all the same.
 */
function webLink(url: string | undefined): string | null {
  return url && /^(?:https?:\/\/|mailto:)/i.test(url) ? url : null;
}

function Block({ block }: { block: Content }) {
  const { kind, text = "", items = [] } = block;
  const list = items.map((item) => <li key={item}>{item}</li>);

  switch (kind) {
    case "h2":
      return <h3>{text}</h3>;
    case "list":
      return <ul>{list}</ul>;
    case "steps":
      return <ol>{list}</ol>;
    case "code":
      return (
        <pre>
          <code>
            {(block.tokens ?? [{ text }]).map((t: Token) => (
              <span key={t.text} className={t.class}>
                {t.text}
              </span>
            ))}
          </code>
        </pre>
      );
    case "note":
      return (
        <aside className="note">
          <SparkleIcon />
          <span>{text}</span>
        </aside>
      );
    case "embed": {
      const url = webLink(block.url);
      return url ? (
        <a className="row embed" href={url} target="_blank" rel="noopener noreferrer">
          <span className="row-icon">
            <VideoIcon />
          </span>
          <span className="text">
            <span>{text}</span>
            <small>{new URL(url).hostname}</small>
          </span>
          <ExternalIcon />
        </a>
      ) : (
        <p>{text}</p>
      );
    }
    case "button": {
      const url = webLink(block.url);
      return url ? (
        <p className="button-row">
          <a className="button primary" href={url} target="_blank" rel="noopener noreferrer">
            {text}
          </a>
        </p>
      ) : (
        <p>{text}</p>
      );
    }
    case "details":
      return (
        <details className="details">
          <summary>
            <ChevronRightIcon className="chevron" />
            <span>{text}</span>
          </summary>
          <Blocks blocks={block.blocks ?? []} />
        </details>
      );
    case "image":
      return (
        <figure className="figure">
          <span className="art">
            <ImageIcon />
          </span>
          <figcaption>{text}</figcaption>
        </figure>
      );
    default:
      return <p>{text}</p>;
  }
}

function Blocks({ blocks }: { blocks: Content[] }) {
  return (
    <>
      {blocks.map((block) => (
        <Block block={block} key={`${block.kind}-${block.text ?? block.items?.join()}`} />
      ))}
    </>
  );
}

export function Prose({ blocks }: { blocks: Content[] }) {
  return (
    <div className="prose">
      <Blocks blocks={blocks} />
    </div>
  );
}

/*
 * Feedback closes a page. Given an article, it asks whether the article
 * helped, with what the visitor already answered marked.
 */
export function Feedback({
  options,
  prompt,
  article,
}: {
  options: Options;
  prompt: string;
  article?: { slug: string; rated?: "yes" | "no" };
}) {
  return (
    <div className="feedback" data-article={article?.slug} data-rated={article?.rated}>
      {article ? (
        <>
          <p>{prompt}</p>
          <div className="reactions">
            <Face answer="no" label="Not helpful" face="😞" rated={article.rated} />
            <Face answer="yes" label="Helpful" face="😃" rated={article.rated} />
          </div>
          <p className="thanks" role="status">
            {options.texts.rating_thanks}
          </p>
        </>
      ) : null}
      <button className="button secondary" type="button" data-action="click->converse">
        {options.texts.still_need_help}
      </button>
    </div>
  );
}

function Face({ answer, label, face, rated }: { answer: "yes" | "no"; label: string; face: string; rated?: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={String(rated === answer)}
      data-action="click->rate"
      data-helpful={answer}
    >
      {face}
    </button>
  );
}
