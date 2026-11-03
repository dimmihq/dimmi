/*
 * A pane that opens or closes from a button eases to its new size, while
 * a pane being dragged follows the pointer directly. This switches the
 * easing on just long enough for one such move.
 */
import { useState } from "react";

/*
 * useEasing turns the open and close motion on for one move of a pane.
 */
export function useEasing() {
  const [easing, setEasing] = useState(false);

  const ease = (move: () => void) => {
    setEasing(true);
    move();
    window.setTimeout(() => setEasing(false), 320);
  };

  return [easing, ease] as const;
}
