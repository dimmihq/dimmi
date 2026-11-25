/*
 * The studio frame workflows and series share with Agent Studio. The
 * main panel holds the header (back, name, status, what is saved, the
 * buttons that publish) and the Build, Analytics and History tabs.
 * Beside it, in order: the step settings of whatever is picked on the
 * canvas, the testing panel (Preview and Simulations), and Operator.
 * The canvas comes first, so testing and Operator open from the header
 * when wanted. Each side pane slides out and drags to any width; on
 * screens too narrow for them, each opens in a drawer instead.
 */
import { RiCloseLine, RiSparkling2Line, RiTestTubeLine } from "@remixicon/react";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { useHeading } from "@/features/agents/use-heading";
import { Swap } from "@/features/inbox/motion";
import { useSidePanes } from "@/features/inbox/use-side-panes";
import { Panel } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { useWide } from "@/features/shell/use-wide";
import { cx } from "@/utils/cx";

const visible = { overflow: "visible" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

export interface StudioTab<T extends string> {
  id: T;
  label: string;
}

export type StudioTabId = "build" | "analytics" | "history";

const studioTabs: StudioTab<StudioTabId>[] = [
  { id: "build", label: "Build" },
  { id: "analytics", label: "Analytics" },
  { id: "history", label: "History" },
];

/*
 * Inspected is the pane of step settings: whether it is asked open, how
 * to close it, and what it shows.
 */
export interface Inspected {
  label: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  node: ReactNode;
}

/*
 * useInspector keeps the step settings pane in step with open, the way
 * PushPane does on its own: it slides the pane out or shut when asked,
 * and tells onOpenChange when the teammate drags it shut.
 */
function useInspector(
  sides: ReturnType<typeof useSidePanes>,
  id: string,
  open: boolean,
  ease: (move: () => void) => void,
  active: boolean,
) {
  const [out, setOut] = useState(false);
  const asked = useRef<boolean | null>(null);

  useEffect(() => {
    const handle = sides.group.current;
    if (!active || !handle) {
      return;
    }
    const now = (handle.getLayout()[id] ?? 0) > 0;
    if (now !== out) {
      setOut(now);
    }
    if (open === now) {
      asked.current = null;
      return;
    }
    if (asked.current === open) {
      return;
    }
    asked.current = open;
    ease(() => sides.toggle(id));
    window.setTimeout(() => {
      asked.current = null;
    }, 400);
  });

  return { out, setOut, asked };
}

export function Studio({
  name,
  label,
  start,
  end,
  tab,
  onTab,
  body,
  inspector,
  side,
  operator,
  testing = 0,
}: {
  /* name keys the remembered layout, such as "dimmi:workflow". */
  name: string;
  label: string;
  start: ReactNode;
  end: ReactNode;
  tab: StudioTabId;
  onTab: (tab: StudioTabId) => void;
  /* body is the open tab's view; it fills the panel below the tabs. */
  body: ReactNode;
  inspector: Inspected;
  side: (onClose: () => void) => ReactNode;
  operator: (onClose: () => void) => ReactNode;
  /* Opens the testing panel each time it goes up, such as when Operator starts trying something. */
  testing?: number;
}) {
  const pane = { editor: `${name}:editor`, inspector: `${name}:inspector`, details: "details", operator: "operator" };
  const sideBySide = useWide(1024);
  const sides = useSidePanes(
    pane.editor,
    { [pane.inspector]: 380, [pane.details]: 380, [pane.operator]: 380 },
    () => document.getElementById(name)?.clientWidth ?? 0,
  );
  const [easing, ease] = useEasing();
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [detailsDrawer, setDetailsDrawer] = useState(false);
  const [operatorDrawer, setOperatorDrawer] = useState(false);
  const inspecting = useInspector(sides, pane.inspector, inspector.open, ease, sideBySide);
  const forward = useHeading(studioTabs, tab);

  const toggle = (id: string) => ease(() => sides.toggle(id));
  const toggleDetails = () => (sideBySide ? toggle(pane.details) : setDetailsDrawer((open) => !open));
  const toggleOperator = () => (sideBySide ? toggle(pane.operator) : setOperatorDrawer((open) => !open));
  const detailsShown = sideBySide ? detailsOpen : detailsDrawer;

  // biome-ignore lint/correctness/useExhaustiveDependencies: opens the panel only when asked again, not as it moves.
  useEffect(() => {
    if (testing === 0 || detailsShown) {
      return;
    }
    if (sideBySide) {
      toggle(pane.details);
    } else {
      setDetailsDrawer(true);
    }
  }, [testing]);
  const operatorShown = sideBySide ? operatorOpen : operatorDrawer;

  const main = (
    <Panel label={label} ground className="h-full w-full">
      <header className="@container flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 flex-1 items-center gap-1.5">{start}</div>
        <div className="flex shrink-0 items-center gap-2">
          {end}
          <IconButton
            size="small"
            icon={RiTestTubeLine}
            className={cx(detailsShown && pressed)}
            aria-label={detailsShown ? "Hide testing" : "Show testing"}
            aria-pressed={detailsShown}
            onClick={toggleDetails}
          />
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
      <div className="shrink-0 px-4">
        <Tabs selectedKey={tab} onSelectionChange={(key) => onTab(String(key) as StudioTabId)}>
          <TabList aria-label={label} items={studioTabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
      </div>
      <div className="relative flex min-h-0 flex-1 flex-col">
        <Swap view={tab} forward={forward} className="flex min-h-0 flex-1 flex-col">
          {body}
        </Swap>
      </div>
    </Panel>
  );

  if (!sideBySide) {
    return (
      <div className="flex h-full min-h-0 min-w-0 flex-1">
        {main}
        <Drawer
          isOpen={inspector.open}
          onOpenChange={inspector.onOpenChange}
          placement="end"
          size="md"
          aria-label={inspector.label}
          className="p-0"
        >
          {inspector.node}
        </Drawer>
        <Drawer
          isOpen={detailsDrawer}
          onOpenChange={setDetailsDrawer}
          placement="end"
          size="md"
          aria-label="Testing"
          className="p-0"
        >
          {side(() => setDetailsDrawer(false))}
        </Drawer>
        <Drawer
          isOpen={operatorDrawer}
          onOpenChange={setOperatorDrawer}
          placement="end"
          size="md"
          aria-label="Operator"
          className="p-0"
        >
          {operator(() => setOperatorDrawer(false))}
        </Drawer>
      </div>
    );
  }

  return (
    <PaneGroup
      id={name}
      panelIds={[pane.editor, pane.inspector, pane.details, pane.operator]}
      easing={easing}
      groupRef={sides.group}
    >
      <Pane id={pane.editor} minSize={380} style={visible} className="flex h-full">
        {main}
      </Pane>
      <PaneHandle hidden={!inspecting.out} />
      <Pane
        id={pane.inspector}
        defaultSize={0}
        minSize={300}
        maxSize={720}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          const out = size.inPixels > 0;
          if (inspecting.asked.current === null) {
            sides.remember(pane.inspector)(size);
            if (inspecting.out && !out && inspector.open) {
              inspector.onOpenChange(false);
            }
          }
          inspecting.setOut(out);
        }}
        style={visible}
        className={cx("flex h-full", !inspecting.out && "invisible")}
      >
        {inspector.node}
      </Pane>
      <PaneHandle hidden={!detailsOpen} />
      <Pane
        id={pane.details}
        defaultSize={0}
        minSize={300}
        maxSize={640}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          sides.remember(pane.details)(size);
          setDetailsOpen(size.inPixels > 0);
        }}
        style={visible}
        className={cx("flex h-full", !detailsOpen && "invisible")}
      >
        {side(() => toggle(pane.details))}
      </Pane>
      <PaneHandle hidden={!operatorOpen} />
      <Pane
        id={pane.operator}
        defaultSize={0}
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
        {operator(() => toggle(pane.operator))}
      </Pane>
    </PaneGroup>
  );
}

export type SideTabId = "preview" | "simulations";

const sideTabs: StudioTab<SideTabId>[] = [
  { id: "preview", label: "Preview" },
  { id: "simulations", label: "Simulations" },
];

/*
 * StudioSide is the testing panel beside the studio: Preview, which
 * reads the draft back in words, and Simulations.
 */
export function StudioSide({
  tab,
  onTab,
  preview,
  simulations,
  onClose,
  className,
}: {
  tab: SideTabId;
  onTab: (tab: SideTabId) => void;
  preview: ReactNode;
  simulations: ReactNode;
  onClose?: () => void;
  className?: string;
}) {
  const forward = useHeading(sideTabs, tab);

  return (
    <Panel label="Testing" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <Tabs selectedKey={tab} onSelectionChange={(key) => onTab(String(key) as SideTabId)}>
          <TabList aria-label="Testing" items={sideTabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close testing" onClick={onClose} /> : null}
      </header>
      <Swap view={tab} forward={forward} className="flex min-h-0 flex-1 flex-col">
        {tab === "preview" ? preview : null}
        {tab === "simulations" ? simulations : null}
      </Swap>
    </Panel>
  );
}
