/*
 * Which of the agent's stories is showing, and how far along it is. A
 * page can drive tabs from the same state the conversation card plays.
 */
import { useEffect, useState } from "react";

import { STORIES } from "./agent-stories";
import { useStartInView } from "./use-scene";

const FRAME_MS = 1500;
const HOLD_MS = 2800;

export function useAgentStories() {
  const { ref, started, inView } = useStartInView();
  const [state, setState] = useState({ active: 0, frame: STORIES[0].frames.length - 1 });

  // The first time the card is seen, rewind the first story and play.
  useEffect(() => {
    if (started) {
      setState({ active: 0, frame: 0 });
    }
  }, [started]);

  // One frame per tick; after the last frame, hold, then the next story.
  useEffect(() => {
    if (!started || !inView) {
      return;
    }
    const last = state.frame >= STORIES[state.active].frames.length - 1;
    const timer = setTimeout(
      () =>
        setState((s) => (last ? { active: (s.active + 1) % STORIES.length, frame: 0 } : { ...s, frame: s.frame + 1 })),
      last ? HOLD_MS : FRAME_MS,
    );
    return () => clearTimeout(timer);
  }, [state, started, inView]);

  const choose = (i: number) => setState({ active: i, frame: started ? 0 : STORIES[i].frames.length - 1 });
  const duration = STORIES[state.active].frames.length * FRAME_MS + HOLD_MS;

  return { ref, started, state, choose, duration };
}
