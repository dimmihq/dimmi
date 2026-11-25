/*
 * The preview of a new message that pops up above the launcher while the
 * messenger is closed: who wrote, marked when it was the AI agent, when,
 * and the first lines of what they said. The whole card opens the conversation; the small button beside it
 * dismisses the preview and leaves the message unread.
 */
import type { Incoming } from "../content";
import { type AiTexts, aiTexts, initials } from "../text";
import { AiBadge } from "./ai-badge";
import { CloseIcon } from "./icons";

export function Notice({ message, ai = aiTexts }: { message: Incoming; ai?: AiTexts }) {
  return (
    <>
      <button className="notice-card" type="button" data-action="click->read">
        <span className="face">{initials(message.author ?? "")}</span>
        <span className="text">
          <span className="notice-head">
            <span className="notice-who">
              <strong>{message.author}</strong>
              {message.from === "agent" ? <AiBadge texts={ai} /> : null}
            </span>
            <time>{message.at}</time>
          </span>
          <span className="notice-body">{message.body}</span>
        </span>
      </button>
      <button className="icon-button notice-close" type="button" data-action="click->ignore" aria-label="Dismiss">
        <CloseIcon />
      </button>
    </>
  );
}
