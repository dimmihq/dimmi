/*
 * The few seconds a reply waits before it leaves, so its writer can take
 * it back. The composer shows how long is left with Undo beside it, and
 * Cmd+Z undoes too while the composer is empty. Undoing puts the text back
 * in the reply field. Leaving the conversation lets the reply go.
 */
import { useEffect } from "react";

import { Notification } from "@/components/base/notification/notification";

import type { Held } from "./use-held";

export function UndoToast({ held }: { held: Held }) {
  const { message, undo } = held;
  useEffect(() => {
    if (!message) {
      return;
    }
    const press = (event: KeyboardEvent) => {
      const field = document.activeElement;
      const typing = field instanceof HTMLElement && (field.textContent ?? "").trim() !== "" && field.isContentEditable;
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "z" && !event.shiftKey && !typing) {
        event.preventDefault();
        undo();
      }
    };
    document.addEventListener("keydown", press, true);

    return () => document.removeEventListener("keydown", press, true);
  }, [message, undo]);

  if (held.failure) {
    return <Notification status="error" title={held.failure} dismissible={false} role="alert" className="w-full" />;
  }
  if (!message) {
    return null;
  }

  return (
    <Notification
      status="information"
      title={`Sending in ${held.left} s`}
      description="Undo with ⌘Z"
      dismissible={false}
      role="status"
      aria-label="Reply waiting to send"
      actions={[{ label: "Undo", onClick: undo, variant: "secondary" }]}
      className="w-full"
    />
  );
}
