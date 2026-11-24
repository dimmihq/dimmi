import { useRef } from "react";

/*
 * useHeading says whether the tab picked now sits after the one before
 * it, so its view slides in from the side the person is heading.
 */
export function useHeading<T>(order: readonly { id: T }[], current: T) {
  const last = useRef(current);
  const forward = useRef(true);
  if (last.current !== current) {
    const at = (id: T) => order.findIndex((item) => item.id === id);
    forward.current = at(current) > at(last.current);
    last.current = current;
  }

  return forward.current;
}
