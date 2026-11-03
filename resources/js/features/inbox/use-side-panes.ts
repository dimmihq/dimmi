/*
 * Opening and closing the side panes of a conversation from its header.
 * The thread is the only pane that gives or takes the room, so closing
 * Details never widens Operator and opening it never squeezes Operator.
 * A pane reopens at the size it last had open, or at its default size;
 * the slivers a pane passes through while closing are not remembered.
 */
import { useRef } from "react";
import { type Layout, type PanelSize, useGroupRef } from "react-resizable-panels";

/*
 * useSidePanes moves the side panes of a group whose growing pane is
 * named thread. defaults holds each side pane's opening size in pixels,
 * and width reads the group's width to turn them into shares.
 */
export function useSidePanes(thread: string, defaults: Record<string, number>, width: () => number) {
  const group = useGroupRef();
  const last = useRef<Record<string, number>>({});

  const remember = (id: string) => (size: PanelSize) => {
    if (size.inPixels >= (defaults[id] ?? 0) / 2) {
      last.current[id] = size.asPercentage;
    }
  };

  const toggle = (id: string) => {
    const handle = group.current;
    if (!handle) {
      return;
    }
    const layout: Layout = { ...handle.getLayout() };
    const now = layout[id] ?? 0;
    const room = layout[thread] ?? 0;
    const fallback = ((defaults[id] ?? 0) / Math.max(width(), 1)) * 100;
    const opening = now > 0 ? 0 : Math.min(last.current[id] ?? fallback, room);
    layout[thread] = room + now - opening;
    layout[id] = opening;
    handle.setLayout(layout);
  };

  return { group, remember, toggle };
}
