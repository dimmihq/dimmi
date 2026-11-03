/*
 * The procedure editor. On the left, the procedure as Dimmi reads it:
 * when to use it, the steps in order (an instruction, a condition, or an
 * action Dimmi takes in your systems) and when to hand over. On the
 * right, whether it is live, the actions it may take and its saved test
 * conversations. Tests run against the workspace's AI provider and cost
 * money, so they only run when someone asks. On screens too narrow for
 * both, the settings open in a drawer from the header. Edits stay in this page
 * until procedures are saved on the server.
 */
import {
  RiArrowLeftSLine,
  RiArrowRightLine,
  RiCloseLine,
  RiFlashlightLine,
  RiGitBranchLine,
  RiHandHeartLine,
  RiSettings3Line,
} from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Textarea } from "@/components/base/textarea/textarea";
import { Action, Choice, Toggle } from "@/features/settings/controls";
import { Empty, Item, Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";

import { KindMark, StatusChip } from "./parts";
import type { Procedure, ProcedureStep } from "./procedures";

const stepMark = {
  instruction: { icon: RiArrowRightLine, tint: "neutral", name: "Instruction" },
  condition: { icon: RiGitBranchLine, tint: "purple", name: "Condition" },
  action: { icon: RiFlashlightLine, tint: "lime", name: "Action" },
} as const;

const allActions = ["Look up an account", "Send a sign-in code", "Look up invoices", "Refund a payment"];
const resultTint = { passed: "lime", failed: "rose", "not run": "neutral" } as const;
const resultName = { passed: "Passed", failed: "Failed", "not run": "Not run" } as const;

function StepLine({ step, index }: { step: ProcedureStep; index: number }) {
  const look = stepMark[step.kind];

  return (
    <div className="flex items-start gap-3 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <span className="flex w-5 shrink-0 justify-end pt-2 text-body-2-medium text-text-tertiary tabular-nums">
        {index + 1}
      </span>
      <span className="pt-1">
        <KindMark icon={look.icon} tint={look.tint} size="sm" />
      </span>
      <div className="min-w-0 flex-1">
        <Textarea aria-label={`${look.name} ${index + 1}`} defaultValue={step.text} rows={1} autoResize maxRows={6} />
      </div>
    </div>
  );
}

function useSteps(procedure: Procedure) {
  const [steps, setSteps] = useState(procedure.steps);
  const made = useRef(0);

  const add = (kind: ProcedureStep["kind"]) => {
    made.current += 1;
    setSteps((before) => [...before, { id: `new-${made.current}`, kind, text: "" }]);
  };

  return { steps, add };
}

function Document({ procedure }: { procedure: Procedure }) {
  const { steps, add } = useSteps(procedure);

  return (
    <div className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-6 pt-8 pb-16">
      <Section title="When Dimmi uses this" description="Dimmi follows this procedure when a request matches.">
        <div className="py-2.5 pe-2.5">
          <Textarea aria-label="When Dimmi uses this" defaultValue={procedure.when} rows={2} autoResize maxRows={6} />
        </div>
      </Section>
      <Section
        title="Steps"
        description="In order. Conditions decide what happens next; actions run in your systems."
        action={
          <span className="flex gap-2">
            <Action onPress={() => add("instruction")}>Instruction</Action>
            <Action onPress={() => add("condition")}>Condition</Action>
            <Action onPress={() => add("action")}>Action</Action>
          </span>
        }
      >
        {steps.map((step, index) => (
          <StepLine key={step.id} step={step} index={index} />
        ))}
      </Section>
      <Section
        title="Hand over to the team when"
        description="Dimmi also hands over when the conversation goes in circles."
      >
        {procedure.handover.map((rule) => (
          <Item key={rule} mark={<KindMark icon={RiHandHeartLine} tint="orange" size="sm" />} title={rule} />
        ))}
      </Section>
    </div>
  );
}

function Side({ procedure, onClose, className }: { procedure: Procedure; onClose?: () => void; className?: string }) {
  const [asked, setAsked] = useState(false);

  return (
    <Panel label="Procedure settings" ground className={className}>
      <PageHeader title="Settings" level={2}>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close settings" onClick={onClose} /> : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          <Section title="Where">
            <Row label="Channels">
              <Choice label="Channels" options={["Messenger and email", "Messenger", "All channels"]} />
            </Row>
          </Section>
          <Section title="Actions it may take" description="Dimmi asks before any action that changes data.">
            {allActions.map((action) => (
              <Row key={action} label={action}>
                <Toggle label={action} on={procedure.actions.includes(action)} />
              </Row>
            ))}
          </Section>
          <Section
            title="Tests"
            description="Simulated customers check the procedure still works. Each run uses your AI provider key."
            action={<Action onPress={() => setAsked(true)}>Run tests</Action>}
          >
            {asked ? <Empty>Tests run once Dimmi answers customers.</Empty> : null}
            {procedure.tests.length === 0 ? <Empty>No tests yet</Empty> : null}
            {procedure.tests.map((test) => (
              <Row key={test.id} label={test.name}>
                <Chip variant="caption" color={resultTint[test.result]}>
                  {resultName[test.result]}
                </Chip>
              </Row>
            ))}
          </Section>
        </div>
      </ScrollArea>
    </Panel>
  );
}

export function ProcedureEditor({ procedure }: { procedure: Procedure }) {
  const [status, setStatus] = useState(procedure.status);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Procedure" ground className="h-full flex-1">
        <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-0 items-center gap-2">
            <Link
              to="/d/procedures"
              aria-label="Back to procedures"
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftSLine aria-hidden className="size-5" />
            </Link>
            <h1 className="min-w-0 truncate text-body-medium text-text-primary">{procedure.name}</h1>
            <StatusChip status={status} />
            <span className="hidden truncate text-body-2-regular text-text-tertiary md:inline">{procedure.edited}</span>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {beside ? null : (
              <IconButton
                size="small"
                icon={RiSettings3Line}
                aria-label="Procedure settings"
                onClick={() => setDrawer(true)}
              />
            )}
            {status === "live" ? (
              <Button variant="secondary" size="small" onClick={() => setStatus("draft")}>
                Turn off
              </Button>
            ) : (
              <Button size="small" onClick={() => setStatus("live")}>
                Publish
              </Button>
            )}
          </div>
        </header>
        <ScrollArea ground>
          <Document procedure={procedure} />
        </ScrollArea>
      </Panel>
      {beside ? (
        <Side procedure={procedure} className="h-full w-[360px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Procedure settings"
          className="p-0"
        >
          <Side procedure={procedure} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
