/*
 * Long-form text for articles and changelog entries. Bodies arrive as
 * blocks rather than HTML, so every piece of text is escaped and no
 * customer markup ever reaches the page. Code in a known language comes
 * as tokens, drawn as spans whose classes the stylesheet colors. Each page ends by asking what
 * the visitor thought, with a way into a conversation.
 */
import type { Block as Content, Token } from "../content";
import type { Options } from "../options";
import { ImageIcon, SparkleIcon } from "./icons";

/*
 * Code draws code, its highlighted tokens as spans keyed by where each
 * starts in the code, or the plain text when it has none.
 */
function Code({ text, tokens }: { text: string; tokens: Token[] }) {
  if (tokens.length === 0) {
    return <code>{text}</code>;
  }
  let at = 0;
  const runs = tokens.map((token) => {
    const start = at;
    at += token.text.length;
    return { start, token };
  });
  return (
    <code>
      {runs.map(({ start, token }) => (
        <span key={start} className={token.class ? `chroma-${token.class}` : undefined}>
          {token.text}
        </span>
      ))}
    </code>
  );
}

function Block({ block }: { block: Content }) {
  const { kind, text = "", items = [], tokens = [] } = block;
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
          <Code text={text} tokens={tokens} />
        </pre>
      );
    case "note":
      return (
        <aside className="note">
          <SparkleIcon />
          <span>{text}</span>
        </aside>
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

export function Prose({ blocks }: { blocks: Content[] }) {
  return (
    <div className="prose">
      {blocks.map((block) => (
        <Block block={block} key={`${block.kind}-${block.text ?? block.items?.join()}`} />
      ))}
    </div>
  );
}

export function Feedback({ options, prompt }: { options: Options; prompt: string }) {
  return (
    <div className="feedback">
      <p>{prompt}</p>
      <div className="reactions">
        <button type="button" aria-label="Not helpful">
          😞
        </button>
        <button type="button" aria-label="Somewhat">
          😐
        </button>
        <button type="button" aria-label="Helpful">
          😃
        </button>
      </div>
      <button className="button secondary" type="button" data-action="click->converse">
        {options.texts.still_need_help}
      </button>
    </div>
  );
}
