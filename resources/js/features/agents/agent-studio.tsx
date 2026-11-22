/*
 * Agent Studio: one agent, edited the way the procedure editor edits a
 * procedure. On the left, Build (the whole agent on one canvas) and
 * History. Beside it, Preview and Simulations, answering as the draft
 * or the live version; then Operator. On screens too narrow for both,
 * the side opens in a drawer from the header. Every change is saved to
 * the draft as it is made; Publish puts it in front of customers.
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
import { useQuery } from "@tanstack/react-query";
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
import { useSidePanes } from "@/features/inbox/use-side-panes";
import { Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { useWide } from "@/features/shell/use-wide";
import { type AgentView, type AnswerAs, agentSimulationsQuery, agentsQuery, resultOf } from "@/lib/api/agents";
import { cx } from "@/utils/cx";

import { StatusChip } from "../automation/parts";
import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { useChipSources } from "../automation/procedure-writer/use-chip-sources";
import { AgentBuild } from "./agent-build";
import { AgentHistory } from "./agent-history";
import { SimulationsChip } from "./agent-list";
import { AgentMark } from "./agent-mark";
import { AgentOperatorPanel } from "./agent-operator-panel";
import { AgentSidePanel, type SideTab } from "./agent-side-panel";
import { type AgentDraft, useAgentDraft } from "./use-agent-draft";

const visible = { overflow: "visible" } as const;
const pane = { group: "dimmi:agent", editor: "agent", details: "details", operator: "operator" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

type Tabbed = "build" | "history";

const tabs: { id: Tabbed; label: string }[] = [
  { id: "build", label: "Build" },
  { id: "history", label: "History" },
];

function AgentSwitcher({ workspace, draft }: { workspace: string; draft: AgentDraft }) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const agents = useQuery(agentsQuery(workspace)).data ?? [];
  const self = { id: draft.id, name: draft.draft.name, is_default: draft.server.is_default };

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger
        aria-label="Switch agent"
        className="flex min-w-16 items-center gap-2 rounded-lg px-1.5 py-1 outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <AgentMark agent={self} size="sm" />
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{self.name}</h1>
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
            {a.id === draft.id ? <RiCheckLine aria-hidden className="size-4 shrink-0" /> : null}
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

function AgentMenu({ draft }: { draft: AgentDraft }) {
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const agent = draft.server;
  if (agent.status !== "live" && agent.is_default) {
    return null;
  }

  return (
    <>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="Agent actions"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary transition-colors duration-150 hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4" />
        </DropdownTrigger>
        <DropdownPopover aria-label="Agent actions" placement="bottom end" className="w-52">
          {agent.status === "live" ? (
            <DropdownItem
              className="gap-2"
              onSelect={() => {
                setOpen(false);
                void draft.pause();
              }}
            >
              <RiPauseCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
              Turn off
            </DropdownItem>
          ) : null}
          {agent.is_default ? null : (
            <>
              {agent.status === "live" ? <DropdownDivider /> : null}
              <DropdownItem
                className="gap-2 text-text-error-primary"
                onSelect={() => {
                  setOpen(false);
                  setDeleting(true);
                }}
              >
                <RiDeleteBinLine aria-hidden className="size-4 shrink-0" />
                Delete agent
              </DropdownItem>
            </>
          )}
        </DropdownPopover>
      </Dropdown>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${draft.draft.name}?`}
        description="Its conversations go to the default agent. This cannot be undone."
        confirmLabel="Delete agent"
        onConfirm={() => {
          setDeleting(false);

          return draft.remove();
        }}
      />
    </>
  );
}

function saveLine(draft: AgentDraft): string {
  if (draft.state === "saving") {
    return "Saving…";
  }
  if (draft.state === "conflict") {
    return "Someone else changed this agent. Reload to see it.";
  }
  if (draft.state === "failed") {
    return "Not saved";
  }
  if (draft.server.status === "draft") {
    return "Saved as draft";
  }

  return draft.server.has_unpublished_changes ? "Unpublished changes" : "Published";
}

function Publish({ draft, failing }: { draft: AgentDraft; failing: number }) {
  const [publishing, setPublishing] = useState(false);
  const agent = draft.server;
  const live = agent.status === "live";
  const reach = live
    ? "Customers get the new version from their next message."
    : "It starts answering customers at once.";

  if (live && !agent.has_unpublished_changes && draft.state === "saved") {
    return null;
  }
  const name = draft.draft.name;

  return (
    <>
      <Button size="small" loading={draft.busy} onClick={() => setPublishing(true)}>
        {live ? "Publish changes" : "Publish"}
      </Button>
      <AlertDialog
        isOpen={publishing}
        onOpenChange={setPublishing}
        variant="default"
        icon={false}
        title={live ? `Publish changes to ${name}?` : `Publish ${name}?`}
        description={failing > 0 ? `${failing} simulation failed on its last run on the draft. ${reach}` : reach}
        confirmLabel={live ? "Publish changes" : "Publish"}
        onConfirm={() => {
          setPublishing(false);

          return draft.publish();
        }}
      />
    </>
  );
}

function MainPanel({
  workspace,
  draft,
  view,
  sources,
  testing,
  operator,
}: {
  workspace: string;
  draft: AgentDraft;
  view: AgentView;
  sources: ChipSources;
  testing: { shown: boolean; toggle: () => void; openSimulations: () => void };
  operator: { shown: boolean; toggle: () => void };
}) {
  const [tab, setTab] = useState<Tabbed>("build");
  const sims = useQuery(agentSimulationsQuery(workspace, draft.id)).data ?? [];
  const results = sims.map((s) => resultOf(s, "draft"));
  const failing = results.filter((r) => r && !r.passed).length;

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
          <AgentSwitcher workspace={workspace} draft={draft} />
          <span className="hidden shrink-0 items-center gap-1.5 @lg:flex">
            {draft.server.is_default ? (
              <Chip variant="caption" color="blue">
                Default
              </Chip>
            ) : null}
            <StatusChip status={draft.server.status} />
          </span>
          <span className="hidden truncate text-body-2-regular text-text-tertiary @3xl:inline">{saveLine(draft)}</span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            aria-label="Open simulations"
            onClick={testing.openSimulations}
            className="hidden cursor-pointer rounded-md outline-none transition-opacity duration-150 hover:opacity-80 focus-visible:ring-2 focus-visible:ring-border-focus-ring @xl:flex"
          >
            <SimulationsChip results={results} />
          </button>
          <Publish draft={draft} failing={failing} />
          <AgentMenu draft={draft} />
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
        {tab === "build" ? <AgentBuild draft={draft} view={view} workspace={workspace} sources={sources} /> : null}
        {tab === "history" ? <AgentHistory workspace={workspace} draft={draft} /> : null}
      </ScrollArea>
    </Panel>
  );
}

export function AgentStudio({ view, workspace }: { view: AgentView; workspace: string }) {
  const navigate = useNavigate();
  const draft = useAgentDraft(workspace, view, () => void navigate({ to: "/d/agents" }));
  const sources = useChipSources(workspace, "");
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
  const [as, setAs] = useState<AnswerAs>("draft");

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
      workspace={workspace}
      draft={draft}
      view={view}
      sources={sources}
      testing={{ shown: detailsShown, toggle: toggleDetails, openSimulations }}
      operator={{ shown: operatorShown, toggle: toggleOperator }}
    />
  );
  const side = (onClose: () => void) => (
    <AgentSidePanel
      workspace={workspace}
      draft={draft}
      sources={sources}
      sideTab={sideTab}
      onSideTab={setSideTab}
      as={as}
      onAs={setAs}
      onClose={onClose}
      className="h-full w-full"
    />
  );
  const operator = (onClose: () => void) => (
    <AgentOperatorPanel
      workspace={workspace}
      agentName={draft.draft.name}
      onClose={onClose}
      className="h-full w-full"
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
