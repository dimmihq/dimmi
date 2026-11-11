/*
 * The connect form of any source, drawn from its driver's JSON Schema:
 * the connected account, settings picked from what that account holds,
 * text for strings, a picker for a list of choices, a switch for yes or
 * no, a number field, one entry per line for lists, and a file picker
 * for an upload. Changing a setting clears the ones picked with it, so
 * a new repository never keeps the old one's branch or files. The
 * server checks the settings against the real service, so the form
 * only marks what is required.
 */
import { useMutation } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import { type SchemaProperty, type SourceType, uploadArchive } from "@/lib/api/knowledge-sources";

import { ChoiceField } from "./choice-field";
import { AccountField } from "./connect-field";
import type { FormValues } from "./model";

export function SchemaForm({
  workspace,
  kind,
  values,
  secretsSet = [],
  errors = {},
  skipAccount = false,
  onChange,
}: {
  workspace: string;
  kind: SourceType;
  values: FormValues;
  secretsSet?: string[];
  errors?: Record<string, string>;
  skipAccount?: boolean;
  onChange: (values: FormValues) => void;
}) {
  const schema = kind.schema;
  const fields = Object.entries(schema.properties).filter(
    ([, prop]) => prop.format !== "x-tree" && !(skipAccount && prop.format === "x-integration"),
  );
  const set = (name: string, value: unknown) => {
    if (values[name] !== value) {
      onChange(withDependents(schema.properties, { ...values, [name]: value }, name));
    }
  };

  return (
    <div className="flex flex-col gap-4">
      {fields.map(([name, prop]) => {
        if (prop.format === "x-integration" && kind.integration) {
          return (
            <AccountField
              key={name}
              workspace={workspace}
              kind={kind}
              provider={kind.integration}
              prop={prop}
              value={values[name]}
              onChange={(value) => set(name, value)}
            />
          );
        }
        if (prop.format === "x-choice") {
          return (
            <ChoiceField
              key={name}
              workspace={workspace}
              type={kind.type}
              name={name}
              prop={prop}
              values={values}
              error={errors[`settings.${name}`]}
              onChange={(value) => set(name, value)}
            />
          );
        }

        return (
          <SchemaField
            key={name}
            workspace={workspace}
            name={name}
            prop={prop}
            required={schema.required?.includes(name) ?? false}
            saved={secretsSet.includes(name)}
            value={values[name]}
            error={errors[`settings.${name}`]}
            onChange={(value) => set(name, value)}
          />
        );
      })}
    </div>
  );
}

/*
 * withDependents clears every setting read with the one that changed,
 * and the ticks of the tree, which follow the settings above them.
 */
function withDependents(properties: Record<string, SchemaProperty>, values: FormValues, changed: string): FormValues {
  const out = { ...values };
  for (const [name, prop] of Object.entries(properties)) {
    if (prop["x-depends"]?.includes(changed)) {
      out[name] = "";
    }
    if (prop.format === "x-tree" && name !== changed) {
      out[name] = [];
    }
  }

  return out;
}

/*
 * FieldProps is what every kind of field is drawn from.
 */
interface FieldProps {
  workspace: string;
  name: string;
  label: string;
  prop: SchemaProperty;
  saved: boolean;
  value: unknown;
  error?: string;
  onChange: (value: unknown) => void;
}

function SchemaField({
  required,
  ...props
}: Omit<FieldProps, "label"> & {
  required: boolean;
}) {
  const label = `${props.prop.title ?? props.name}${required ? "" : " (optional)"}`;
  const Field = fieldFor(props.prop);

  return <Field {...props} label={label} />;
}

/*
 * fieldFor picks the control a setting is typed with.
 */
function fieldFor(prop: SchemaProperty): (props: FieldProps) => ReactNode {
  if (prop.format === "x-upload") {
    return UploadField;
  }
  if (prop.enum) {
    return EnumField;
  }
  if (prop.type === "boolean") {
    return SwitchField;
  }

  return prop.type === "array" ? ListField : TextField;
}

function EnumField({ label, prop, value, onChange }: FieldProps) {
  const options = prop.enum ?? [];
  const labelOf = (i: number) => prop["x-enum-labels"]?.[i] ?? options[i];

  return (
    <Select
      label={label}
      selectedKey={String(value ?? prop.default ?? options[0])}
      onSelectionChange={(key) => key !== null && onChange(String(key))}
    >
      {options.map((option, i) => (
        <SelectItem key={option} id={option} textValue={labelOf(i)}>
          {labelOf(i)}
        </SelectItem>
      ))}
    </Select>
  );
}

function SwitchField({ name, prop, value, onChange }: FieldProps) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex min-w-0 flex-col">
        <span className="text-body-medium text-text-primary">{prop.title ?? name}</span>
        {prop.description ? <span className="text-body-2-regular text-text-tertiary">{prop.description}</span> : null}
      </div>
      <Switch aria-label={prop.title ?? name} size="sm" isSelected={Boolean(value)} onChange={onChange} />
    </div>
  );
}

function ListField({ label, prop, value, onChange }: FieldProps) {
  return (
    <Textarea
      label={label}
      hint={prop.description ?? "One per line."}
      rows={3}
      value={Array.isArray(value) ? value.join("\n") : ""}
      onChange={(text) => onChange(text.split("\n"))}
    />
  );
}

function TextField({ label, prop, saved, value, error, onChange }: FieldProps) {
  const type = prop["x-secret"] ? "password" : prop.type === "integer" ? "number" : "text";

  return (
    <Input
      label={label}
      type={type}
      placeholder={saved ? "Saved. Type a new one to replace it." : prop["x-placeholder"]}
      hint={error ?? prop.description}
      isInvalid={error !== undefined}
      value={value === undefined || value === null ? "" : String(value)}
      onChange={onChange}
      autoComplete="off"
    />
  );
}

/*
 * UploadField sends the chosen file at once and keeps the key the server
 * names it by.
 */
function UploadField({ workspace, label, prop, value, onChange }: FieldProps) {
  const upload = useMutation({
    mutationFn: (file: File) => uploadArchive(workspace, file),
    onSuccess: (stored) => onChange(stored.key),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "The file could not be uploaded."),
  });

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-body-2-medium text-text-secondary">{label}</span>
      <div className="flex items-center gap-3">
        <Button variant="secondary" size="small" loading={upload.isPending} onClick={() => pick(upload.mutate)}>
          {value ? "Choose another file" : "Choose file"}
        </Button>
        <span className="truncate text-body-2-regular text-text-tertiary">
          {upload.data?.name ?? (value ? "File uploaded" : "No file yet")}
        </span>
      </div>
      {prop.description ? <span className="text-body-2-regular text-text-tertiary">{prop.description}</span> : null}
    </div>
  );
}

function pick(send: (file: File) => void) {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".zip,.md,.mdx,.markdown";
  input.onchange = () => {
    const file = input.files?.[0];
    if (file) {
      send(file);
    }
  };
  input.click();
}
