/*
 * Long-form text for articles and changelog entries. Bodies arrive as
 * blocks rather than HTML, so every piece of text is escaped and no
 * customer markup ever reaches the page. Each page ends by asking what
 * the visitor thought, with a way into a conversation.
 */
import type { Block as Content } from "../content";
import type { Options } from "../options";
import { ImageIcon, SparkleIcon } from "./icons";

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
          <code>{text}</code>
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
