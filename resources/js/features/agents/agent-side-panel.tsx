/*
 * Beside Agent Studio, the way it sits beside a procedure: Preview, to
 * chat with the agent as a customer would, and Simulations, the chats
 * it must keep getting right. Both answer as the draft or the live
 * version; an agent never published has only its draft.
 */
import { RiCloseLine } from "@remixicon/react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { Panel } from "@/features/shell/page";
import type { AnswerAs } from "@/lib/api/agents";

import type { ChipSources } from "../automation/procedure-writer/procedure-suggestions";
import { AgentPreviewChat } from "./agent-preview";
import { AgentSimulations } from "./agent-simulations";
import type { AgentDraft } from "./use-agent-draft";

export type SideTab = "preview" | "simulations";

const answerAs = [
  ["draft", "Draft"],
  ["live", "Live"],
] as const;

const sideTabs: { id: SideTab; label: string }[] = [
  { id: "preview", label: "Preview" },
  { id: "simulations", label: "Simulations" },
];

export function AgentSidePanel({
  workspace,
  draft,
  sources,
  sideTab,
  onSideTab,
  as,
  onAs,
  onClose,
  className,
}: {
  workspace: string;
  draft: AgentDraft;
  sources: ChipSources;
  sideTab: SideTab;
  onSideTab: (tab: SideTab) => void;
  as: AnswerAs;
  onAs: (as: AnswerAs) => void;
  onClose?: () => void;
  className?: string;
}) {
  const published = draft.server.live_version > 0;
  const shown = published ? as : "draft";

  return (
    <Panel label="Testing" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <Tabs selectedKey={sideTab} onSelectionChange={(key) => onSideTab(String(key) as SideTab)}>
          <TabList aria-label="Testing" items={sideTabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
        <span className="flex items-center gap-1.5">
          {published ? (
            <SegmentedControl
              aria-label="Answer as"
              selectedKeys={[shown]}
              onSelectionChange={(keys) => onAs((String([...keys][0] ?? shown) as AnswerAs) || "draft")}
            >
              {answerAs.map(([key, label]) => (
                <SegmentedControlItem key={key} id={key}>
                  {label}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
          ) : null}
          {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close testing" onClick={onClose} /> : null}
        </span>
      </header>
      {sideTab === "preview" ? (
        <AgentPreviewChat
          key={shown}
          workspace={workspace}
          agent={draft.id}
          name={draft.draft.name}
          as={shown}
          sources={sources}
          flush={draft.flush}
          onSaved={() => onSideTab("simulations")}
        />
      ) : (
        <AgentSimulations
          workspace={workspace}
          agent={draft.id}
          name={draft.draft.name}
          as={shown}
          liveVersion={draft.server.live_version}
          sources={sources}
          flush={draft.flush}
        />
      )}
    </Panel>
  );
}
