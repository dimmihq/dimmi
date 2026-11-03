/*
 * The preview of a new message that pops up above the launcher while the
 * messenger is closed: who wrote, when, and the first lines of what they
 * said. The whole card opens the conversation; the small button beside it
 * dismisses the preview and leaves the message unread.
 */
import type { Incoming } from "../content";
import { initials } from "../text";
import { CloseIcon } from "./icons";

export function Notice({ message }: { message: Incoming }) {
  return (
    <>
      <button className="notice-card" type="button" data-action="click->read">
        <span className="face">{initials(message.author ?? "")}</span>
        <span className="text">
          <span className="notice-head">
            <strong>{message.author}</strong>
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
