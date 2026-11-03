/*
 * Whether the screen is at least a given width, following resizes, so a
 * panel can sit beside the thread when there is room and slide over it
 * when there is not.
 */
import { useSyncExternalStore } from "react";

export function useWide(minWidth: number): boolean {
  const query = `(min-width: ${minWidth}px)`;

  return useSyncExternalStore(
    (change) => {
      const media = window.matchMedia(query);
      media.addEventListener("change", change);

      return () => media.removeEventListener("change", change);
    },
    () => window.matchMedia(query).matches,
  );
}
