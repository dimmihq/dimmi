/*
 * Version history opens beside the article the way Operator opens beside
 * a conversation: on a wide screen a pane eases in from the end and can
 * be dragged wider, taking its room from the writer; on a narrow one it
 * slides over the page as a drawer.
 */
import type { ReactNode } from "react";
import { useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Drawer } from "@/components/base/drawer/drawer";
import { useSidePanes } from "@/features/inbox/use-side-panes";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { useWide } from "@/features/shell/use-wide";
import type { Article } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { VersionHistory } from "./version-history";

const pane = { group: "dimmi:article", writer: "writer", history: "history" } as const;
const visible = { overflow: "visible" } as const;

/*
 * useHistoryPane says whether the history shows, toggles it, and lays
 * the writer out beside it.
 */
export function useHistoryPane(workspace: string, article: string, onRestored: (article: Article) => void) {
  const wide = useWide(1024);
  const sides = useSidePanes(
    pane.writer,
    { [pane.history]: 360 },
    () => document.getElementById(pane.group)?.clientWidth ?? 0,
  );
  const [open, setOpen] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [easing, ease] = useEasing();
  const toggle = () => (wide ? ease(() => sides.toggle(pane.history)) : setDrawer(!drawer));

  const layout = (writer: ReactNode) => {
    if (!wide) {
      return (
        <div className="flex h-full min-w-0 flex-1">
          {writer}
          <Drawer
            isOpen={drawer}
            onOpenChange={setDrawer}
            placement="end"
            size="md"
            aria-label="Version history"
            className="p-0"
          >
            <VersionHistory
              workspace={workspace}
              article={article}
              onRestored={onRestored}
              onClose={() => setDrawer(false)}
              className="h-full"
            />
          </Drawer>
        </div>
      );
    }

    return (
      <PaneGroup id={pane.group} panelIds={[pane.writer, pane.history]} easing={easing} groupRef={sides.group}>
        <Pane id={pane.writer} minSize={420} style={visible} className="flex h-full">
          {writer}
        </Pane>
        <PaneHandle hidden={!open} />
        <Pane
          id={pane.history}
          defaultSize={0}
          minSize={280}
          maxSize={520}
          collapsible
          groupResizeBehavior="preserve-pixel-size"
          onResize={(size) => {
            sides.remember(pane.history)(size);
            setOpen(size.inPixels > 0);
          }}
          style={visible}
          className={cx("flex h-full", !open && "invisible")}
        >
          {open ? (
            <VersionHistory
              workspace={workspace}
              article={article}
              onRestored={onRestored}
              onClose={toggle}
              className="h-full w-full"
            />
          ) : null}
        </Pane>
      </PaneGroup>
    );
  };

  return { shown: wide ? open : drawer, toggle, layout };
}
