/*
 * The messages view: every conversation the visitor has had, or an empty
 * state, with a button to start a new one always at the bottom.
 */
import { Bar } from "../components/chrome";
import { ArrowUpIcon, MessagesIcon } from "../components/icons";
import { ConversationRow } from "../components/rows";
import { Loadable, SkeletonRows } from "../components/skeleton";
import type { Content } from "../content";
import type { Options } from "../options";

export function Messages({ options, content }: { options: Options; content: Content }) {
  const { texts } = options;

  return (
    <>
      <Bar title={texts.messages} />
      <div className="stack fill">
        <Loadable skeleton={<SkeletonRows count={4} round />}>
          {content.conversations.length > 0 ? (
            <div className="list">
              {content.conversations.map((conversation) => (
                <ConversationRow conversation={conversation} key={conversation.number} />
              ))}
            </div>
          ) : (
            <div className="empty">
              <span className="glyph">
                <MessagesIcon />
              </span>
              <strong>{texts.messages_empty_title}</strong>
              <p>{texts.messages_empty_body}</p>
            </div>
          )}
        </Loadable>
      </div>
      <div className="dock">
        <button className="button primary wide" type="button" data-action="click->converse">
          {texts.send_message}
          <ArrowUpIcon />
        </button>
      </div>
    </>
  );
}
