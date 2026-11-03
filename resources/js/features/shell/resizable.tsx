/*
 * Resizable panes, built on react-resizable-panels. A page lays its
 * floating panels out in a PaneGroup and drags them apart on the gap
 * between them, where a slim handle lights up on hover. Each browser
 * remembers the sizes it chose. A pane that opens or closes from a button
 * eases to its new size on BoardUI's curve, while dragging stays direct.
 */

import type { ReactNode, Ref } from "react";
import {
  Group,
  type GroupImperativeHandle,
  type LayoutStorage,
  Separator,
  useDefaultLayout,
} from "react-resizable-panels";

import { cx } from "@/utils/cx";

/*
 * browserStorage keeps layouts in localStorage, and forgets them quietly
 * where storage is blocked.
 */
const browserStorage: LayoutStorage = {
  getItem: (key) => {
    try {
      return window.localStorage.getItem(key);
    } catch {
      return null;
    }
  },
  setItem: (key, value) => {
    try {
      window.localStorage.setItem(key, value);
    } catch {
      return;
    }
  },
};

/*
 * PaneGroup lays panes side by side and remembers their sizes under id.
 * panelIds names the panes currently shown, so each arrangement keeps its
 * own layout. easing switches on the open and close motion; groupRef
 * hands out the group for moves made from code.
 */
export function PaneGroup({
  id,
  panelIds,
  easing = false,
  groupRef,
  className,
  children,
}: {
  id: string;
  panelIds: string[];
  easing?: boolean;
  groupRef?: Ref<GroupImperativeHandle | null>;
  className?: string;
  children: ReactNode;
}) {
  const layout = useDefaultLayout({ id, panelIds, storage: browserStorage });

  return (
    <Group
      id={id}
      groupRef={groupRef}
      defaultLayout={layout.defaultLayout}
      onLayoutChanged={layout.onLayoutChanged}
      className={cx(
        "h-full min-h-0 min-w-0 flex-1",
        easing &&
          "[&>[data-panel]]:transition-[flex-grow] [&>[data-panel]]:duration-300 [&>[data-panel]]:ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:[&>[data-panel]]:transition-none",
        className,
      )}
    >
      {children}
    </Group>
  );
}

/*
 * PaneHandle is the gap between two panes, and the grip for resizing
 * them. hidden folds the gap away while the pane beside it is closed; the
 * handle stays enabled underneath, since a pane whose only handle is
 * disabled cannot be opened again from code.
 */
export function PaneHandle({ hidden = false }: { hidden?: boolean }) {
  return (
    <Separator
      className={cx(
        "group/handle flex items-center justify-center outline-none",
        hidden ? "pointer-events-none w-0" : "w-3",
      )}
    >
      <span
        aria-hidden
        className={cx(
          "h-10 w-1 rounded-full bg-border-button-hover opacity-0 transition-opacity duration-150",
          "group-data-[separator=hover]/handle:opacity-100 group-data-[separator=active]/handle:opacity-100",
          "group-data-[separator=focus]/handle:opacity-100",
          hidden && "hidden",
        )}
      />
    </Separator>
  );
}
