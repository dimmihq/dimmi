/*
 * A conversation: who the visitor is talking to and how soon they reply,
 * the thread, and the composer pinned under it.
 */
import { Tools } from "../components/chrome";
import { Composer } from "../components/composer";
import { Faces } from "../components/faces";
import { BackIcon } from "../components/icons";
import { Loadable, SkeletonThread } from "../components/skeleton";
import { Thread } from "../components/thread";
import type { Content } from "../content";
import type { Options } from "../options";

export function Conversation({ options, content }: { options: Options; content: Content }) {
  return (
    <>
      <header className="bar chat-bar">
        <button className="icon-button" type="button" data-action="click->back" aria-label="Back">
          <BackIcon />
        </button>
        <span className="whoami">
          <Faces team={content.team} className="small" />
          <span className="text">
            <strong>{content.company}</strong>
            <small>
              <i className="online" />
              {content.reply_time}
            </small>
          </span>
        </span>
        <Tools />
      </header>
      <div className="thread" data-target="thread">
        <Loadable skeleton={<SkeletonThread />}>
          <p className="day">Today</p>
          <Thread messages={content.thread} />
          <p className="seen">Seen</p>
        </Loadable>
      </div>
      <Composer options={options} />
    </>
  );
}
