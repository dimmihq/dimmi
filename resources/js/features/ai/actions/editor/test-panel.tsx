/*
 * Trying an action before customers meet it. The admin plays a signed-in
 * customer (their ID, email and the attributes the action reads) and
 * fills in what the agent would. A dry run only builds the request; a
 * live run sends it, and a live run of an action that changes data asks
 * for a second yes. The result shows the request as sent with secrets
 * masked, the API's answer, and exactly what the agent would read, plus
 * the confirmation card a customer would see. Testing saves unsaved
 * changes first, since tests run the saved draft.
 */
import { RiCloseLine, RiPlayLine } from "@remixicon/react";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { ApiError } from "@/lib/api/client";

import { type ActionTestResult, testAction } from "../api";
import { type Draft, definitionOf } from "../draft";
import { humanize, requestText, responseBody, responseStatus } from "../format";
import { Segments } from "../parts";
import { type InputField, placeholdersIn } from "../schema";
import type { Editor } from "./use-editor";

type View = "agent" | "request" | "response";

/*
 * attributesUsed lists the customer attributes the action reads.
 */
function attributesUsed(draft: Draft): string[] {
  const texts = [draft.path, draft.body, ...draft.query.map((r) => r.value), ...draft.headers.map((r) => r.value)];
  const keys = texts
    .concat(draft.bound.map((row) => row.value))
    .flatMap(placeholdersIn)
    .map((token) => /^contact\.(?:attr|custom_attributes)\.([a-z0-9_]+)$/.exec(token)?.[1])
    .filter((key): key is string => Boolean(key));

  return [...new Set(keys)];
}

function coerce(field: InputField, value: string): unknown {
  if (field.type === "boolean") {
    return value === "true";
  }
  if (field.type === "number" || field.type === "integer") {
    const number = Number(value);
    return value.trim() === "" || Number.isNaN(number) ? undefined : number;
  }

  return value === "" ? undefined : value;
}

function FieldInput({ field, value, onChange }: { field: InputField; value: string; onChange: (v: string) => void }) {
  const label = humanize(field.name || "input");
  const choices = field.choices
    .split(",")
    .map((choice) => choice.trim())
    .filter(Boolean);
  if (field.type === "boolean") {
    return (
      <Checkbox size="sm" isSelected={value === "true"} onChange={(on) => onChange(on ? "true" : "false")}>
        {label}
      </Checkbox>
    );
  }
  if (choices.length > 0) {
    return (
      <Select label={label} size="sm" selectedKey={value || null} onSelectionChange={(key) => onChange(String(key))}>
        {choices.map((choice) => (
          <SelectItem key={choice} id={choice} textValue={choice}>
            {choice}
          </SelectItem>
        ))}
      </Select>
    );
  }

  return (
    <Input
      label={label}
      size="small"
      inputDir="ltr"
      inputMode={field.type === "string" ? undefined : "decimal"}
      placeholder={field.description || undefined}
      value={value}
      onChange={onChange}
    />
  );
}

function Code({ value }: { value: unknown }) {
  const text = typeof value === "string" ? value : JSON.stringify(value, null, 2);

  return (
    <pre
      dir="ltr"
      className="max-h-80 overflow-auto rounded-xl bg-background-tertiary-default p-3 font-mono text-caption-1-regular whitespace-pre-wrap break-all text-text-primary"
    >
      {text ?? "—"}
    </pre>
  );
}

const statusTones = { rendered: "blue", succeeded: "lime", failed: "rose", blocked: "orange" } as const;
const statusNames = {
  rendered: "Request built",
  succeeded: "Succeeded",
  failed: "Failed",
  blocked: "Blocked",
} as const;

function Result({ result, write }: { result: ActionTestResult; write: boolean }) {
  const [view, setView] = useState<View>("agent");
  const status = responseStatus(result.response);

  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      <div className="flex flex-wrap items-center gap-2">
        <Chip variant="caption" color={statusTones[result.status]}>
          {statusNames[result.status]}
        </Chip>
        {status !== null ? <span className="text-body-2-regular text-text-secondary">HTTP {status}</span> : null}
        {result.status !== "rendered" ? (
          <span className="text-body-2-regular text-text-tertiary">{result.latency_ms} ms</span>
        ) : null}
        {result.error_code ? (
          <code className="font-mono text-caption-1-medium text-text-error-primary">{result.error_code}</code>
        ) : null}
      </div>
      {result.validation_errors.length > 0 ? (
        <ul className="flex flex-col gap-1 text-body-2-regular text-text-error-primary">
          {result.validation_errors.map((error) => (
            <li key={error}>{error}</li>
          ))}
        </ul>
      ) : null}
      {write && result.confirmation_text ? (
        <div className="flex flex-col gap-1 rounded-xl bg-background-primary-default p-3 shadow-card">
          <span className="text-caption-1-medium text-text-tertiary">The customer would see</span>
          <span className="text-body-2-medium text-text-primary">{result.confirmation_text}</span>
        </div>
      ) : null}
      <Segments
        label="Result view"
        value={view}
        options={[
          { id: "agent", label: "Agent sees" },
          { id: "request", label: "Request" },
          { id: "response", label: "Response" },
        ]}
        onChange={setView}
      />
      {view === "agent" ? <Code value={result.result} /> : null}
      {view === "request" ? <Code value={requestText(result.request) || "No request was built."} /> : null}
      {view === "response" ? <Code value={responseBody(result.response) ?? "Nothing was sent."} /> : null}
    </div>
  );
}

/*
 * useRun runs a test of the saved draft, saving first when needed.
 */
function useRun(workspace: string, editor: Editor, onResult: (result: ActionTestResult) => void) {
  const [running, setRunning] = useState(false);

  const run = async (body: Parameters<typeof testAction>[2]) => {
    setRunning(true);
    try {
      const current = editor.dirty || !editor.saved ? await editor.store() : editor.saved;
      if (!current) {
        return;
      }
      onResult(await testAction(workspace, current.id, body));
    } catch (error) {
      toast.error("Test did not run", { description: error instanceof ApiError ? error.message : "Try again." });
    } finally {
      setRunning(false);
    }
  };

  return { run, running };
}

export function TestPanel({
  workspace,
  editor,
  kind,
  result,
  onResult,
  onClose,
  className,
}: {
  workspace: string;
  editor: Editor;
  kind: "http" | "mcp";
  result: ActionTestResult | null;
  onResult: (result: ActionTestResult) => void;
  onClose?: () => void;
  className?: string;
}) {
  const { draft } = editor;
  const [live, setLive] = useState(false);
  const [sure, setSure] = useState(false);
  const [customer, setCustomer] = useState<Record<string, string>>({ external_id: "", email: "" });
  const [values, setValues] = useState<Record<string, string>>({});
  const { run, running } = useRun(workspace, editor, onResult);
  const write = draft.side_effect === "write";
  const attributes = attributesUsed(draft);
  const fields = definitionOf(draft, kind).input_schema.properties;
  const blocked = live && write && !sure;

  const start = () => {
    const input = Object.fromEntries(
      draft.fields
        .filter((field) => field.name && fields[field.name])
        .map((field) => [field.name, coerce(field, values[field.key] ?? "")])
        .filter(([, value]) => value !== undefined),
    );
    const custom_attributes = Object.fromEntries(attributes.map((key) => [key, customer[`attr.${key}`] ?? ""]));
    void run({
      input,
      contact: {
        external_id: customer.external_id || undefined,
        email: customer.email || undefined,
        custom_attributes,
      },
      version: "draft",
      mode: live ? "live" : "dry_run",
      confirm_side_effects: live && write && sure,
    });
  };

  return (
    <Panel label="Test" ground className={className}>
      <PageHeader title="Test" level={2}>
        {onClose ? <IconButton size="small" icon={RiCloseLine} aria-label="Close test" onClick={onClose} /> : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <div className="flex flex-col gap-6 pt-2">
          <Segments
            label="Test mode"
            value={live ? "live" : "dry"}
            options={[
              { id: "dry", label: "Dry run" },
              { id: "live", label: "Live" },
            ]}
            onChange={(mode) => setLive(mode === "live")}
          />
          <Section title="Signed-in customer" description="Who the test runs as. Their details are trusted.">
            <div className="flex flex-col gap-3 py-3 pe-3">
              <Input
                label="Customer's ID in your system"
                size="small"
                inputDir="ltr"
                placeholder="user_42"
                value={customer.external_id}
                onChange={(external_id) => setCustomer({ ...customer, external_id })}
              />
              <Input
                label="Email"
                size="small"
                type="email"
                placeholder="ada@example.com"
                value={customer.email}
                onChange={(email) => setCustomer({ ...customer, email })}
              />
              {attributes.map((key) => (
                <Input
                  key={key}
                  label={humanize(key)}
                  size="small"
                  inputDir="ltr"
                  value={customer[`attr.${key}`] ?? ""}
                  onChange={(value) => setCustomer({ ...customer, [`attr.${key}`]: value })}
                />
              ))}
            </div>
          </Section>
          <Section title="What the agent fills in">
            <div className="flex flex-col gap-3 py-3 pe-3">
              {draft.fields.length === 0 ? (
                <span className="text-body-2-regular text-text-tertiary">This action takes no inputs.</span>
              ) : null}
              {draft.fields.map((field) => (
                <FieldInput
                  key={field.key}
                  field={field}
                  value={values[field.key] ?? ""}
                  onChange={(value) => setValues({ ...values, [field.key]: value })}
                />
              ))}
            </div>
          </Section>
          <div className="flex flex-col gap-3">
            {live && write ? (
              <Checkbox size="sm" isSelected={sure} onChange={setSure}>
                This changes real data. Run it anyway.
              </Checkbox>
            ) : null}
            <Button leadingIcon={RiPlayLine} loading={running} disabled={blocked} onClick={start}>
              {live ? "Run live" : "Run dry run"}
            </Button>
            <span className="text-body-2-regular text-text-tertiary">
              {live ? "Sends the request to your API." : "Builds the request without sending it."}
            </span>
          </div>
          {result ? (
            <Section title="Result">
              <Result result={result} write={write} />
            </Section>
          ) : null}
        </div>
      </ScrollArea>
    </Panel>
  );
}
