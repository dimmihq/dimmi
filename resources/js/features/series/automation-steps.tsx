/*
 * The settings of the series steps that share building blocks with
 * agents. An extract step lets the AI read the person's profile and what
 * they did lately and write typed values to their attributes, a
 * fallback standing in when it is unsure. A run action step calls one of
 * the workspace's published actions with inputs filled from the person
 * and writes the outputs it picks to their attributes. Attributes and
 * actions are picked from the workspace's own, never typed in.
 */
import { RiAddLine, RiDeleteBinLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Row, Section } from "@/features/settings/layout";
import { OptionsEditor } from "@/features/settings/options-editor";
import { type Action, actionsQuery } from "@/lib/api/ai-actions";
import { attributesQuery } from "@/lib/api/attributes";
import type { ExtractValue } from "@/lib/api/series";

import { Body, Field, Pick, Text } from "./inputs";
import type { StepProps } from "./step-settings";

const types: [ExtractValue["type"], string][] = [
  ["text", "Text"],
  ["number", "Number"],
  ["date", "Date"],
  ["boolean", "Yes or no"],
  ["choice", "One of a list"],
];

const newKey = "__new";

/*
 * useAttributeKeys lists the attributes the traveler carries: the
 * company's in an account series, the person's otherwise.
 */
function useAttributeKeys(workspace: string, account: boolean): [string, string][] {
  const defs = useQuery({ ...attributesQuery(workspace), retry: false }).data ?? [];

  return defs
    .filter((d) => d.status === "active" && d.of === (account ? "company" : "contact"))
    .map((d) => [d.key, d.label]);
}

/*
 * AttributeKey picks the attribute a value is written to, or names a new
 * one, which is defined from its first value.
 */
function AttributeKey({
  label,
  value,
  keys,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  keys: [string, string][];
  disabled: boolean;
  onChange: (key: string) => void;
}) {
  const known = keys.some(([key]) => key === value);
  const picked = value && !known ? newKey : value;

  return (
    <span className="flex flex-wrap items-center gap-2">
      <Pick
        label={label}
        value={picked || undefined}
        options={[...keys, [newKey, "New attribute"]]}
        onChange={(key) => onChange(key === newKey ? "" : key)}
        disabled={disabled}
      />
      {picked === newKey || (!value && !known && keys.length === 0) ? (
        <Text
          label={`${label} key`}
          value={value}
          placeholder="team_size"
          width="w-32"
          disabled={disabled}
          onChange={(key) => onChange(key.toLowerCase().replace(/[^a-z0-9_]/g, "_"))}
        />
      ) : null}
    </span>
  );
}

function Fallback({
  value,
  disabled,
  onChange,
}: {
  value: ExtractValue;
  disabled: boolean;
  onChange: (fallback: unknown) => void;
}) {
  if (value.type === "boolean" || value.type === "choice") {
    const options: [string, string][] =
      value.type === "boolean"
        ? [
            ["true", "Yes"],
            ["false", "No"],
          ]
        : (value.options ?? []).map((o): [string, string] => [o, o]);

    return (
      <Pick
        label={`${value.key} fallback`}
        value={value.fallback === undefined ? "none" : String(value.fallback)}
        options={[["none", "No fallback"], ...options]}
        disabled={disabled}
        onChange={(picked) =>
          onChange(picked === "none" ? undefined : value.type === "boolean" ? picked === "true" : picked)
        }
      />
    );
  }

  return (
    <Text
      label={`${value.key} fallback`}
      value={value.fallback === undefined ? "" : String(value.fallback)}
      placeholder="No fallback"
      width="w-32"
      disabled={disabled}
      onChange={(typed) => {
        const t = typed.trim();
        onChange(!t ? undefined : value.type === "number" && !Number.isNaN(Number(t)) ? Number(t) : t);
      }}
    />
  );
}

export function ExtractStep({ workspace, node, account, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const fields = s.fields ?? [];
  const keys = useAttributeKeys(workspace, account);
  const setFields = (next: ExtractValue[]) => set({ ...s, fields: next });

  return (
    <>
      <Section
        title="What the AI reads"
        description="The person, their company and what they did lately. It writes only what it is sure of."
      >
        <div className="py-3">
          <Body
            label="Instructions"
            value={s.prompt}
            onChange={(prompt) => set({ ...s, prompt })}
            rows={3}
            disabled={!editable}
          />
        </div>
      </Section>
      {fields.map((f, i) => {
        const change = (next: Partial<ExtractValue>) =>
          setFields(fields.map((g, j) => (i === j ? { ...g, ...next } : g)));

        return (
          <Section
            key={`${String(i)}-${String(fields.length)}`}
            title={`Value ${i + 1}`}
            action={
              editable && fields.length > 1 ? (
                <LinkButton
                  size="xs"
                  variant="secondary"
                  leadingIcon={RiDeleteBinLine}
                  onClick={() => setFields(fields.filter((_, j) => j !== i))}
                >
                  Remove
                </LinkButton>
              ) : null
            }
          >
            <Row label="Write to">
              <AttributeKey
                label={`Value ${i + 1} attribute`}
                value={f.key}
                keys={keys}
                disabled={!editable}
                onChange={(key) => change({ key })}
              />
            </Row>
            <Row label="Type">
              <Pick
                label={`Value ${i + 1} type`}
                value={f.type}
                options={types}
                disabled={!editable}
                onChange={(type) =>
                  change({ type, options: type === "choice" ? (f.options ?? []) : undefined, fallback: undefined })
                }
              />
            </Row>
            <Field label="What it means">
              <Text
                label={`Value ${i + 1} description`}
                value={f.description}
                placeholder="How many people would use it"
                width="w-full"
                disabled={!editable}
                onChange={(description) => change({ description })}
              />
            </Field>
            {f.type === "choice" ? (
              <div className="py-2.5 pe-2.5">
                <OptionsEditor
                  label={`Value ${i + 1} choices`}
                  value={f.options ?? []}
                  isDisabled={!editable}
                  onChange={(options) => change({ options: options.filter((o) => o.trim() !== "") })}
                />
              </div>
            ) : null}
            <Row
              label="When the AI is not sure"
              description="Written instead. Without one, the not found path is taken."
            >
              <Fallback key={f.type} value={f} disabled={!editable} onChange={(fallback) => change({ fallback })} />
            </Row>
          </Section>
        );
      })}
      {editable && fields.length < 10 ? (
        <div>
          <Button
            size="xs"
            variant="secondary"
            leadingIcon={RiAddLine}
            onClick={() => setFields([...fields, { key: "", type: "text" }])}
          >
            Add value
          </Button>
        </div>
      ) : null}
    </>
  );
}

/*
 * details are the details of the person an input can read.
 */
const details: [string, string][] = [
  ["{{ contact.email }}", "Email"],
  ["{{ contact.name }}", "Name"],
  ["{{ contact.first_name }}", "First name"],
  ["{{ contact.phone }}", "Phone"],
  ["{{ contact.external_id }}", "External id"],
  ["{{ contact.id }}", "Contact id"],
  ["{{ company.name }}", "Company name"],
  ["{{ company.id }}", "Company id"],
];

const fixedValue = "__fixed";

function InputSource({
  workspace,
  label,
  value,
  disabled,
  onChange,
}: {
  workspace: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const defs = useQuery({ ...attributesQuery(workspace), retry: false }).data ?? [];
  const attributes = defs
    .filter((d) => d.status === "active")
    .map((d): [string, string] => [
      `{{ ${d.of === "company" ? "company" : "contact"}.attr.${d.key} }}`,
      `${d.of === "company" ? "Company" : "Person"}: ${d.label}`,
    ]);
  const options: [string, string][] = [...details, ...attributes, [fixedValue, "Fixed value"]];
  const known = options.some(([id]) => id === value);
  const picked = known ? value : value ? fixedValue : undefined;

  return (
    <span className="flex flex-wrap items-center gap-2">
      <Pick
        label={`${label} from`}
        value={picked}
        options={options}
        disabled={disabled}
        onChange={(next) => onChange(next === fixedValue ? "" : next)}
      />
      {picked === fixedValue ? (
        <Text label={`${label} value`} value={value} width="w-32" disabled={disabled} onChange={onChange} />
      ) : null}
    </span>
  );
}

/*
 * inputNames lists an action's inputs a step fills, required first, the
 * ones the admin bound left out.
 */
function inputNames(action: Action | undefined): { name: string; required: boolean }[] {
  const schema = action?.input_schema as { properties?: Record<string, unknown>; required?: string[] } | undefined;
  const bound = Object.keys(action?.bound_arguments ?? {});

  return Object.keys(schema?.properties ?? {})
    .filter((name) => !bound.includes(name))
    .map((name) => ({ name, required: (schema?.required ?? []).includes(name) }))
    .sort((a, b) => Number(b.required) - Number(a.required));
}

export function RunActionStep({ workspace, node, account, editable, set }: StepProps) {
  const s = node.settings ?? {};
  const actions = useQuery({ ...actionsQuery(workspace, { status: "published" }), retry: false }).data ?? [];
  const action = actions.find((a) => a.id === s.action_id);
  const keys = useAttributeKeys(workspace, account);
  const inputs = s.inputs ?? [];
  const saves = s.save ?? [];
  const setInput = (name: string, value: string) => {
    const others = inputs.filter((i) => i.name !== name);
    set({ ...s, inputs: value ? [...others, { name, value }] : others });
  };
  const outputs = (action?.response_fields ?? []).map((f): [string, string] => [f.name, f.name]);

  return (
    <>
      <Section title="Action" description="One of your published actions, the same ones your agents call.">
        <Row label="Action">
          <Pick
            label="Action"
            value={s.action_id || undefined}
            options={actions.map((a): [string, string] => [a.id, a.title])}
            placeholder={actions.length === 0 ? "No published actions" : "Choose…"}
            disabled={!editable}
            onChange={(action_id) => set({ ...s, action_id, inputs: [], save: [] })}
          />
        </Row>
      </Section>
      {action ? (
        <Section title="Inputs" description="Where each input comes from.">
          {inputNames(action).map((input) => (
            <Row key={input.name} label={input.name} description={input.required ? "Required" : "Optional"}>
              <InputSource
                workspace={workspace}
                label={`Input ${input.name}`}
                value={inputs.find((i) => i.name === input.name)?.value ?? ""}
                disabled={!editable}
                onChange={(value) => setInput(input.name, value)}
              />
            </Row>
          ))}
        </Section>
      ) : null}
      {action && outputs.length > 0 ? (
        <Section
          title="Write from the result"
          action={
            editable ? (
              <Button
                size="xs"
                variant="secondary"
                leadingIcon={RiAddLine}
                onClick={() => set({ ...s, save: [...saves, { output: outputs[0][0], attribute: "" }] })}
              >
                Add output
              </Button>
            ) : null
          }
        >
          {saves.map((sv, i) => (
            <Row
              key={`${String(i)}-${String(saves.length)}`}
              label={
                <Pick
                  label={`Output ${i + 1}`}
                  value={sv.output}
                  options={outputs}
                  disabled={!editable}
                  onChange={(output) => set({ ...s, save: saves.map((o, j) => (i === j ? { ...o, output } : o)) })}
                />
              }
            >
              <AttributeKey
                label={`Output ${i + 1} attribute`}
                value={sv.attribute}
                keys={keys}
                disabled={!editable}
                onChange={(attribute) => set({ ...s, save: saves.map((o, j) => (i === j ? { ...o, attribute } : o)) })}
              />
            </Row>
          ))}
        </Section>
      ) : null}
    </>
  );
}
