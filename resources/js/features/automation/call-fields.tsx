/*
 * The settings of a step that calls your own system: the connection it
 * goes through, the method and address, headers and a body, how long it
 * may take, what to keep from the answer, and whether a failed call takes
 * a path of its own. Credentials only ever come from a connection, kept
 * encrypted on the server, so no header here carries them.
 */
import { Action, Danger, Field, Toggle } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import { choicesOf } from "./choices";
import { type EditorProps, Note, Words } from "./field-parts";
import { Pick } from "./pick";
import { SaveTarget } from "./save-target";
import { text, useSet, withFailurePath, withSettings } from "./step-settings";
import type { Step } from "./workflows";

interface Header {
  name: string;
  value: string;
}

interface Kept {
  path: string;
  field: string;
}

const noConnection = "none";
const most = 10;
const methods = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;
const withBody = ["POST", "PUT", "PATCH"];
const timeouts = { "2s": "2 seconds", "5s": "5 seconds", "8s": "8 seconds", "10s": "10 seconds" };

function rows<T>(step: Step, name: string): T[] {
  const value = step.settings?.[name];

  return Array.isArray(value) ? (value as T[]) : [];
}

function Headers({ step, onEdit }: EditorProps) {
  const headers = rows<Header>(step, "headers");
  const save = (next: Header[]) =>
    onEdit((current) => withSettings(current, { headers: next.length > 0 ? next : undefined }));

  return (
    <Section title="Headers" description="Credentials come from the connection, never from a header.">
      {headers.map((header, index) => (
        <Row
          key={`${String(index)}-${String(headers.length)}`}
          label={
            <Field
              label={`Header ${index + 1} name`}
              placeholder="X-Customer-Id"
              value={header.name}
              width="w-32"
              onChange={(name) => save(headers.map((old, at) => (at === index ? { ...old, name } : old)))}
            />
          }
        >
          <Field
            label={`Header ${index + 1} value`}
            placeholder="{{contact.id}}"
            value={header.value}
            width="w-32"
            onChange={(value) => save(headers.map((old, at) => (at === index ? { ...old, value } : old)))}
          />
          <Danger onPress={() => save(headers.filter((_, at) => at !== index))}>Remove</Danger>
        </Row>
      ))}
      {headers.some((header) => header.name.trim().toLowerCase() === "authorization") ? (
        <Note>An Authorization header is refused. Put the credentials on a connection.</Note>
      ) : null}
      {headers.length < most ? (
        <Row label="Another header">
          <Action onPress={() => save([...headers, { name: "", value: "" }])}>Add header</Action>
        </Row>
      ) : null}
    </Section>
  );
}

function Keep({ step, onEdit }: EditorProps) {
  const kept = rows<Kept>(step, "save");
  const save = (next: Kept[]) =>
    onEdit((current) => withSettings(current, { save: next.length > 0 ? next : undefined }));

  return (
    <Section title="Save from the response" description="A dotted path into the JSON, such as data.plan or items.0.id.">
      {kept.map((item, index) => (
        <Row
          key={`${String(index)}-${String(kept.length)}`}
          label={
            <Field
              label={`Response path ${index + 1}`}
              placeholder="data.plan"
              value={item.path}
              width="w-32"
              onChange={(path) => save(kept.map((old, at) => (at === index ? { ...old, path } : old)))}
            />
          }
        >
          <SaveTarget
            label={`Save path ${index + 1} as`}
            value={item.field}
            onChange={(field) => save(kept.map((old, at) => (at === index ? { ...old, field: field ?? "" } : old)))}
          />
          <Danger onPress={() => save(kept.filter((_, at) => at !== index))}>Remove</Danger>
        </Row>
      ))}
      {kept.length < most ? (
        <Row label="Another value">
          <Action onPress={() => save([...kept, { path: "", field: "" }])}>Add value</Action>
        </Row>
      ) : null}
    </Section>
  );
}

export function CallFields(props: EditorProps) {
  const { step, lists, onEdit } = props;
  const set = useSet(onEdit);
  const connection = text(step.settings, "connection_id");
  const method = text(step.settings, "method") || "GET";
  const failing = (step.paths ?? []).some((path) => path.outcome === "failure");

  return (
    <>
      <Section title="Request">
        <Note>
          Credentials only come from a connection, stored encrypted. Add connections in Agent settings, Actions.
        </Note>
        <Row label="Connection" description={connection ? "The address is added to its base URL." : undefined}>
          <Pick
            key={`connections-${lists.connections.length}`}
            label="Connection"
            choices={[[noConnection, "No connection (public URL)"], ...lists.connections]}
            value={connection || noConnection}
            onChange={(id) => set({ connection_id: id === noConnection ? undefined : id })}
          />
        </Row>
        <Row label="Method">
          <Pick
            label="Method"
            choices={methods.map((name) => [name, name] as const)}
            value={method}
            onChange={(value) => set(withBody.includes(value) ? { method: value } : { method: value, body: undefined })}
          />
        </Row>
        <Row label={connection ? "Path" : "URL"}>
          <Field
            key={connection ? "path" : "url"}
            label="Request URL"
            placeholder={connection ? "/v1/customers?email={{contact.email}}" : "https://api.example.com/v1/…"}
            value={text(step.settings, "url")}
            onChange={(url) => set({ url })}
          />
        </Row>
        <Row label="Give up after">
          <Pick
            label="Give up after"
            choices={choicesOf(timeouts)}
            value={text(step.settings, "timeout") || "8s"}
            onChange={(value) => set({ timeout: value === "8s" ? undefined : value })}
          />
        </Row>
      </Section>
      {withBody.includes(method) ? (
        <Section title="Body" description="JSON. Use {{contact.email}} and other variables to fill in details.">
          <Words
            label="Request body"
            value={text(step.settings, "body")}
            placeholder={'{"email": "{{contact.email}}"}'}
            onChange={(value) => set({ body: value.trim() ? value : undefined })}
          />
        </Section>
      ) : null}
      <Headers {...props} />
      <Keep {...props} />
      <Section title="When the call fails">
        <Row label="Take a separate path when the call fails">
          <Toggle
            label="Take a separate path when the call fails"
            on={failing}
            onChange={(on) => onEdit((current) => withFailurePath(current, on))}
          />
        </Row>
      </Section>
    </>
  );
}
