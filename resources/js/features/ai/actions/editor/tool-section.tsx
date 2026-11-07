/*
 * Which MCP tool an action runs, and which of its arguments dimmi fills
 * from the signed-in customer instead of the agent. A bound argument is
 * taken out of what the agent sees, so a customer cannot talk the agent
 * into acting on someone else's account. An input whose name looks like
 * it identifies the customer (customer_id, email) is pointed out with a
 * one-click way to bind it to their verified identity.
 */
import { RiAddLine, RiCloseLine, RiShieldUserLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Section } from "@/features/settings/layout";

import { type Draft, type Pair, rowKey } from "../draft";
import { Block } from "../parts";
import { contactPlaceholders, identityLike } from "../schema";
import type { Editor } from "./use-editor";

const attribute = "attribute";

/*
 * sourceOf reads which customer detail a bound value names.
 */
function sourceOf(value: string): { source: string; key: string } {
  const attr = /^\{contact\.(?:attr|custom_attributes)\.([a-z0-9_]+)\}$/.exec(value);
  if (attr) {
    return { source: attribute, key: attr[1] ?? "" };
  }

  return { source: value.replace(/^\{|\}$/g, ""), key: "" };
}

function BindingRow({
  row,
  index,
  onChange,
  onRemove,
}: {
  row: Pair;
  index: number;
  onChange: (change: Partial<Pair>) => void;
  onRemove: () => void;
}) {
  const { source, key } = sourceOf(row.value);

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
      <Input
        aria-label={`Argument ${index + 1}`}
        size="small"
        inputDir="ltr"
        placeholder="customer_id"
        value={row.name}
        onChange={(name) => onChange({ name })}
        className="min-w-0 sm:w-2/5"
        fieldClassName="font-mono"
      />
      <div className="flex min-w-0 flex-1 gap-2">
        <Select
          aria-label={`Customer detail for argument ${index + 1}`}
          size="sm"
          selectedKey={source}
          onSelectionChange={(picked) =>
            onChange({ value: picked === attribute ? "{contact.attr.}" : `{${String(picked)}}` })
          }
          className="min-w-0 flex-1"
        >
          {[...contactPlaceholders, { token: attribute, label: "A trusted attribute…" }].map((item) => (
            <SelectItem key={item.token} id={item.token} textValue={item.label}>
              {item.label}
            </SelectItem>
          ))}
        </Select>
        {source === attribute ? (
          <Input
            aria-label={`Attribute for argument ${index + 1}`}
            size="small"
            inputDir="ltr"
            placeholder="stripe_customer_id"
            value={key}
            onChange={(next) => onChange({ value: `{contact.attr.${next}}` })}
            className="w-36"
          />
        ) : null}
        <Button
          variant="ghost"
          size="small"
          leadingIcon={RiCloseLine}
          aria-label={`Stop binding argument ${index + 1}`}
          onClick={onRemove}
        />
      </div>
    </div>
  );
}

/*
 * bind moves an input the agent filled in to the customer's verified
 * identity.
 */
function bind(draft: Draft, name: string): Draft {
  const token = name.includes("email") ? "{contact.email}" : "{contact.external_id}";

  return {
    ...draft,
    fields: draft.fields.filter((field) => field.name !== name),
    bound: [...draft.bound, { key: rowKey(), name, value: token }],
  };
}

export function ToolSection({ editor }: { editor: Editor }) {
  const { draft, update, problems } = editor;
  const suspects = draft.fields.filter((field) => identityLike(field.name));
  const setRow = (key: string, change: Partial<Pair>) =>
    update({ bound: draft.bound.map((row) => (row.key === key ? { ...row, ...change } : row)) });

  return (
    <Section title="Tool" description="The MCP tool this action runs on the server.">
      <Block label="Tool name" hint="Its name on the server. Synced tools fill this in." problem={problems.tool}>
        <Input
          aria-label="Tool name on the server"
          inputDir="ltr"
          placeholder="issue_refund"
          value={draft.tool}
          onChange={(tool) => update({ tool })}
          isInvalid={Boolean(problems.tool)}
          fieldClassName="font-mono"
        />
      </Block>
      <Block
        label="Filled from the signed-in customer"
        hint="dimmi sets these arguments from the customer's verified identity. The agent never sees or chooses them."
        problem={problems.identity}
      >
        {suspects.map((field) => (
          <div
            key={field.key}
            className="flex flex-col gap-2 rounded-xl bg-status-yellow-background p-3 text-status-yellow-text sm:flex-row sm:items-center"
          >
            <RiShieldUserLine aria-hidden className="size-4 shrink-0" />
            <span className="flex-1 text-body-2-regular">
              <code className="font-mono">{field.name}</code> looks like it names the customer. Bind it so the agent
              cannot pick someone else.
            </span>
            <Button variant="secondary" size="xs" onClick={() => update((before) => bind(before, field.name))}>
              Bind to the customer
            </Button>
          </div>
        ))}
        {draft.bound.map((row, index) => (
          <BindingRow
            key={row.key}
            row={row}
            index={index}
            onChange={(change) => setRow(row.key, change)}
            onRemove={() => update({ bound: draft.bound.filter((other) => other.key !== row.key) })}
          />
        ))}
        <Button
          variant="secondary"
          size="xs"
          leadingIcon={RiAddLine}
          className="self-start"
          onClick={() =>
            update({ bound: [...draft.bound, { key: rowKey(), name: "", value: "{contact.external_id}" }] })
          }
        >
          Bind an argument
        </Button>
      </Block>
    </Section>
  );
}
