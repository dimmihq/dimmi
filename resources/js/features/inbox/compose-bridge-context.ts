/*
 * The way into the reply box from beside it. Operator drafts replies in
 * its own pane; "Use in reply" puts a draft into the conversation's reply
 * box, switching it to reply, where the teammate edits and sends it as
 * their own. The composer registers itself while it is on screen; with
 * none open, nothing is offered.
 */
import { createContext, useContext, useEffect, useRef } from "react";

export type Put = (text: string) => void;

export interface Bridge {
  put: Put | null;
  register: (put: Put | null) => void;
}

export const ComposeBridgeContext = createContext<Bridge>({ put: null, register: () => undefined });

/*
 * useComposeTarget registers the reply box, which takes text with put.
 */
export function useComposeTarget(put: Put) {
  const { register } = useContext(ComposeBridgeContext);
  const latest = useRef(put);
  latest.current = put;

  useEffect(() => {
    register((text) => latest.current(text));

    return () => register(null);
  }, [register]);
}

/*
 * useCompose returns what puts text into the reply box, null when none
 * is open.
 */
export function useCompose(): Put | null {
  return useContext(ComposeBridgeContext).put;
}
