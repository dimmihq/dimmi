/*
 * useFollowNewest keeps a thread on its newest message, the way a chat
 * reads. A conversation opens at its end. While the reader sits near
 * the end, the thread follows whatever lands, step by step as a new
 * message grows into place; once they scroll up to read, it leaves them
 * where they are. Sending a message brings them back to the end, so
 * what they sent lands in view.
 */
import { type RefObject, useCallback, useEffect, useLayoutEffect, useRef } from "react";

/*
 * nearEnd is how close to the end, in pixels, still counts as reading
 * the newest message.
 */
const nearEnd = 96;

export function useFollowNewest(
  scroller: RefObject<HTMLDivElement | null>,
  content: HTMLElement | null,
  conversation: string,
  ready: boolean,
) {
  const following = useRef(true);

  useLayoutEffect(() => {
    const element = scroller.current;
    if (!ready || !element || conversation === "") {
      return;
    }
    following.current = true;
    element.scrollTop = element.scrollHeight;
  }, [scroller, conversation, ready]);

  useEffect(() => {
    const element = scroller.current;
    if (!ready || !element || !content) {
      return;
    }
    const onScroll = () => {
      following.current = element.scrollHeight - element.scrollTop - element.clientHeight < nearEnd;
    };
    const observer = new ResizeObserver(() => {
      if (following.current) {
        element.scrollTop = element.scrollHeight;
      }
    });
    element.addEventListener("scroll", onScroll, { passive: true });
    observer.observe(content);

    return () => {
      element.removeEventListener("scroll", onScroll);
      observer.disconnect();
    };
  }, [scroller, content, ready]);

  return useCallback(() => {
    following.current = true;
    const element = scroller.current;
    if (element) {
      element.scrollTop = element.scrollHeight;
    }
  }, [scroller]);
}
