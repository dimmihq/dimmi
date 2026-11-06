/*
 * A conversation's messages, grouped by sender the way people read chat:
 * one name and face over a run of bubbles from the same person and one
 * time under it. Visitors' bubbles sit on the right in the brand color, the
 * team's and the agent's on the left, the agent marked as AI with the
 * articles it drew on. Text, files, images and voice notes each have their
 * own bubble. The sample thread and every message a visitor sends render
 * through these components, so they always look alike.
 */
import { raw } from "widget-jsx/jsx-runtime";

import type { Attachment, Message, Source } from "../content";
import { markdown } from "../lib/markdown";
import { initials, levels } from "../text";
import { BookIcon, CheckIcon, DownloadIcon, FileIcon, ImageIcon, Mark, PauseIcon, PersonIcon, PlayIcon } from "./icons";

export function Waveform({ className = "wave" }: { className?: string }) {
  return <span className={className}>{raw(levels.map((level) => `<i data-height="${level}"></i>`).join(""))}</span>;
}

/*
 * VoiceBubble is a voice note's player, playing the recording once it has
 * an address. A click on the waveform jumps to that point.
 */
function VoiceBubble({ attachment }: { attachment: Attachment }) {
  return (
    <div className="bubble voice" data-controller="voice" data-length={attachment.duration ?? "0:05"}>
      <button className="play" type="button" data-action="click->toggle" aria-label="Play voice note">
        <PlayIcon />
        <PauseIcon />
      </button>
      <span className="wave-hit" data-action="click->seek">
        <Waveform />
      </span>
      <time>{attachment.duration ?? "0:00"}</time>
      {attachment.url ? (
        <audio src={attachment.url} preload="metadata">
          <track kind="captions" />
        </audio>
      ) : null}
    </div>
  );
}

/*
 * ImageBubble shows the picture itself once it has an address, sized from
 * its stored dimensions so the thread does not jump when it loads, and
 * opens it across the messenger when clicked. Until then, and in a preview, it shows
 * the placeholder.
 */
function ImageBubble({ attachment }: { attachment: Attachment }) {
  if (!attachment.url) {
    return (
      <figure className="bubble photo">
        <span className="shot">
          <ImageIcon />
        </span>
        <figcaption>{attachment.name}</figcaption>
      </figure>
    );
  }

  return (
    <figure className="bubble photo">
      <a
        className="shot picture"
        href={attachment.url}
        target="_blank"
        rel="noopener noreferrer"
        data-action="click->view"
      >
        <img
          src={attachment.url}
          alt={attachment.name}
          loading="lazy"
          width={attachment.width}
          height={attachment.height}
        />
      </a>
    </figure>
  );
}

function FileBubble({ attachment }: { attachment: Attachment }) {
  return (
    <div className="bubble file">
      <span className="file-icon">
        <FileIcon />
      </span>
      <span className="text">
        <strong>{attachment.name}</strong>
        <small>{attachment.size}</small>
      </span>
      {attachment.url ? (
        <a
          className="icon-button"
          href={attachment.url}
          download={attachment.name}
          target="_blank"
          rel="noopener noreferrer"
          aria-label={`Download ${attachment.name}`}
        >
          <DownloadIcon />
        </a>
      ) : null}
    </div>
  );
}

function AttachmentBubble({ attachment }: { attachment: Attachment }) {
  if (attachment.kind === "voice") {
    return <VoiceBubble attachment={attachment} />;
  }

  return attachment.kind === "image" ? <ImageBubble attachment={attachment} /> : <FileBubble attachment={attachment} />;
}

/*
 * keyOf names a message for the patch that keeps unchanged messages in
 * place.
 */
function keyOf(message: Message): string {
  return message.key ?? message.id ?? `${message.at}-${message.body ?? message.attachment?.name ?? ""}`;
}

/*
 * SourceChip is an article an answer cites, opening it in a new tab when
 * it has an address.
 */
function SourceChip({ source }: { source: Source }) {
  if (!source.url) {
    return (
      <span className="source">
        <BookIcon />
        <span className="source-title">{source.title}</span>
      </span>
    );
  }

  return (
    <a className="source" href={source.url} target="_blank" rel="noopener noreferrer">
      <BookIcon />
      <span className="source-title">{source.title}</span>
    </a>
  );
}

function Bubble({ message }: { message: Message }) {
  const sources = message.sources ?? [];
  const key = keyOf(message);

  return (
    <>
      {message.body ? (
        <div
          className={message.from === "visitor" ? "bubble" : "bubble rich"}
          data-key={`${key}:body`}
          data-state={message.state}
        >
          {message.from === "visitor" ? message.body : raw(markdown(message.body))}
          {sources.length > 0 && (
            <span className="sources">
              {sources.map((source) => (
                <SourceChip source={source} key={source.title} />
              ))}
            </span>
          )}
        </div>
      ) : null}
      {message.attachment ? (
        <div className="bubble-wrap" data-key={`${key}:file`}>
          <AttachmentBubble attachment={message.attachment} />
        </div>
      ) : null}
    </>
  );
}

function Who({ message }: { message: Message }) {
  const agent = message.from === "agent";

  return (
    <div className="who">
      <span className="face">
        {agent && message.avatar ? (
          <img src={message.avatar} alt="" />
        ) : agent ? (
          <Mark />
        ) : (
          initials(message.author ?? "")
        )}
      </span>
      <span>{message.author}</span>
      {agent ? <em className="badge">AI</em> : null}
    </div>
  );
}

export function Said({ messages }: { messages: Message[] }) {
  const [first] = messages;
  const last = messages.at(-1);
  if (!first || !last) {
    return raw("");
  }

  return (
    <div className="said" data-from={first.from} data-key={`said:${keyOf(first)}`}>
      {first.from !== "visitor" && <Who message={first} />}
      <div className="bubbles">
        {messages.map((message) => (
          <Bubble message={message} key={`${message.at}-${message.body ?? message.attachment?.name}`} />
        ))}
      </div>
      <time>{last.at}</time>
    </div>
  );
}

/*
 * runs groups consecutive messages from the same sender.
 */
function runs(messages: Message[]): Message[][] {
  const grouped: Message[][] = [];
  for (const message of messages) {
    const run = grouped.at(-1);
    const previous = run?.[0];
    if (run && previous?.from === message.from && previous.author === message.author) {
      run.push(message);
    } else {
      grouped.push([message]);
    }
  }

  return grouped;
}

export function Thread({ messages }: { messages: Message[] }) {
  return (
    <>
      {runs(messages).map((run) => (
        <Said messages={run} key={`${run[0]?.at}-${run[0]?.from}-${run.length}`} />
      ))}
    </>
  );
}

/*
 * AiActionTexts are the words of the two actions under an answer from
 * the agent, and the thanks once the visitor said it helped.
 */
export interface AiActionTexts {
  helped: string;
  handoff: string;
  thanks: string;
}

/*
 * AiActions are the quiet choices under the agent's latest reply: that
 * it helped, offered only under a real answer, and to talk to a person
 * instead. Once the visitor says it helped, a thanks takes their place.
 */
export function AiActions({ texts, helped, answered }: { texts: AiActionTexts; helped: boolean; answered: boolean }) {
  if (helped) {
    return (
      <p className="ai-thanks" data-key="ai-actions">
        <CheckIcon />
        {texts.thanks}
      </p>
    );
  }

  return (
    <div className="ai-actions" data-key="ai-actions">
      {answered ? (
        <button type="button" className="ai-action" data-ai="helped">
          <CheckIcon />
          {texts.helped}
        </button>
      ) : null}
      <button type="button" className="ai-action" data-ai="handoff">
        <PersonIcon />
        {texts.handoff}
      </button>
    </div>
  );
}

export function Typing() {
  return (
    <div className="said" data-from="team" data-typing data-key="typing">
      <div className="bubbles">
        <div className="bubble dots" role="status" aria-label="Typing">
          <i />
          <i />
          <i />
        </div>
      </div>
    </div>
  );
}
