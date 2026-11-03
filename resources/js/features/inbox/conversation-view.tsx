/*
 * One open conversation across three floating panels, resizable on the
 * gaps between them: the thread in the middle, the details beside it with
 * the moves a teammate makes on the case, and Operator on the right for
 * talking the case over with the team's own agent. Details and Operator
 * fold away from the header or by dragging them past their smallest size;
 * each browser remembers its layout. Details start open. On screens too
 * narrow for the side panels, Operator slides over the thread instead.
 */
import { RiArrowLeftSLine, RiLayoutRightLine, RiSparkling2Line } from "@remixicon/react";
import { useSuspenseQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Panel } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { meQuery } from "@/lib/api/auth";
import { cx } from "@/utils/cx";

import { useCase } from "./case";
import type { Conversation } from "./conversations";
import { DetailsPanel } from "./details-panel";
import { OperatorPanel } from "./operator-panel";
import { Thread } from "./thread";
import { useSidePanes } from "./use-side-panes";
import { useWide } from "./use-wide";

const visible = { overflow: "visible" } as const;

/*
 * The panes' names, under which each browser remembers their sizes.
 */
const pane = { group: "dimmi:conversation", thread: "thread", details: "details", operator: "operator" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

export function ConversationView({ view, conversation }: { view: string; conversation: Conversation }) {
  const { data: me } = useSuspenseQuery(meQuery);
  const actor = me?.user.name ?? "You";
  const caze = useCase(conversation, actor);
  const sideBySide = useWide(1024);
  const operatorDefault = useWide(1280);
  const sides = useSidePanes(
    pane.thread,
    { [pane.details]: 360, [pane.operator]: 380 },
    () => document.getElementById(pane.group)?.clientWidth ?? 0,
  );
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [operatorOpen, setOperatorOpen] = useState(operatorDefault);
  const [drawer, setDrawer] = useState(false);
  const [easing, ease] = useEasing();

  const toggle = (id: string) => ease(() => sides.toggle(id));
  const toggleOperator = () => (sideBySide ? toggle(pane.operator) : setDrawer(!drawer));
  const operatorShown = sideBySide ? operatorOpen : drawer;

  const thread = (
    <Panel label="Conversation" className="h-full w-full">
      <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 items-center gap-2">
          <Link
            to="/d/inbox/$view"
            params={{ view }}
            aria-label="Back to the list"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring 2xl:hidden"
          >
            <RiArrowLeftSLine aria-hidden className="size-5" />
          </Link>
          <h1 className="min-w-0 truncate ps-1.5 text-body-medium text-text-primary">{conversation.subject}</h1>
          {conversation.example ? (
            <Chip variant="caption" color="neutral">
              Example
            </Chip>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {sideBySide ? (
            <IconButton
              size="small"
              icon={RiLayoutRightLine}
              className={cx(detailsOpen && pressed)}
              aria-label={detailsOpen ? "Hide details" : "Show details"}
              aria-pressed={detailsOpen}
              onClick={() => toggle(pane.details)}
            />
          ) : null}
          <IconButton
            size="small"
            icon={RiSparkling2Line}
            className={cx(operatorShown && pressed)}
            aria-label="Operator"
            aria-pressed={operatorShown}
            onClick={toggleOperator}
          />
        </div>
      </header>
      <Thread conversation={conversation} events={caze.events} onAskOperator={toggleOperator} />
    </Panel>
  );

  if (!sideBySide) {
    return (
      <div className="flex h-full min-w-0 flex-1">
        {thread}
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Operator"
          className="p-0"
        >
          <OperatorPanel conversation={conversation} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      </div>
    );
  }

  return (
    <PaneGroup
      id={pane.group}
      panelIds={[pane.thread, pane.details, pane.operator]}
      easing={easing}
      groupRef={sides.group}
    >
      <Pane id={pane.thread} minSize={360} style={visible} className="flex h-full">
        {thread}
      </Pane>
      <PaneHandle hidden={!detailsOpen} />
      <Pane
        id={pane.details}
        defaultSize={360}
        minSize={300}
        maxSize={560}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          sides.remember(pane.details)(size);
          setDetailsOpen(size.inPixels > 0);
        }}
        style={visible}
        className={cx("flex h-full", !detailsOpen && "invisible")}
      >
        <DetailsPanel conversation={conversation} caze={caze} me={actor} onClose={() => toggle(pane.details)} />
      </Pane>
      <PaneHandle hidden={!operatorOpen} />
      <Pane
        id={pane.operator}
        defaultSize={operatorDefault ? 380 : 0}
        minSize={300}
        maxSize={600}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          sides.remember(pane.operator)(size);
          setOperatorOpen(size.inPixels > 0);
        }}
        style={visible}
        className={cx("flex h-full", !operatorOpen && "invisible")}
      >
        <OperatorPanel conversation={conversation} onClose={() => toggle(pane.operator)} className="h-full w-full" />
      </Pane>
    </PaneGroup>
  );
}
