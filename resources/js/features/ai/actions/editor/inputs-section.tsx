/*
 * What the agent fills in when it calls the action, one row per input:
 * a name, a type, whether it is required, what it means (the agent reads
 * this) and its limits. Text can be limited to a list of choices and
 * numbers to a range, which is how a refund stays under a ceiling no
 * matter what a customer says. Edit as JSON opens the schema itself for
 * anything the rows do not cover; the rows take it back when applied.
 */
import { RiAddLine, RiBracesLine, RiCloseLine } from "@remixicon/react";
import { useState } from "react";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { Section } from "@/features/settings/layout";

import { FIELD_TYPES, type FieldType } from "../api";
import { rowKey } from "../draft";
import { fieldsFromSchema, fieldTypeLabels, type InputField, parseSchema, schemaFromFields } from "../schema";
import type { Editor } from "./use-editor";

function blankField(): InputField {
  return {
    key: rowKey(),
    name: "",
    type: "string",
    required: true,
    description: "",
    choices: "",
    minimum: "",
    maximum: "",
  };
}

function FieldRow({
  field,
  index,
  onChange,
  onRemove,
}: {
  field: InputField;
  index: number;
  onChange: (change: Partial<InputField>) => void;
  onRemove: () => void;
}) {
  const label = field.name || `Input ${index + 1}`;
  const numeric = field.type === "number" || field.type === "integer";

  return (
    <li className="flex flex-col gap-2 border-b border-separator-border py-3 pe-3 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          aria-label={`Input ${index + 1} name`}
          size="small"
          inputDir="ltr"
          placeholder="invoice_id"
          value={field.name}
          onChange={(name) => onChange({ name })}
          className="min-w-0 flex-1 basis-40"
          fieldClassName="font-mono"
        />
        <Select
          aria-label={`${label} type`}
          size="sm"
          selectedKey={field.type}
          onSelectionChange={(key) => onChange({ type: String(key) as FieldType })}
          className="w-36"
        >
          {FIELD_TYPES.map((type) => (
            <SelectItem key={type} id={type} textValue={fieldTypeLabels[type]}>
              {fieldTypeLabels[type]}
            </SelectItem>
          ))}
        </Select>
        <Checkbox size="sm" isSelected={field.required} onChange={(required) => onChange({ required })}>
          Required
        </Checkbox>
        <IconButton size="small" icon={RiCloseLine} aria-label={`Remove ${label}`} onClick={onRemove} />
      </div>
      <Input
        aria-label={`${label} description`}
        size="small"
        placeholder="What it is and where the agent finds it, such as “The invoice number, like inv_1001”"
        value={field.description}
        onChange={(description) => onChange({ description })}
      />
      {field.type === "string" ? (
        <Input
          aria-label={`${label} allowed values`}
          size="small"
          placeholder="Allowed values, separated by commas (leave empty for any text)"
          value={field.choices}
          onChange={(choices) => onChange({ choices })}
        />
      ) : null}
      {numeric ? (
        <div className="flex gap-2">
          <Input
            aria-label={`${label} minimum`}
            size="small"
            inputMode="decimal"
            placeholder="Minimum"
            value={field.minimum}
            onChange={(minimum) => onChange({ minimum })}
            className="flex-1"
          />
          <Input
            aria-label={`${label} maximum`}
            size="small"
            inputMode="decimal"
            placeholder="Maximum"
            value={field.maximum}
            onChange={(maximum) => onChange({ maximum })}
            className="flex-1"
          />
        </div>
      ) : null}
    </li>
  );
}

/*
 * JsonEditor is the schema as text, applied back to the rows.
 */
function JsonEditor({ fields, onApply }: { fields: InputField[]; onApply: (fields: InputField[]) => void }) {
  const [text, setText] = useState(() => JSON.stringify(schemaFromFields(fields), null, 2));
  const [problem, setProblem] = useState<string | null>(null);

  const apply = () => {
    const parsed = parseSchema(text);
    if ("problem" in parsed) {
      setProblem(parsed.problem);
      return;
    }
    onApply(fieldsFromSchema(parsed.schema));
  };

  return (
    <div className="flex flex-col gap-2 py-3 pe-3">
      <Textarea
        aria-label="Input schema"
        rows={8}
        autoResize
        maxRows={24}
        value={text}
        onChange={(next) => {
          setText(next);
          setProblem(null);
        }}
        isInvalid={problem !== null}
        fieldClassName="font-mono"
      />
      {problem ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {problem}
        </p>
      ) : null}
      <Button size="xs" variant="secondary" className="self-start" onClick={apply}>
        Apply schema
      </Button>
    </div>
  );
}

export function InputsSection({ editor }: { editor: Editor }) {
  const { draft, update, problems } = editor;
  const [json, setJson] = useState(false);
  const setField = (key: string, change: Partial<InputField>) =>
    update({ fields: draft.fields.map((field) => (field.key === key ? { ...field, ...change } : field)) });

  return (
    <Section
      title="Inputs"
      description="What the agent fills in from the conversation. Limits are enforced before anything is sent."
      action={
        <LinkButton size="small" variant="secondary" leadingIcon={RiBracesLine} onClick={() => setJson(!json)}>
          {json ? "Edit as rows" : "Edit as JSON"}
        </LinkButton>
      }
    >
      {json ? (
        <JsonEditor
          fields={draft.fields}
          onApply={(fields) => {
            update({ fields });
            setJson(false);
          }}
        />
      ) : (
        <>
          {draft.fields.length === 0 ? (
            <p className="py-3 pe-3 text-body-2-regular text-text-tertiary">
              The agent fills in nothing. Add what it needs from the conversation, such as an invoice number.
            </p>
          ) : (
            <ul aria-label="Inputs">
              {draft.fields.map((field, index) => (
                <FieldRow
                  key={field.key}
                  field={field}
                  index={index}
                  onChange={(change) => setField(field.key, change)}
                  onRemove={() => update({ fields: draft.fields.filter((other) => other.key !== field.key) })}
                />
              ))}
            </ul>
          )}
          {problems.fields ? (
            <p role="alert" className="pe-3 pb-1 text-body-2-regular text-text-error-primary">
              {problems.fields}
            </p>
          ) : null}
          <div className="py-3 pe-3">
            <Button
              variant="secondary"
              size="xs"
              leadingIcon={RiAddLine}
              onClick={() => update({ fields: [...draft.fields, blankField()] })}
            >
              Add input
            </Button>
          </div>
        </>
      )}
    </Section>
  );
}
