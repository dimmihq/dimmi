/*
 * Version history and reader feedback open beside the article the way
 * Operator opens beside a conversation: on a wide screen a pane eases in
 * from the end and can be dragged wider, taking its room from the writer;
 * on a narrow one it slides over the page as a drawer. One pane holds
 * either; picking the other while it is open swaps what it shows.
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

import { ReaderFeedback } from "./reader-feedback";
import { VersionHistory } from "./version-history";

export type SideView = "history" | "feedback";

const pane = { group: "dimmi:article", writer: "writer", history: "history" } as const;
const visible = { overflow: "visible" } as const;

/*
 * useHistoryPane says what the side pane shows, opens, swaps or closes
 * it, and lays the writer out beside it.
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
  const [view, setView] = useState<SideView>("history");
  const shown = wide ? open : drawer;
  const close = () => (wide ? ease(() => sides.toggle(pane.history)) : setDrawer(false));
  const toggle = (next: SideView = view) => {
    setView(next);
    if (shown && next !== view) {
      return;
    }
    if (wide) {
      ease(() => sides.toggle(pane.history));
      return;
    }
    setDrawer(!drawer);
  };
  const content = (className: string) =>
    view === "history" ? (
      <VersionHistory
        workspace={workspace}
        article={article}
        onRestored={onRestored}
        onClose={close}
        className={className}
      />
    ) : (
      <ReaderFeedback workspace={workspace} article={article} onClose={close} className={className} />
    );

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
            aria-label={view === "history" ? "Version history" : "Reader feedback"}
            className="p-0"
          >
            {content("h-full")}
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
          {open ? content("h-full w-full") : null}
        </Pane>
      </PaneGroup>
    );
  };

  return { shown, view, toggle, layout };
}
