/*
 * Telling teammates who have the conversation open that the person is
 * writing a reply. While they type, a report goes out at most every four
 * seconds, so the indicator stays up without a request per key; sending
 * the reply or leaving the field says they stopped. Notes are never
 * reported: the customer never sees them and teammates do not wait on
 * them. A report that fails is dropped, since the next keystroke sends
 * another.
 */
import { useCallback, useEffect, useRef } from "react";

import { sendTyping } from "@/lib/api/inbox";

export const typingInterval = 4000;

/*
 * Report says whether the person is writing a reply right now.
 */
export type Report = (typing: boolean) => void;

/*
 * reporter returns a Report that sends start at most once per interval
 * while typing and stop once after a start, through send.
 */
export function reporter(send: (state: "start" | "stop") => void, now: () => number = Date.now): Report {
  let last = 0;

  return (typing) => {
    if (typing) {
      if (now() - last >= typingInterval) {
        last = now();
        send("start");
      }
      return;
    }

    if (last !== 0) {
      last = 0;
      send("stop");
    }
  };
}

/*
 * useTypingReport returns the Report of the conversation, which says it
 * stopped when the conversation closes.
 */
export function useTypingReport(workspace: string, conversation: string): Report {
  const report = useRef<Report>(() => undefined);

  useEffect(() => {
    const current = reporter((state) => {
      sendTyping(workspace, conversation, state).catch(() => undefined);
    });
    report.current = current;

    return () => current(false);
  }, [workspace, conversation]);

  return useCallback((typing: boolean) => report.current(typing), []);
}
