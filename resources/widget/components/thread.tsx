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

import type { Attachment, Message } from "../content";
import { initials, levels } from "../text";
import { BookIcon, DownloadIcon, FileIcon, ImageIcon, Mark, PauseIcon, PlayIcon } from "./icons";

export function Waveform({ className = "wave" }: { className?: string }) {
  return <span className={className}>{raw(levels.map((level) => `<i data-height="${level}"></i>`).join(""))}</span>;
}

function VoiceBubble({ attachment }: { attachment: Attachment }) {
  return (
    <div className="bubble voice" data-controller="voice" data-length={attachment.duration ?? "0:05"}>
      <button className="play" type="button" data-action="click->toggle" aria-label="Play voice note">
        <PlayIcon />
        <PauseIcon />
      </button>
      <Waveform />
      <time>{attachment.duration}</time>
    </div>
  );
}

function ImageBubble({ attachment }: { attachment: Attachment }) {
  return (
    <figure className="bubble photo">
      <span className="shot">
        <ImageIcon />
      </span>
      <figcaption>{attachment.name}</figcaption>
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
      <button className="icon-button" type="button" aria-label={`Download ${attachment.name}`}>
        <DownloadIcon />
      </button>
    </div>
  );
}

function AttachmentBubble({ attachment }: { attachment: Attachment }) {
  if (attachment.kind === "voice") {
    return <VoiceBubble attachment={attachment} />;
  }

  return attachment.kind === "image" ? <ImageBubble attachment={attachment} /> : <FileBubble attachment={attachment} />;
}

function Bubble({ message }: { message: Message }) {
  const sources = message.sources ?? [];

  return (
    <>
      {message.body ? (
        <div className="bubble">
          {message.body}
          {sources.length > 0 && (
            <span className="sources">
              {sources.map((source) => (
                <span className="source" key={source.title}>
                  <BookIcon />
                  {source.title}
                </span>
              ))}
            </span>
          )}
        </div>
      ) : null}
      {message.attachment ? <AttachmentBubble attachment={message.attachment} /> : null}
    </>
  );
}

function Who({ message }: { message: Message }) {
  const agent = message.from === "agent";

  return (
    <div className="who">
      <span className="face">{agent ? <Mark /> : initials(message.author ?? "")}</span>
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
    <div className="said" data-from={first.from}>
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

export function Typing() {
  return (
    <div className="said" data-from="team" data-typing>
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
