/*
 * The inspector beside the canvas: the settings of whatever is selected,
 * the trigger or a step, laid out like the settings pages. The trigger
 * shows what starts the workflow, which never changes, and holds the
 * workflow's own settings: its name, the channels it runs on and how
 * often it may run on one conversation. Whatever checking the workflow
 * found about the selection is listed on top, so a problem is fixed
 * where it is. Someone who may only read workflows sees every setting
 * and changes none. On screens too narrow to sit beside the canvas it
 * opens in a drawer, with a close button.
 */
import { RiCloseLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import type { Issue, Spec } from "@/lib/api/workflows";
import { choicesOf } from "./choices";
import { KindMark } from "./parts";
import { Pick, Picks } from "./pick";
import { type Lists, StepFields } from "./step-fields";
import { stepLook } from "./step-kinds";
import { type TriggerKind, tintOf, triggers } from "./triggers";
import type { Step, StepKind } from "./workflows";

const channelChoices = { all: "All channels", widget: "Messenger", email: "Email" };
const statusChoices = { open: "Open", snoozed: "Snoozed", closed: "Closed" };

/*
 * Head changes the workflow's own settings: its name and the parts of
 * its definition outside the steps.
 */
export interface Head {
  name: string;
  onRename: (name: string) => void;
  onSpec: (edit: (spec: Spec) => Spec) => void;
  onArchive: () => void;
}

function Issues({ issues }: { issues: Issue[] }) {
  if (issues.length === 0) {
    return null;
  }

  return (
    <Section title="To fix before publishing">
      {issues.map((issue) => (
        <Row key={`${issue.code}-${issue.path_id ?? ""}-${issue.field ?? ""}-${issue.message}`} label={issue.message} />
      ))}
    </Section>
  );
}

function TriggerSettings({ spec, head }: { spec: Spec; head: Head }) {
  const trigger = spec.trigger as TriggerKind;
  const look = triggers[trigger];
  const to = Array.isArray(spec.trigger_settings?.to) ? (spec.trigger_settings.to as string[]) : [];
  const once = spec.reentry.mode === "once_per_conversation";

  return (
    <>
      <Section title="Name">
        <Row label="Name">
          <Field label="Workflow name" value={head.name} onChange={head.onRename} />
        </Row>
      </Section>
      <Section
        title="When it starts"
        description="A workflow keeps the trigger it was made with. To start from another, make a new workflow."
      >
        <Row
          label={
            <span className="flex items-center gap-2">
              {look ? <KindMark icon={look.icon} tint={tintOf(trigger)} size="sm" /> : null}
              {look?.name ?? spec.trigger}
            </span>
          }
          description={look?.detail}
        />
        {trigger === "conversation.status_changed" ? (
          <>
            <Row label="Only when it becomes" description="Leave all unticked to run on every change." />
            <Picks
              choices={choicesOf(statusChoices)}
              value={to}
              empty=""
              onChange={(next) =>
                head.onSpec((current) => ({
                  ...current,
                  trigger_settings: next.length > 0 ? { to: next } : undefined,
                }))
              }
            />
          </>
        ) : null}
      </Section>
      <Section title="Who it runs for">
        <Row label="Channels">
          <Pick
            label="Channels"
            choices={choicesOf(channelChoices)}
            value={spec.channels?.[0] ?? "all"}
            onChange={(channel) =>
              head.onSpec((current) => ({ ...current, channels: channel === "all" ? undefined : [channel] }))
            }
          />
        </Row>
        <Row
          label="Once per conversation"
          description={
            look?.onceOnly
              ? "This trigger never runs twice in one conversation."
              : "Never run twice in the same conversation."
          }
        >
          <Toggle
            label="Once per conversation"
            on={once || look?.onceOnly === true}
            disabled={look?.onceOnly === true}
            onChange={(on) =>
              head.onSpec((current) => ({
                ...current,
                reentry: { mode: on ? "once_per_conversation" : "every_time" },
              }))
            }
          />
        </Row>
      </Section>
      <Section title="Archive">
        <Row label="Archive this workflow" description="It stops running. You can bring it back later.">
          <Danger onPress={head.onArchive}>Archive</Danger>
        </Row>
      </Section>
    </>
  );
}

export function Inspector({
  spec,
  step,
  issues,
  lists,
  head,
  readOnly,
  onRemove,
  onEdit,
  onClose,
  className,
}: {
  spec: Spec;
  step: Step | undefined;
  issues: Issue[];
  lists: Lists;
  head: Head;
  readOnly: boolean;
  onRemove: (id: string) => void;
  onEdit: (id: string, edit: (step: Step) => Step) => void;
  onClose?: () => void;
  className?: string;
}): ReactNode {
  const title = step ? (stepLook[step.kind as StepKind]?.name ?? step.kind) : "Trigger";

  return (
    <Panel label="Step settings" ground className={className}>
      <PageHeader title={title} level={2}>
        {onClose ? (
          <IconButton size="small" icon={RiCloseLine} aria-label="Close step settings" onClick={onClose} />
        ) : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <fieldset disabled={readOnly} className="flex min-w-0 flex-col gap-6 pt-2">
          <Issues issues={issues} />
          {step ? (
            <StepFields
              key={step.id}
              step={step}
              trigger={spec.trigger as TriggerKind}
              lists={lists}
              onEdit={(edit) => onEdit(step.id, edit)}
            />
          ) : (
            <TriggerSettings spec={spec} head={head} />
          )}
          {step && !readOnly ? (
            <Section title="Remove">
              <Row label="Remove this step" description="Steps on its paths go with it.">
                <Danger onPress={() => onRemove(step.id)}>Remove</Danger>
              </Row>
            </Section>
          ) : null}
        </fieldset>
      </ScrollArea>
    </Panel>
  );
}
