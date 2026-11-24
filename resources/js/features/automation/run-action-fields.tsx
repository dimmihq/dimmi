/*
 * The settings of a step that runs one of the workspace's actions, the
 * same ones agents call: the action, picked from the published ones;
 * where each of its inputs comes from; which of its outputs to save, and
 * where; and whether a failed call takes a path of its own. Connections
 * and credentials stay on the action, in Settings, Actions.
 */
import { useQuery } from "@tanstack/react-query";

import { Action as Button, Danger, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type Action, actionsQuery } from "@/lib/api/ai-actions";

import { type EditorProps, Note } from "./field-parts";
import { Pick } from "./pick";
import { SaveTarget } from "./save-target";
import { text, useSet, withFailurePath, withSettings } from "./step-settings";
import { inputsOf } from "./step-values";
import { ValueSource } from "./value-source";
import type { Step } from "./workflows";

interface Input {
  name: string;
  value: string;
}

interface Kept {
  output: string;
  field: string;
}

function rows<T>(step: Step, name: string): T[] {
  const value = step.settings?.[name];

  return Array.isArray(value) ? (value as T[]) : [];
}

function Inputs({ step, action, onEdit }: EditorProps & { action: Action }) {
  const inputs = rows<Input>(step, "inputs");
  const set = (name: string, value: string) => {
    const others = inputs.filter((input) => input.name !== name);
    const next = value ? [...others, { name, value }] : others;
    onEdit((current) => withSettings(current, { inputs: next.length > 0 ? next : undefined }));
  };
  const wanted = inputsOf(action);

  if (wanted.length === 0) {
    return (
      <Section title="Inputs">
        <Note>This action takes no inputs.</Note>
      </Section>
    );
  }

  return (
    <Section title="Inputs" description="Where each input comes from when the step runs.">
      {wanted.map((input) => (
        <Row key={input.name} label={input.name} description={input.required ? "Required" : "Optional"}>
          <ValueSource
            label={`Input ${input.name}`}
            value={inputs.find((item) => item.name === input.name)?.value ?? ""}
            onChange={(value) => set(input.name, value)}
          />
        </Row>
      ))}
    </Section>
  );
}

function Outputs({ step, action, onEdit }: EditorProps & { action: Action }) {
  const kept = rows<Kept>(step, "save");
  const save = (next: Kept[]) =>
    onEdit((current) => withSettings(current, { save: next.length > 0 ? next : undefined }));
  const outputs = action.response_fields.map((field) => [field.name, field.name] as const);

  if (outputs.length === 0) {
    return (
      <Section title="Save from the result">
        <Note>This action names no outputs. Add response fields to it in Settings, Actions to save them.</Note>
      </Section>
    );
  }

  return (
    <Section title="Save from the result" description="Later steps read saved outputs as variables.">
      {kept.map((item, index) => (
        <Row
          key={`${String(index)}-${String(kept.length)}`}
          label={
            <Pick
              label={`Output ${index + 1}`}
              choices={outputs}
              value={item.output}
              onChange={(output) => save(kept.map((old, at) => (at === index ? { ...old, output } : old)))}
            />
          }
        >
          <SaveTarget
            label={`Save output ${index + 1} as`}
            value={item.field}
            onChange={(field) => save(kept.map((old, at) => (at === index ? { ...old, field: field ?? "" } : old)))}
          />
          <Danger onPress={() => save(kept.filter((_, at) => at !== index))}>Remove</Danger>
        </Row>
      ))}
      {kept.length < 10 ? (
        <Row label="Another output">
          <Button onPress={() => save([...kept, { output: outputs[0][0], field: "" }])}>Save an output</Button>
        </Row>
      ) : null}
    </Section>
  );
}

export function RunActionFields(props: EditorProps) {
  const { step, onEdit } = props;
  const set = useSet(onEdit);
  const workspace = useAccount().workspace.id;
  const actions = useQuery({ ...actionsQuery(workspace, { status: "published" }), retry: false }).data ?? [];
  const id = text(step.settings, "action_id");
  const action = actions.find((item) => item.id === id);
  const failing = (step.paths ?? []).some((path) => path.outcome === "failure");

  return (
    <>
      <Section title="Action">
        <Note>
          Runs one of your published actions, the same ones your agents call. Manage them in Settings, Actions.
        </Note>
        <Row label="Action">
          <Pick
            key={`actions-${actions.length}`}
            label="Action"
            choices={actions.map((item) => [item.id, item.title] as const)}
            value={id}
            placeholder={actions.length === 0 ? "No published actions" : "Pick an action"}
            onChange={(next) => set({ action_id: next, inputs: undefined, save: undefined })}
          />
        </Row>
        {action ? <Row label="What it does" description={action.description} /> : null}
      </Section>
      {action ? (
        <>
          <Inputs {...props} action={action} />
          <Outputs {...props} action={action} />
        </>
      ) : null}
      <Section title="When the action fails">
        <Row label="Take a separate path when the action fails">
          <Toggle
            label="Take a separate path when the action fails"
            on={failing}
            onChange={(on) => onEdit((current) => withFailurePath(current, on))}
          />
        </Row>
      </Section>
    </>
  );
}
