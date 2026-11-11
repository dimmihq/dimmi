/*
 * A side pane that pushes the page aside instead of covering it, the way
 * Operator sits beside a conversation. It slides out from the end edge
 * when asked, the main pane giving up the room, and the gap between them
 * drags to resize it; each browser remembers the width chosen. Dragging
 * it shut closes it. On screens too narrow for two panes, it opens in a
 * drawer over the page instead. The group reaches into the page's
 * margin so the panels' shadows are not cut off at its edges.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";
import { type Layout, Panel as Pane, useGroupRef } from "react-resizable-panels";

import { Drawer } from "@/components/base/drawer/drawer";
import { cx } from "@/utils/cx";

import { PaneGroup, PaneHandle } from "./resizable";
import { useEasing } from "./use-easing";
import { useWide } from "./use-wide";

const visible = { overflow: "visible" } as const;

/*
 * openingSize is how much of the group, in percent, the side takes when
 * it opens: the width last chosen, or width in pixels the first time,
 * always leaving the main pane some room.
 */
function openingSize(name: string, room: number, chosen: number | undefined, width: number) {
  const total = document.getElementById(name)?.clientWidth ?? 0;

  return Math.min(chosen ?? (width / Math.max(total, 1)) * 100, room - 30);
}

/*
 * PushPane lays main and side out side by side, remembering the width
 * under name. open says whether side is out; onOpenChange hears when the
 * teammate drags it shut or closes the drawer. width is the side's
 * opening width in pixels.
 */
export function PushPane({
  name,
  label,
  open,
  onOpenChange,
  main,
  side,
  width = 400,
}: {
  name: string;
  label: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  main: ReactNode;
  side: ReactNode;
  width?: number;
}) {
  const sideBySide = useWide(1024);
  const group = useGroupRef();
  const last = useRef<number | null>(null);
  const wasOut = useRef(false);
  const asked = useRef<boolean | null>(null);
  const [shown, setShown] = useState(open);
  const [easing, ease] = useEasing();
  const ids = { main: `${name}:main`, side: `${name}:side` };

  useEffect(() => {
    const handle = group.current;
    if (!sideBySide || !handle) {
      return;
    }
    const layout: Layout = { ...handle.getLayout() };
    const out = (layout[ids.side] ?? 0) > 0;
    if (out !== shown) {
      setShown(out);
    }
    if (open === out) {
      asked.current = null;
      return;
    }
    if (asked.current === open) {
      return;
    }
    asked.current = open;
    const room = (layout[ids.main] ?? 0) + (layout[ids.side] ?? 0);
    const opening = open ? openingSize(name, room, last.current ?? undefined, width) : 0;
    layout[ids.main] = room - opening;
    layout[ids.side] = opening;
    ease(() => handle.setLayout(layout));
    window.setTimeout(() => {
      asked.current = null;
    }, 400);
    setShown(open);
  });

  if (!sideBySide) {
    return (
      <div className="flex h-full min-w-0 flex-1">
        {main}
        <Drawer isOpen={open} onOpenChange={onOpenChange} placement="end" size="md" aria-label={label} className="p-0">
          {side}
        </Drawer>
      </div>
    );
  }

  return (
    <PaneGroup id={name} panelIds={[ids.main, ids.side]} easing={easing} groupRef={group} className="-m-3 p-3">
      <Pane id={ids.main} minSize={420} style={visible} className="flex h-full">
        {main}
      </Pane>
      <PaneHandle hidden={!shown} />
      <Pane
        id={ids.side}
        defaultSize={open ? width : 0}
        minSize={320}
        maxSize={720}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          const out = size.inPixels > 0;
          if (size.inPixels >= width / 2) {
            last.current = size.asPercentage;
          }
          setShown(out);
          if (asked.current === null && wasOut.current && !out && open) {
            onOpenChange(false);
          }
          wasOut.current = out;
        }}
        style={visible}
        className={cx("flex h-full", !shown && "invisible")}
      >
        {side}
      </Pane>
    </PaneGroup>
  );
}
