/*
 * AI attributes: what the workspace wants known about every
 * conversation, such as the customer's intent or an order number. Each
 * has a key, a name, a type and a description the AI reads; a choice
 * also lists its values. The AI fills them in on every conversation and
 * checks them again as the customer writes. Workflows branch and start
 * on them, and inbox views filter by them.
 *
 * A key and a type are fixed once saved. Turning detection off archives
 * an attribute, which keeps the values it found. Every change, new
 * attributes included, waits in a draft until Save in the bar at the
 * bottom is pressed; Discard puts everything back, and leaving with
 * changes not saved asks first.
 */
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { Pick } from "@/features/automation/pick";
import { keyFor as keyOf } from "@/features/automation/step-values";
import { usePermission } from "@/features/settings/inboxes";
import {
  type AIAttribute,
  type AIAttributeChange,
  type AIAttributeInput,
  type AIAttributeType,
  aiAttributesQuery,
  aiAttributeTypes,
  createAIAttribute,
  updateAIAttribute,
} from "@/lib/api/ai-attributes";

import { Danger, Field, Toggle } from "../controls";
import { Empty, Row, Section, SettingsPage } from "../layout";
import { OptionsEditor } from "../options-editor";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

/*
 * Added is a new attribute in the draft, under an id of its own until it
 * is saved.
 */
interface Added extends AIAttributeInput {
  draft: string;
}

const typeChoices = Object.entries(aiAttributeTypes) as [AIAttributeType, string][];

/*
 * useDraft holds the changes not saved yet: edits by attribute id and
 * the attributes added. Save sends them one by one, then reads the list
 * again; Discard drops them and puts the controls back.
 */
function useDraft(workspace: string) {
  const client = useQueryClient();
  const [edits, setEdits] = useState<Record<string, AIAttributeChange>>({});
  const [added, setAdded] = useState<Added[]>([]);
  const [version, setVersion] = useState(0);
  const save = useMutation({
    mutationFn: async () => {
      for (const item of added) {
        const { draft: _, ...body } = item;
        await createAIAttribute(workspace, { ...body, key: body.key || keyOf(body.label) });
      }
      for (const [id, change] of Object.entries(edits)) {
        await updateAIAttribute(workspace, id, change);
      }
    },
    onSuccess: async () => {
      setEdits({});
      setAdded([]);
      await client.invalidateQueries({ queryKey: aiAttributesQuery(workspace).queryKey });
      toast.success("AI attributes saved");
    },
  });

  return {
    edits,
    added,
    version,
    save,
    dirty: Object.keys(edits).length > 0 || added.length > 0,
    edit: (attribute: AIAttribute, change: AIAttributeChange) =>
      setEdits((all) => ({ ...all, [attribute.id]: { ...all[attribute.id], ...change } })),
    add: () =>
      setAdded((all) => [...all, { draft: crypto.randomUUID(), key: "", label: "", data_type: "text", options: [] }]),
    change: (draft: string, change: Partial<AIAttributeInput>) =>
      setAdded((all) => all.map((item) => (item.draft === draft ? { ...item, ...change } : item))),
    drop: (draft: string) => setAdded((all) => all.filter((item) => item.draft !== draft)),
    discard: () => {
      save.reset();
      setEdits({});
      setAdded([]);
      setVersion((v) => v + 1);
    },
  };
}

type Draft = ReturnType<typeof useDraft>;

function Words({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="py-2.5 pe-2.5">
      <Textarea
        aria-label={label}
        defaultValue={value}
        placeholder="What it means, so the AI can tell. The customer's main reason for writing, for example."
        isDisabled={disabled}
        onChange={onChange}
        rows={2}
        autoResize
        maxRows={6}
      />
    </div>
  );
}

function Saved({ attribute, draft, canManage }: { attribute: AIAttribute; draft: Draft; canManage: boolean }) {
  const shown = { ...attribute, ...draft.edits[attribute.id] };
  const active = shown.status === "active";

  return (
    <Section title={shown.label || attribute.key} description={`conversation.ai.${attribute.key}`}>
      <Row label="Name">
        <Field
          label={`${attribute.key} name`}
          value={attribute.label}
          width="w-48"
          disabled={!canManage}
          onChange={(label) => draft.edit(attribute, { label })}
        />
      </Row>
      <Row label="Type" description="Fixed once saved.">
        <span className="text-body-regular text-text-secondary">{aiAttributeTypes[attribute.data_type]}</span>
      </Row>
      <Words
        label={`${attribute.key} description`}
        value={attribute.description}
        disabled={!canManage}
        onChange={(description) => draft.edit(attribute, { description })}
      />
      {attribute.data_type === "choice" ? (
        <div className="py-2.5 pe-2.5">
          <OptionsEditor
            label={`${attribute.key} values`}
            value={attribute.options}
            isDisabled={!canManage}
            onChange={(options) => draft.edit(attribute, { options: options.filter((o) => o.trim() !== "") })}
          />
        </div>
      ) : null}
      <Row label="Detect on conversations" description={active ? undefined : "Archived. Its values are kept."}>
        <Toggle
          label={`Detect ${attribute.key}`}
          on={attribute.status === "active"}
          disabled={!canManage}
          onChange={(on) => draft.edit(attribute, { status: on ? "active" : "archived" })}
        />
      </Row>
    </Section>
  );
}

function New({ item, draft }: { item: Added; draft: Draft }) {
  return (
    <Section
      title={item.label || "New attribute"}
      description={`conversation.ai.${item.key || keyOf(item.label) || "key"}`}
      action={<Danger onPress={() => draft.drop(item.draft)}>Remove</Danger>}
    >
      <Row label="Name">
        <Field
          label="New attribute name"
          value={item.label}
          placeholder="Intent"
          width="w-48"
          onChange={(label) => draft.change(item.draft, { label })}
        />
      </Row>
      <Row label="Type">
        <Pick
          label="New attribute type"
          choices={typeChoices}
          value={item.data_type}
          onChange={(type) => draft.change(item.draft, { data_type: type as AIAttributeType, options: [] })}
        />
      </Row>
      <Words
        label="New attribute description"
        value={item.description ?? ""}
        disabled={false}
        onChange={(description) => draft.change(item.draft, { description })}
      />
      {item.data_type === "choice" ? (
        <div className="py-2.5 pe-2.5">
          <OptionsEditor
            label="New attribute values"
            value={item.options ?? []}
            onChange={(options) => draft.change(item.draft, { options: options.filter((o) => o.trim() !== "") })}
          />
        </div>
      ) : null}
    </Section>
  );
}

export function AIAttributesSettings() {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(aiAttributesQuery(workspace));
  const canManage = usePermission(workspace, "ai_agent:manage");
  const draft = useDraft(workspace);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="AI attributes"
      description="What the AI notes on every conversation, for workflows, views and your team."
      actions={
        canManage ? (
          <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={draft.add}>
            New attribute
          </Button>
        ) : undefined
      }
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? failureText(draft.save.error) : null}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      {data ? (
        <div key={draft.version} className="flex flex-col gap-8">
          {data.length === 0 && draft.added.length === 0 ? (
            <Section title="Attributes">
              <Empty>None yet. Add one, such as the customer's intent or their order number.</Empty>
            </Section>
          ) : null}
          {draft.added.map((item) => (
            <New key={item.draft} item={item} draft={draft} />
          ))}
          {data.map((attribute) => (
            <Saved key={attribute.id} attribute={attribute} draft={draft} canManage={canManage} />
          ))}
        </div>
      ) : (
        <Section title="Attributes" loading rows={3} />
      )}
    </SettingsPage>
  );
}
