/*
 * Beside Agent Studio, the way it sits beside a procedure: Preview, to
 * chat with the agent as a customer would, and Simulations, the chats
 * it must keep getting right. Both answer as the draft or the live
 * version. Mockup: a fixed preview chat and sample simulations.
 */
import { RiChat3Line, RiCloseLine, RiRestartLine } from "@remixicon/react";
import { useState } from "react";
import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Tab, TabList, Tabs } from "@/components/base/tabs/tabs";
import { toast } from "@/components/base/toast/toast";
import { subjectOf } from "@/features/inbox/conversations";
import { MessageBody } from "@/features/inbox/message-body";
import { ActivityLog } from "@/features/operator/activity-log";
import { avatarColor } from "@/features/shell/avatar-color";
import { Panel, ScrollArea } from "@/features/shell/page";
import type { Conversation } from "@/lib/api/inbox";
import type { SimPersona } from "@/lib/api/procedures";

import { PreviewAddMenu } from "../automation/procedure-simulator/preview-add-menu";
import { Added, PreviewComposer } from "../automation/procedure-simulator/preview-composer";
import { fromCustomer } from "../automation/procedure-simulator/simulator";
import { AgentSimulations } from "./agent-simulations";

/* A made-up signed-in customer on the messenger, as procedures preview by default. */
const madeUp: SimPersona = { verified: true, channel: "widget" };

/*
 * AddedRow lists what was added to the preview, as procedures show it:
 * the customer the agent answers as, and the conversation being
 * replayed.
 */
function AddedRow({
  persona,
  replay,
  onNoCustomer,
  onNoReplay,
}: {
  persona: SimPersona;
  replay: Conversation | null;
  onNoCustomer: () => void;
  onNoReplay: () => void;
}) {
  const customer = persona.name || persona.email;
  if (!customer && !replay) {
    return null;
  }

  return (
    <div className="flex flex-wrap gap-1.5 px-0.5 pt-0.5">
      {customer ? (
        <Added
          mark={<Avatar size="md" color={avatarColor(customer)} initials={customer.charAt(0).toUpperCase()} />}
          title={customer}
          detail={persona.name && persona.email ? `Testing as ${persona.email}` : "Testing as this customer"}
          onRemove={onNoCustomer}
        />
      ) : null}
      {replay ? (
        <Added
          mark={
            <span className="flex size-8 items-center justify-center rounded-full bg-background-tertiary-default">
              <RiChat3Line aria-hidden className="size-4 text-foreground-icon-secondary" />
            </span>
          }
          title={subjectOf(replay)}
          detail={`Replaying conversation #${replay.number}`}
          onRemove={onNoReplay}
        />
      ) : null}
    </div>
  );
}

export type SideTab = "preview" | "simulations";

const sideTabs: { id: SideTab; label: string }[] = [
  { id: "preview", label: "Preview" },
  { id: "simulations", label: "Simulations" },
];

const answerAs = [
  ["draft", "Draft"],
  ["live", "Live"],
] as const;

function Preview({ workspace, onSaved }: { workspace: string; onSaved: () => void }) {
  const [message, setMessage] = useState("");
  const [persona, setPersona] = useState<SimPersona>(madeUp);
  const [replay, setReplay] = useState<Conversation | null>(null);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex h-10 shrink-0 items-center justify-end gap-1 px-3">
        <Button
          variant="ghost"
          size="xs"
          onClick={() => {
            toast.success("Saved as a simulation.");
            onSaved();
          }}
        >
          Save as simulation
        </Button>
        <Button variant="ghost" size="xs" iconOnly leadingIcon={RiRestartLine} aria-label="Start over" />
      </div>
      <ScrollArea className="flex flex-col">
        <ol aria-label="Preview conversation" className="flex flex-1 flex-col justify-end gap-5 px-4 pt-4 pb-6">
          <li className="flex flex-col gap-5">
            <p className={fromCustomer}>How much is it for a team of 25?</p>
            <div className="flex flex-col gap-3">
              <ActivityLog
                working={false}
                steps={[
                  { tool: "guidance", summary: "Followed Accuracy and Qualifying", ok: true },
                  { tool: "search", summary: "Read Pricing & plans", ok: true },
                  { tool: "step", summary: "Qualify & book demo, step 1", ok: true },
                ]}
              />
              <MessageBody
                text="Growth is $29 a seat each month, so $725 for 25 seats. What will your team use dimmi for?"
                className="text-body-regular text-text-primary"
              />
            </div>
          </li>
          <li className="flex flex-col gap-5">
            <p className={fromCustomer}>We're already customers actually, the invoice export is broken</p>
            <span className="flex items-center gap-2 self-stretch text-center text-body-2-regular text-text-tertiary">
              <span className="h-px min-w-4 flex-1 bg-separator-border" />
              Sales transferred to dimmi: invoice export bug
              <span className="h-px min-w-4 flex-1 bg-separator-border" />
            </span>
            <MessageBody
              text="Sorry about that. Were you exporting to CSV or PDF?"
              className="text-body-regular text-text-primary"
            />
          </li>
        </ol>
      </ScrollArea>
      <div className="shrink-0 px-4 pb-4">
        <PreviewComposer
          value={message}
          onValueChange={setMessage}
          onSubmit={() => setMessage("")}
          onStop={() => undefined}
          busy={false}
          placeholder={replay ? "Press send to replay it" : "Write as the customer"}
          canSubmit={replay !== null}
          added={
            <AddedRow
              persona={persona}
              replay={replay}
              onNoCustomer={() => setPersona(madeUp)}
              onNoReplay={() => setReplay(null)}
            />
          }
          menu={<PreviewAddMenu workspace={workspace} replayable onConversation={setReplay} onCustomer={setPersona} />}
        />
      </div>
    </div>
  );
}

export function AgentSidePanel({
  workspace,
  sideTab,
  onSideTab,
  onClose,
  className,
}: {
  workspace: string;
  sideTab: SideTab;
  onSideTab: (tab: SideTab) => void;
  onClose?: () => void;
  className?: string;
}) {
  const [version, setVersion] = useState("draft");

  return (
    <Panel label="Testing" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <Tabs selectedKey={sideTab} onSelectionChange={(key) => onSideTab(String(key) as SideTab)}>
          <TabList aria-label="Testing" items={sideTabs}>
            {(item) => <Tab id={item.id}>{item.label}</Tab>}
          </TabList>
        </Tabs>
        <span className="flex items-center gap-1.5">
          <SegmentedControl
            aria-label="Answer as"
            selectedKeys={[version]}
            onSelectionChange={(keys) => setVersion(String([...keys][0] ?? version))}
          >
            {answerAs.map(([key, label]) => (
              <SegmentedControlItem key={key} id={key}>
                {label}
              </SegmentedControlItem>
            ))}
          </SegmentedControl>
          {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close testing" onClick={onClose} /> : null}
        </span>
      </header>
      {sideTab === "preview" ? (
        <Preview workspace={workspace} onSaved={() => onSideTab("simulations")} />
      ) : (
        <AgentSimulations version={version} />
      )}
    </Panel>
  );
}
