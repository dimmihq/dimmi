/*
 * Tells the server, every half minute while the dashboard is visible,
 * which conversation it shows, so no email or push arrives about what
 * the member is already looking at. Hiding the tab says so at once.
 * The server forgets a beat on its own after about a minute, so a
 * closed laptop never silences anyone for long.
 */
import { useEffect } from "react";

import { focusNotifications } from "@/lib/api/notifications";

const beat = 30_000;

/*
 * conversationIn reads the conversation a dashboard address shows.
 */
export function conversationIn(path: string): string | undefined {
  return /\/inbox\/[^/]+\/(conv_[0-9a-z]+)/.exec(path)?.[1];
}

export function useNotificationFocus(workspace: string | null, path: string) {
  useEffect(() => {
    if (!workspace) {
      return;
    }
    const conversation = conversationIn(path);
    const send = () => {
      const visible = document.visibilityState === "visible";
      void focusNotifications(workspace, visible, visible ? conversation : undefined).catch(() => undefined);
    };
    send();
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") {
        send();
      }
    }, beat);
    document.addEventListener("visibilitychange", send);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", send);
    };
  }, [workspace, path]);
}
