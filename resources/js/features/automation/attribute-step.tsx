/*
 * The settings of the step that sets conversation attributes: one row
 * per attribute set, named by the attribute, with its value; a picker
 * of the active attributes not set yet adds one. A value may hold
 * variables such as {{run.vars.order_number}}, a list takes one of its
 * options, and an empty value clears the attribute. A value the
 * attribute refuses fails the step when it runs.
 */
import { useQuery } from "@tanstack/react-query";

import { Danger, Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { conversationAttributesQuery } from "@/lib/api/conversation-attributes";

import { type EditorProps, Note } from "./field-parts";
import { Pick } from "./pick";
import { withSettings } from "./step-settings";

interface AttributeValue {
  key: string;
  value: string;
}

function valuesOf(settings: Record<string, unknown> | undefined): AttributeValue[] {
  const list = settings?.attributes;

  return Array.isArray(list) ? (list as AttributeValue[]) : [];
}

export function SetAttributesFields({ step, onEdit }: EditorProps) {
  const workspace = useAccount().workspace.id;
  const attributes = (useQuery({ ...conversationAttributesQuery(workspace), retry: false }).data ?? []).filter(
    (a) => a.status === "active",
  );
  const values = valuesOf(step.settings);
  const save = (next: AttributeValue[]) =>
    onEdit((current) => withSettings(current, { attributes: next.length > 0 ? next : undefined }));
  const unused = attributes
    .filter((a) => !values.some((value) => value.key === a.key))
    .map((a) => [a.key, a.label] as const);

  return (
    <Section title="Attribute values" description="Map answers and variables. An empty value clears it.">
      {attributes.length === 0 ? <Note>No conversation attributes yet. Add them in Settings.</Note> : null}
      {values.map((item, index) => {
        const attribute = attributes.find((a) => a.key === item.key);
        const name = attribute?.label ?? item.key;
        const change = (value: string) => save(values.map((old, at) => (at === index ? { ...old, value } : old)));

        return (
          <Row key={item.key} label={name}>
            <div className="flex items-center gap-2">
              {attribute?.data_type === "choice" ? (
                <Pick
                  label={`${name} value`}
                  placeholder="Clear it"
                  choices={attribute.options.map((o) => [o, o] as const)}
                  value={item.value}
                  onChange={change}
                />
              ) : attribute?.data_type === "boolean" ? (
                <Pick
                  label={`${name} value`}
                  placeholder="Clear it"
                  choices={[
                    ["true", "Yes"],
                    ["false", "No"],
                  ]}
                  value={item.value}
                  onChange={change}
                />
              ) : (
                <Field
                  label={`${name} value`}
                  value={item.value}
                  width="w-40"
                  placeholder="{{run.vars.order_number}}"
                  onChange={change}
                />
              )}
              <Danger onPress={() => save(values.filter((_, at) => at !== index))}>Remove</Danger>
            </div>
          </Row>
        );
      })}
      {unused.length > 0 ? (
        <Row label="Set an attribute">
          <Pick
            key={values.length}
            label="Attribute to set"
            placeholder="Pick an attribute"
            choices={unused}
            value=""
            onChange={(key) => save([...values, { key, value: "" }])}
          />
        </Row>
      ) : null}
    </Section>
  );
}
