/*
 * Agent Studio: one agent, edited the way the procedure editor edits a
 * procedure. On the left, Build (the whole agent on one canvas),
 * History and Analytics. Beside it, Preview and Simulations, answering
 * as the draft or the live version; then Operator. On screens too
 * narrow for both, the side opens in a drawer from the header. Mockup:
 * sample data, nothing saved.
 */
import {
  RiAddLine,
  RiArrowDownSLine,
  RiArrowLeftSLine,
  RiCheckLine,
  RiDeleteBinLine,
  RiMore2Line,
  RiPauseCircleLine,
  RiSparkling2Line,
  RiTestTubeLine,
} from "@remixicon/react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { toast } from "@/components/base/toast/toast";
import { useSidePanes } from "@/features/inbox/use-side-panes";
import { Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { useWide } from "@/features/shell/use-wide";
import { cx } from "@/utils/cx";

import { StatusChip } from "../automation/parts";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { useChipSources } from "../automation/procedure-writer/use-chip-sources";
import { AgentBuild } from "./agent-build";
import { AgentAnalytics, AgentHistory } from "./agent-history";
import { SimulationsChip } from "./agent-list";
import { AgentMark } from "./agent-mark";
import { AgentOperatorPanel } from "./agent-operator-panel";
import { AgentSidePanel, type SideTab } from "./agent-side-panel";
import { agents, type MockAgent, sampleSources } from "./mock";

const visible = { overflow: "visible" } as const;
const pane = { group: "dimmi:agent", editor: "agent", details: "details", operator: "operator" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

type Tabbed = "build" | "history" | "analytics";

const tabs: { id: Tabbed; label: string }[] = [
  { id: "build", label: "Build" },
  { id: "history", label: "History" },
  { id: "analytics", label: "Analytics" },
];

/*
 * withSamples fills each kind of chip the workspace has none of with
 * sample names, so the mockup's @ menu always has something to show.
 */
function withSamples(real: ChipSources): ChipSources {
  return {
    ...real,
    procedures: real.procedures.length > 0 ? real.procedures : sampleSources.procedures,
    articles: real.articles.length > 0 ? real.articles : sampleSources.articles,
    teams: real.teams.length > 0 ? real.teams : sampleSources.teams,
  };
}

function AgentSwitcher({ agent }: { agent: MockAgent }) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label="Switch agent"
        className="flex min-w-16 items-center gap-2 rounded-lg px-1.5 py-1 outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <AgentMark agent={agent} size="sm" />
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{agent.name}</h1>
        <RiArrowDownSLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Agents" placement="bottom start" className="w-60">
        {agents.map((a) => (
          <DropdownItem
            key={a.id}
            className="gap-2"
            onSelect={() => {
              setOpen(false);
              void navigate({ to: "/d/agents/$agent", params: { agent: a.id } });
            }}
          >
            <AgentMark agent={a} size="sm" />
            <span className="min-w-0 flex-1 truncate">{a.name}</span>
            {a.id === agent.id ? <RiCheckLine aria-hidden className="size-4 shrink-0" /> : null}
          </DropdownItem>
        ))}
        <DropdownDivider />
        <DropdownItem
          className="gap-2"
          onSelect={() => {
            setOpen(false);
            void navigate({ to: "/d/agents" });
          }}
        >
          <RiAddLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          New agent
        </DropdownItem>
      </DropdownPopover>
    </Dropdown>
  );
}

function AgentMenu({ agent }: { agent: MockAgent }) {
  const [open, setOpen] = useState(false);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label="Agent actions"
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
      >
        <RiMore2Line aria-hidden className="size-4" />
      </DropdownTrigger>
      <DropdownPopover aria-label="Agent actions" placement="bottom end" className="w-52">
        {agent.status === "live" ? (
          <DropdownItem className="gap-2" onSelect={() => setOpen(false)}>
            <RiPauseCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            Turn off
          </DropdownItem>
        ) : null}
        {agent.isDefault ? null : (
          <>
            <DropdownDivider />
            <DropdownItem className="gap-2 text-text-error-primary" onSelect={() => setOpen(false)}>
              <RiDeleteBinLine aria-hidden className="size-4 shrink-0" />
              Delete agent
            </DropdownItem>
          </>
        )}
      </DropdownPopover>
    </Dropdown>
  );
}

function saveLine(agent: MockAgent): string {
  if (agent.status === "draft") {
    return "Saved as draft";
  }

  return agent.unpublished ? "Unpublished changes" : "Published";
}

function Publish({ agent }: { agent: MockAgent }) {
  const [publishing, setPublishing] = useState(false);
  const live = agent.status === "live";
  const failing = agent.tests.total - agent.tests.passing;
  const reach = live
    ? "Customers get the new version from their next message."
    : "It starts answering customers at once.";

  if (agent.status !== "draft" && !agent.unpublished) {
    return null;
  }

  return (
    <>
      <Button size="small" onClick={() => setPublishing(true)}>
        {live ? "Publish changes" : "Publish"}
      </Button>
      <AlertDialog
        isOpen={publishing}
        onOpenChange={setPublishing}
        variant="default"
        icon={false}
        title={live ? `Publish changes to ${agent.name}?` : `Publish ${agent.name}?`}
        description={failing > 0 ? `${failing} simulation failed on the last run. ${reach}` : reach}
        confirmLabel={live ? "Publish changes" : "Publish"}
        onConfirm={() => {
          setPublishing(false);
          toast.success(`Version ${agent.version + 1} is live`);
        }}
      />
    </>
  );
}

function MainPanel({
  agent,
  sources,
  testing,
  operator,
}: {
  agent: MockAgent;
  sources: ChipSources;
  testing: { shown: boolean; toggle: () => void; openSimulations: () => void };
  operator: { shown: boolean; toggle: () => void };
}) {
  const [tab, setTab] = useState<Tabbed>("build");

  return (
    <Panel label="Agent" ground className="h-full w-full">
      <header className="@container flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 flex-1 items-center gap-1.5">
          <Link
            to="/d/agents"
            aria-label="Back to agents"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
          </Link>
          <AgentSwitcher agent={agent} />
          <span className="hidden shrink-0 items-center gap-1.5 @lg:flex">
            {agent.isDefault ? (
              <Chip variant="caption" color="blue">
                Default
              </Chip>
            ) : null}
            <StatusChip status={agent.status} />
          </span>
          <span className="hidden truncate text-body-2-regular text-text-tertiary @3xl:inline">{saveLine(agent)}</span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            aria-label="Open simulations"
            onClick={testing.openSimulations}
            className="hidden cursor-pointer rounded-md outline-none transition-opacity duration-150 hover:opacity-80 focus-visible:ring-2 focus-visible:ring-border-focus-ring @xl:flex"
          >
            <SimulationsChip tests={agent.tests} />
          </button>
          <Publish agent={agent} />
          <AgentMenu agent={agent} />
          <IconButton
            size="small"
            icon={RiTestTubeLine}
            className={cx(testing.shown && pressed)}
            aria-label={testing.shown ? "Hide testing" : "Show testing"}
            aria-pressed={testing.shown}
            onClick={testing.toggle}
          />
          <IconButton
            size="small"
            icon={RiSparkling2Line}
            className={cx(operator.shown && pressed)}
            aria-label="Operator"
            aria-pressed={operator.shown}
            onClick={operator.toggle}
          />
        </div>
      </header>
      <div className="shrink-0 px-4">
        <Tabs selectedKey={tab} onSelectionChange={(key) => setTab(String(key) as Tabbed)}>
          <TabList aria-label="Agent" items={tabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
      </div>
      <ScrollArea>
        {tab === "build" ? <AgentBuild agent={agent} sources={sources} /> : null}
        {tab === "history" ? <AgentHistory /> : null}
        {tab === "analytics" ? <AgentAnalytics agent={agent} /> : null}
      </ScrollArea>
    </Panel>
  );
}

export function AgentStudio({ agent, workspace }: { agent: MockAgent; workspace: string }) {
  const sources = withSamples(useChipSources(workspace, ""));
  const sideBySide = useWide(1024);
  const sides = useSidePanes(
    pane.editor,
    { [pane.details]: 380, [pane.operator]: 380 },
    () => document.getElementById(pane.group)?.clientWidth ?? 0,
  );
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [operatorOpen, setOperatorOpen] = useState(false);
  const [detailsDrawer, setDetailsDrawer] = useState(false);
  const [operatorDrawer, setOperatorDrawer] = useState(false);
  const [easing, ease] = useEasing();
  const [sideTab, setSideTab] = useState<SideTab>("preview");

  const toggle = (id: string) => ease(() => sides.toggle(id));
  const toggleDetails = () => (sideBySide ? toggle(pane.details) : setDetailsDrawer((open) => !open));
  const toggleOperator = () => (sideBySide ? toggle(pane.operator) : setOperatorDrawer((open) => !open));
  const detailsShown = sideBySide ? detailsOpen : detailsDrawer;
  const operatorShown = sideBySide ? operatorOpen : operatorDrawer;
  const openSimulations = () => {
    setSideTab("simulations");
    if (!detailsShown) {
      toggleDetails();
    }
  };

  const main = (
    <MainPanel
      agent={agent}
      sources={sources}
      testing={{ shown: detailsShown, toggle: toggleDetails, openSimulations }}
      operator={{ shown: operatorShown, toggle: toggleOperator }}
    />
  );

  if (!sideBySide) {
    return (
      <div className="flex h-full min-h-0 min-w-0 flex-1">
        {main}
        <Drawer
          isOpen={detailsDrawer}
          onOpenChange={setDetailsDrawer}
          placement="end"
          size="md"
          aria-label="Testing"
          className="p-0"
        >
          <AgentSidePanel
            workspace={workspace}
            sideTab={sideTab}
            onSideTab={setSideTab}
            onClose={() => setDetailsDrawer(false)}
            className="h-full w-full"
          />
        </Drawer>
        <Drawer
          isOpen={operatorDrawer}
          onOpenChange={setOperatorDrawer}
          placement="end"
          size="md"
          aria-label="Operator"
          className="p-0"
        >
          <AgentOperatorPanel
            workspace={workspace}
            agentName={agent.name}
            onClose={() => setOperatorDrawer(false)}
            className="h-full w-full"
          />
        </Drawer>
      </div>
    );
  }

  return (
    <PaneGroup
      id={pane.group}
      panelIds={[pane.editor, pane.details, pane.operator]}
      easing={easing}
      groupRef={sides.group}
    >
      <Pane id={pane.editor} minSize={380} style={visible} className="flex h-full">
        {main}
      </Pane>
      <PaneHandle hidden={!detailsOpen} />
      <Pane
        id={pane.details}
        defaultSize={380}
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
        <AgentSidePanel
          workspace={workspace}
          sideTab={sideTab}
          onSideTab={setSideTab}
          onClose={() => toggle(pane.details)}
          className="h-full w-full"
        />
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
        <AgentOperatorPanel
          workspace={workspace}
          agentName={agent.name}
          onClose={() => toggle(pane.operator)}
          className="h-full w-full"
        />
      </Pane>
    </PaneGroup>
  );
}
