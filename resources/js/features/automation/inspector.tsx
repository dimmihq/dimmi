/*
 * The inspector beside the canvas: the settings of whatever is selected,
 * the trigger or a step, laid out like the settings pages. Every kind of
 * step has its own few settings; the trigger shows what starts the
 * workflow, which never changes, and holds the workflow's own settings
 * (channels, audience, office hours, how often it runs). Dimmi's step
 * repeats the rules Dimmi always keeps, so nobody has to guess them. On
 * screens too narrow to sit beside the canvas it opens in a drawer, with
 * a close button. Every change shows on the step's card as it is made.
 */
import { RiCloseLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Textarea } from "@/components/base/textarea/textarea";
import { Action, Choice, Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { KindMark } from "./parts";
import { stepLook } from "./step-kinds";
import { fieldsOf, options, unsetSummary, withField, withPathLabel } from "./step-settings";
import { type TriggerKind, tintOf, triggers } from "./triggers";
import type { Step } from "./workflows";

type Edit = (edit: (step: Step) => Step) => void;

function Words({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="py-2.5 pe-2.5">
      <Textarea
        aria-label={label}
        defaultValue={value}
        placeholder={unsetSummary}
        onChange={onChange}
        rows={3}
        autoResize
        maxRows={8}
      />
    </div>
  );
}

function TriggerSettings({ trigger }: { trigger: TriggerKind }) {
  return (
    <>
      <Section
        title="When it starts"
        description="A workflow keeps the trigger it was made with. To start from another, make a new workflow."
      >
        <Row
          label={
            <span className="flex items-center gap-2">
              <KindMark icon={triggers[trigger].icon} tint={tintOf(trigger)} size="sm" />
              {triggers[trigger].name}
            </span>
          }
          description={triggers[trigger].detail}
        />
      </Section>
      <Section title="Who it runs for">
        <Row label="Channels">
          <Choice label="Channels" options={["Messenger, email", "Messenger", "Email", "All channels"]} />
        </Row>
        <Row label="Audience">
          <Choice label="Audience" options={["Everyone", "Signed-in customers", "Visitors"]} />
        </Row>
        <Row label="Only during office hours">
          <Toggle label="Only during office hours" />
        </Row>
        <Row label="Once per conversation" description="Never run twice in the same conversation.">
          <Toggle label="Once per conversation" on />
        </Row>
      </Section>
    </>
  );
}

/*
 * StepFields shows the settings of one step. Every change goes back to
 * the step through onEdit, which also rewrites the sentence on its card.
 */
function StepFields({ step, onEdit }: { step: Step; onEdit: Edit }): ReactNode {
  const fields = fieldsOf(step);
  const set = (name: string) => (value: string) => onEdit((current) => withField(current, name, value));
  const flip = (name: string) => (on: boolean) => set(name)(on ? "on" : "off");

  switch (step.kind) {
    case "message":
    case "note": {
      const name = step.kind === "note" ? "Note" : "Message";
      return (
        <Section title={name}>
          <Words label={name} value={fields.text ?? ""} onChange={set("text")} />
        </Section>
      );
    }
    case "buttons":
      return (
        <Section title="Question and buttons">
          <Words label="Question" value={fields.text ?? ""} onChange={set("text")} />
          {step.paths?.map((path, index) => (
            <Row key={path.id} label={`Button ${index + 1}`}>
              <Field
                label={`Button ${index + 1}`}
                value={path.label}
                onChange={(label) => onEdit((current) => withPathLabel(current, path.id, label))}
              />
            </Row>
          ))}
        </Section>
      );
    case "collect":
      return (
        <Section title="Question">
          <Words label="Question" value={fields.text ?? ""} onChange={set("text")} />
          <Row label="Save the answer as">
            <Choice
              label="Save the answer as"
              options={options.saveAs}
              value={fields.saveAs}
              onChange={set("saveAs")}
            />
          </Row>
        </Section>
      );
    case "branch":
      return (
        <Section title="Paths" description="The first path whose condition matches wins.">
          {step.paths?.map((path) => (
            <Row key={path.id} label={path.label}>
              <Action>Edit condition</Action>
            </Row>
          ))}
        </Section>
      );
    case "dimmi":
      return <DimmiFields />;
    case "assign":
      return (
        <Section title="Who takes it">
          <Row label="Assignee">
            <Choice label="Assignee" options={options.assignee} value={fields.assignee} onChange={set("assignee")} />
          </Row>
        </Section>
      );
    case "tag":
      return (
        <Section title="Which tag">
          <Row label="Tag">
            <Choice label="Tag" options={options.tag} value={fields.tag} onChange={set("tag")} />
          </Row>
        </Section>
      );
    case "wait":
      return (
        <Section title="How long">
          <Row label="For">
            <Choice label="Wait for" options={options.wait} value={fields.wait} onChange={set("wait")} />
          </Row>
          <Row label="Stop when the customer replies">
            <Toggle label="Stop when the customer replies" on={fields.stop !== "off"} onChange={flip("stop")} />
          </Row>
        </Section>
      );
    case "close":
      return (
        <Section title="Before closing">
          <Row label="Ask for a rating first">
            <Toggle label="Ask for a rating first" on={fields.rating === "on"} onChange={flip("rating")} />
          </Row>
        </Section>
      );
    default:
      return (
        <Section title="Step">
          <Row label={step.summary} />
        </Section>
      );
  }
}

function DimmiFields() {
  return (
    <>
      <Section title="While Dimmi handles it">
        <Row label="Ask for details before handing over">
          <Toggle label="Ask for details before handing over" on />
        </Row>
        <Row label="Follow up when the customer goes quiet" description="After 4 minutes.">
          <Toggle label="Follow up when the customer goes quiet" on />
        </Row>
        <Row label="Close when the customer confirms">
          <Toggle label="Close when the customer confirms" on />
        </Row>
      </Section>
      <Section
        title="Always"
        description="Dimmi only answers typed messages, stops once a teammate replies, and respects office hours when it hands over."
      >
        <Row label="Outcomes" description="Resolved, hands over, or no answer. Each continues down its path." />
      </Section>
    </>
  );
}

export function Inspector({
  trigger,
  step,
  onRemove,
  onEdit,
  onClose,
  className,
}: {
  trigger: TriggerKind;
  step: Step | undefined;
  onRemove: (id: string) => void;
  onEdit: (id: string, edit: (step: Step) => Step) => void;
  onClose?: () => void;
  className?: string;
}) {
  return (
    <Panel label="Step settings" ground className={className}>
      <PageHeader title={step ? stepLook[step.kind].name : "Trigger"} level={2}>
        {onClose ? (
          <IconButton size="small" icon={RiCloseLine} aria-label="Close step settings" onClick={onClose} />
        ) : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          {step ? (
            <StepFields key={step.id} step={step} onEdit={(edit) => onEdit(step.id, edit)} />
          ) : (
            <TriggerSettings trigger={trigger} />
          )}
          {step ? (
            <Section title="Remove">
              <Row label="Remove this step" description="Steps on its paths go with it.">
                <Danger onPress={() => onRemove(step.id)}>Remove</Danger>
              </Row>
            </Section>
          ) : null}
        </div>
      </ScrollArea>
    </Panel>
  );
}
