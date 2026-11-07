/*
 * How a REST action calls the API: the method and path under the
 * connection's base URL, query parameters, extra headers and, for
 * anything but GET, a body. Any of them can hold placeholders that dimmi
 * fills in at call time: what the agent filled in ({input.order_id}),
 * the signed-in customer's verified details ({contact.external_id}) or a
 * connection variable. The placeholders this action can use are listed
 * under the request, and clicking one copies it. The secret is added by
 * dimmi after the request is built and never appears here.
 */
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Section } from "@/features/settings/layout";

import { type ActionConnection, HTTP_METHODS, type HttpMethod } from "../api";
import { endpointBase } from "../format";
import { Block, PairRows, Segments } from "../parts";
import { contactPlaceholders } from "../schema";
import type { Editor } from "./use-editor";

/*
 * Placeholders lists what the action's templates can use, each copied
 * on a click.
 */
export function Placeholders({ inputs, variables }: { inputs: string[]; variables: string[] }) {
  const tokens = [
    ...inputs.filter(Boolean).map((name) => ({ token: `input.${name}`, label: "Filled in by the agent" })),
    ...contactPlaceholders,
    { token: "contact.attr.plan", label: "A trusted customer attribute" },
    ...variables.map((name) => ({ token: `connection.${name}`, label: "Connection variable" })),
  ];
  const copy = (token: string) => {
    void navigator.clipboard?.writeText(`{${token}}`).catch(() => undefined);
    toast.success("Copied", { description: `{${token}}` });
  };

  return (
    <div className="flex flex-col gap-2 py-3 pe-3">
      <span className="text-body-2-medium text-text-secondary">Placeholders</span>
      <ul className="flex flex-wrap gap-1.5">
        {tokens.map((item) => (
          <li key={item.token}>
            <button
              type="button"
              title={item.label}
              onClick={() => copy(item.token)}
              className="cursor-pointer rounded-md bg-background-tertiary-default px-1.5 py-0.5 font-mono text-caption-1-medium text-text-secondary outline-none transition-colors duration-150 hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              dir="ltr"
            >
              {`{${item.token}}`}
            </button>
          </li>
        ))}
      </ul>
      <span className="text-body-2-regular text-text-tertiary">
        Customer details resolve only when the customer signed in with a verified identity.
      </span>
    </div>
  );
}

export function RequestSection({ editor, connection }: { editor: Editor; connection: ActionConnection }) {
  const { draft, update, problems } = editor;
  const hasBody = draft.method !== "GET";
  const base = endpointBase(connection.base_url);

  return (
    <Section title="Request" description="How dimmi calls your API. The secret is added after, never shown here.">
      <Block label="Endpoint" problem={problems.path}>
        <div className="flex flex-col gap-2 sm:flex-row">
          <Select
            aria-label="Method"
            selectedKey={draft.method}
            onSelectionChange={(key) => update({ method: String(key) as HttpMethod })}
            className="sm:w-32"
          >
            {HTTP_METHODS.map((method) => (
              <SelectItem key={method} id={method} textValue={method}>
                {method}
              </SelectItem>
            ))}
          </Select>
          <Input
            aria-label="Path"
            inputDir="ltr"
            value={draft.path}
            onChange={(path) => update({ path })}
            placeholder="/customers/{contact.external_id}/invoices"
            isInvalid={Boolean(problems.path)}
            leadingAddon={
              <span dir="ltr" className="max-w-40 shrink-0 truncate ps-1 text-body-regular text-text-tertiary">
                {base}
              </span>
            }
            className="min-w-0 flex-1"
            fieldClassName="font-mono"
          />
        </div>
        {problems.identity ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {problems.identity}
          </p>
        ) : null}
      </Block>
      <Block label="Query parameters">
        <PairRows
          label="Query parameter"
          rows={draft.query}
          onChange={(query) => update({ query })}
          namePlaceholder="status"
          valuePlaceholder="{input.status}"
          addLabel="Add parameter"
        />
      </Block>
      <Block label="Headers" hint="Sent with this action only, on top of the connection's default headers.">
        <PairRows
          label="Header"
          rows={draft.headers}
          onChange={(headers) => update({ headers })}
          namePlaceholder="X-Reason"
          valuePlaceholder="{input.reason}"
          addLabel="Add header"
        />
      </Block>
      {hasBody ? (
        <Block label="Body" hint="A placeholder alone in a value keeps its type: numbers stay numbers.">
          <Segments
            label="Body encoding"
            value={draft.body_encoding}
            options={[
              { id: "json", label: "JSON" },
              { id: "form", label: "Form" },
            ]}
            onChange={(body_encoding) => update({ body_encoding })}
          />
          <Textarea
            aria-label="Body"
            rows={5}
            autoResize
            maxRows={16}
            value={draft.body}
            onChange={(body) => update({ body })}
            placeholder={
              '{\n  "customer": "{contact.external_id}",\n  "invoice_id": "{input.invoice_id}",\n  "amount": "{input.amount}"\n}'
            }
            fieldClassName="font-mono"
          />
        </Block>
      ) : null}
      <Placeholders
        inputs={draft.fields.map((field) => field.name)}
        variables={Object.keys(connection.variables ?? {})}
      />
    </Section>
  );
}
