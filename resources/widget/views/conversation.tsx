/*
 * A conversation: who the visitor is talking to and how soon they reply,
 * the thread, and the composer pinned under it.
 */
import { Tools } from "../components/chrome";
import { Composer } from "../components/composer";
import { BackIcon } from "../components/icons";
import { Loadable, SkeletonThread } from "../components/skeleton";
import { Whoami } from "../components/whoami";
import type { Content } from "../content";
import { thread } from "../lib/threads";
import type { Options } from "../options";

export function Conversation({ options, content }: { options: Options; content: Content }) {
  const ai = { label: options.texts.ai_label, name: options.texts.ai_agent };

  return (
    <>
      <header className="bar chat-bar">
        <button className="icon-button" type="button" data-action="click->back" aria-label="Back">
          <BackIcon />
        </button>
        <Whoami content={content} ai={ai} />
        <Tools />
      </header>
      <div className="thread" data-target="thread">
        <Loadable skeleton={<SkeletonThread />}>
          <p className="day">{options.texts.today}</p>
          {thread()?.Thread({ messages: content.thread, ai }) ?? null}
          <p className="seen">{options.texts.seen}</p>
        </Loadable>
      </div>
      <Composer options={options} />
    </>
  );
}
