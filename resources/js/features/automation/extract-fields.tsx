/*
 * The settings of a step that pulls typed values out of the
 * conversation with AI: what to read, the team's instructions, and each
 * value with its name, type, what it means, where it is saved and what
 * stands in when the AI is not sure. The AI never guesses: a value it
 * cannot find takes the fallback, or the not found path when there is
 * none.
 */
import { Action, Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { OptionsEditor } from "@/features/settings/options-editor";

import { choicesOf } from "./choices";
import { type EditorProps, Note, Words } from "./field-parts";
import { Pick } from "./pick";
import { SaveTarget } from "./save-target";
import { text, useSet, withSettings } from "./step-settings";
import { keyFor, withNonePath } from "./step-values";
import type { Step } from "./workflows";

/*
 * Value is one value the step looks for, as the server stores it.
 */
export interface Value {
  key: string;
  label?: string;
  description?: string;
  type: "text" | "number" | "date" | "boolean" | "choice";
  options?: string[];
  save_to?: string;
  fallback?: unknown;
}

const types = {
  text: "Text",
  number: "Number",
  date: "Date",
  boolean: "Yes or no",
  choice: "One of a list",
};

const sources = { conversation: "The whole conversation", last_message: "The customer's newest message" };

const most = 10;

function values(step: Step): Value[] {
  const fields = step.settings?.fields;

  return Array.isArray(fields) ? (fields as Value[]) : [];
}

function Fallback({ value, onChange }: { value: Value; onChange: (fallback: unknown) => void }) {
  const none = "none";
  if (value.type === "boolean" || value.type === "choice") {
    const options =
      value.type === "boolean"
        ? ([
            ["true", "Yes"],
            ["false", "No"],
          ] as const)
        : (value.options ?? []).map((option) => [option, option] as const);

    return (
      <Pick
        label={`${value.key} fallback`}
        choices={[[none, "No fallback"], ...options]}
        value={value.fallback === undefined ? none : String(value.fallback)}
        onChange={(picked) => {
          if (picked === none) {
            onChange(undefined);
          } else {
            onChange(value.type === "boolean" ? picked === "true" : picked);
          }
        }}
      />
    );
  }

  return (
    <Field
      label={`${value.key} fallback`}
      placeholder={value.type === "date" ? "2026-01-31" : "No fallback"}
      value={value.fallback === undefined ? "" : String(value.fallback)}
      width="w-32"
      onChange={(typed) => {
        const trimmed = typed.trim();
        if (!trimmed) {
          onChange(undefined);
        } else if (value.type === "number" && !Number.isNaN(Number(trimmed))) {
          onChange(Number(trimmed));
        } else {
          onChange(trimmed);
        }
      }}
    />
  );
}

function OneValue({
  value,
  index,
  count,
  onChange,
  onRemove,
}: {
  value: Value;
  index: number;
  count: number;
  onChange: (value: Value) => void;
  onRemove: () => void;
}) {
  return (
    <Section
      title={`Value ${index + 1}`}
      description={`Saved as ${value.save_to || `run.vars.${value.key || "key"}`}`}
      action={count > 1 ? <Danger onPress={onRemove}>Remove</Danger> : undefined}
    >
      <Row label="Name">
        <Field
          label={`Value ${index + 1} name`}
          value={value.label ?? ""}
          placeholder="Order number"
          width="w-40"
          onChange={(label) => onChange({ ...value, label, key: keyFor(label) || value.key })}
        />
      </Row>
      <Row label="Type">
        <Pick
          label={`Value ${index + 1} type`}
          choices={choicesOf(types)}
          value={value.type}
          onChange={(type) =>
            onChange({
              ...value,
              type: type as Value["type"],
              options: type === "choice" ? (value.options ?? []) : undefined,
              fallback: undefined,
            })
          }
        />
      </Row>
      <Words
        label={`Value ${index + 1} description`}
        value={value.description ?? ""}
        placeholder="What it means, to help the AI find it. Optional."
        onChange={(description) => onChange({ ...value, description: description.trim() ? description : undefined })}
      />
      {value.type === "choice" ? (
        <div className="py-2.5 pe-2.5">
          <OptionsEditor
            label={`Value ${index + 1} choices`}
            value={value.options ?? []}
            onChange={(options) => onChange({ ...value, options: options.filter((option) => option.trim() !== "") })}
          />
        </div>
      ) : null}
      <Row label="Save as" description="Later steps can use it.">
        <SaveTarget
          label={`Save value ${index + 1} as`}
          value={value.save_to ?? `run.vars.${value.key}`}
          onChange={(target) => onChange({ ...value, save_to: target })}
        />
      </Row>
      <Row label="When the AI is not sure" description="Saved instead. Without one, the not found path is taken.">
        <Fallback key={value.type} value={value} onChange={(fallback) => onChange({ ...value, fallback })} />
      </Row>
    </Section>
  );
}

export function ExtractFields({ step, onEdit }: EditorProps) {
  const set = useSet(onEdit);
  const list = values(step);
  const save = (next: Value[]) => onEdit((current) => withSettings(current, { fields: next }));
  const unsure = (step.paths ?? []).some((path) => path.outcome === "none");

  return (
    <>
      <Section title="AI">
        <Note>
          The AI reads the conversation and saves only what it is sure of. It uses the decision model, then your AI
          provider. Without one, every value takes its fallback.
        </Note>
        <Row label="Read">
          <Pick
            label="Read"
            choices={choicesOf(sources)}
            value={text(step.settings, "source") || "conversation"}
            onChange={(source) => set({ source: source === "conversation" ? undefined : source })}
          />
        </Row>
      </Section>
      <Section title="Instructions" description="Added to what the AI is told about the values.">
        <Words
          label="Instructions"
          value={text(step.settings, "instructions")}
          placeholder="Order numbers start with A-."
          onChange={(value) => set({ instructions: value.trim() ? value : undefined })}
        />
      </Section>
      {list.map((value, index) => (
        <OneValue
          key={`${String(index)}-${String(list.length)}`}
          value={value}
          index={index}
          count={list.length}
          onChange={(next) => save(list.map((old, at) => (at === index ? next : old)))}
          onRemove={() => save(list.filter((_, at) => at !== index))}
        />
      ))}
      <Section title="More values" description="Names must differ from one another.">
        {list.length < most ? (
          <Row label="Another value">
            <Action onPress={() => save([...list, { key: `value_${list.length + 1}`, type: "text" }])}>
              Add value
            </Action>
          </Row>
        ) : (
          <Note>A step looks for 10 values at most.</Note>
        )}
      </Section>
      <Section title="When a value is not found">
        <Row label="Take a separate path when a value has no fallback and is not found">
          <Toggle
            label="Take a separate path when a value is not found"
            on={unsure}
            onChange={(on) => onEdit((current) => withNonePath(current, on))}
          />
        </Row>
      </Section>
    </>
  );
}
