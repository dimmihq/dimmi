/*
 * One conversation attribute, laid out flat: its name and key, its type
 * (fixed once saved), its options and what each means, the description
 * the AI reads, then one card for who fills it, whether teammates must
 * fill it before closing and the teams that see it, each picked from a
 * dropdown whose choices say what they mean, and a danger zone that archives or
 * restores it, or deletes it while nothing uses it. "new" makes one.
 * Every change waits for Save in the bar at the bottom; Discard puts it
 * back, and leaving with changes not saved asks first.
 */
import { RiArrowLeftLine, RiPriceTag3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { type ComponentType, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { MultiPick } from "@/features/automation/pick";
import { keyFor } from "@/features/automation/step-values";
import { usePermission } from "@/features/settings/inboxes";
import { EmptyState, Panel } from "@/features/shell/page";
import {
  aiFillable,
  type ConversationAttribute,
  type ConversationAttributeChange,
  type ConversationAttributeType,
  conversationAttributesQuery,
  conversationAttributeTypes,
  conversationAttributeUsageQuery,
  createConversationAttribute,
  deleteConversationAttribute,
  type FilledBy,
  filledByLabels,
  updateConversationAttribute,
} from "@/lib/api/conversation-attributes";
import { teamsQuery } from "@/lib/api/teams";

import { Danger } from "../controls";
import { PageSkeleton, Row, Section, SettingsPage } from "../layout";
import { optionList } from "../options";
import { OptionsEditor } from "../options-editor";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { OptionDescriptions } from "./conversation-attribute-parts";
import { choosable, describedOf, typeHints, typeIcons } from "./conversation-attribute-types";

/*
 * Draft is everything the page edits.
 */
interface Draft {
  label: string;
  key: string;
  dataType: ConversationAttributeType;
  options: string[];
  descriptions: Record<string, string>;
  description: string;
  filledBy: FilledBy;
  required: boolean;
  teams: string[];
}

const blank: Draft = {
  label: "",
  key: "",
  dataType: "choice",
  options: [],
  descriptions: {},
  description: "",
  filledBy: "both",
  required: false,
  teams: [],
};

function draftOf(a: ConversationAttribute): Draft {
  return {
    label: a.label,
    key: a.key,
    dataType: a.data_type,
    options: a.options,
    descriptions: a.option_descriptions ?? {},
    description: a.description,
    filledBy: a.filled_by,
    required: a.required_to_close,
    teams: a.team_ids,
  };
}

/*
 * bodyOf is what saving sends: the options and descriptions only for a
 * list, who fills it as the type allows, and no teams for every team.
 */
function bodyOf(d: Draft): ConversationAttributeChange {
  const options = choosable.includes(d.dataType) ? optionList(d.options) : [];

  return {
    label: d.label.trim(),
    description: d.description.trim(),
    options,
    option_descriptions: describedOf(options, d.descriptions),
    filled_by: aiFillable(d.dataType) ? d.filledBy : "teammates",
    required_to_close: d.required,
    team_ids: d.teams,
  };
}

/*
 * problemOf says why the draft cannot be saved yet, null when it can.
 */
function problemOf(d: Draft, key: string): string | null {
  if (d.label.trim() === "") {
    return "Give it a name.";
  }
  if (key === "") {
    return "Give it a key.";
  }
  if (choosable.includes(d.dataType) && optionList(d.options).length === 0) {
    return "Add at least one option.";
  }

  return null;
}

function same(a: Draft, b: Draft): boolean {
  return JSON.stringify(bodyOf(a)) === JSON.stringify(bodyOf(b)) && a.key === b.key && a.dataType === b.dataType;
}

function useAttributeDraft(workspace: string, attribute: ConversationAttribute | null) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const start = attribute ? draftOf(attribute) : blank;
  const [saved, setSaved] = useState(start);
  const [draft, setDraft] = useState(start);
  const [keyTouched, setKeyTouched] = useState(attribute !== null);
  const leaving = useRef(false);
  const key = keyTouched ? draft.key : keyFor(draft.label);
  const dirty = attribute === null || !same(saved, draft);
  const save = useMutation({
    mutationFn: () =>
      attribute
        ? updateConversationAttribute(workspace, attribute.id, bodyOf(draft))
        : createConversationAttribute(workspace, {
            ...bodyOf(draft),
            label: draft.label.trim(),
            key,
            data_type: draft.dataType,
          }),
    onSuccess: async (next) => {
      const fresh = draftOf(next);
      setSaved(fresh);
      setDraft(fresh);
      await client.invalidateQueries({ queryKey: conversationAttributesQuery(workspace).queryKey });
      toast.success(attribute ? "Attribute saved" : "Attribute created");
      if (!attribute) {
        leaving.current = true;
        await navigate({
          to: "/d/settings/conversation-attributes/$attribute",
          params: { attribute: next.id },
          replace: true,
        });
      }
    },
  });

  return {
    draft,
    key,
    dirty,
    save,
    leaving,
    problem: problemOf(draft, key),
    set: (change: Partial<Draft>) => setDraft((was) => ({ ...was, ...change })),
    setKey: (next: string) => {
      setKeyTouched(true);
      setDraft((was) => ({ ...was, key: next }));
    },
    discard: () => setDraft(saved),
  };
}

type Edit = ReturnType<typeof useAttributeDraft>;

function BackToList() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "conversation-attributes" } })}
      >
        All conversation attributes
      </Button>
    </div>
  );
}

const fillHints: Record<FilledBy, string> = {
  both: "The AI fills it as the customer writes. A teammate's value always wins.",
  teammates: "The AI leaves it alone.",
  ai: "Teammates see the value but can't change it.",
};

/*
 * Option is one choice of a dropdown: its name, with an optional mark,
 * and a line saying what it means.
 */
function Option({
  icon: Icon,
  name,
  hint,
}: {
  icon?: ComponentType<{ className?: string }>;
  name: string;
  hint: string;
}) {
  return (
    <span className="flex min-w-0 items-start gap-2">
      {Icon ? <Icon aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" /> : null}
      <span className="flex min-w-0 flex-col">
        <span className="text-body-2-medium text-text-primary">{name}</span>
        <span className="text-caption-1-regular text-text-tertiary">{hint}</span>
      </span>
    </span>
  );
}

function TypeRow({ edit, saved }: { edit: Edit; saved: boolean }) {
  const type = edit.draft.dataType;
  const Icon = typeIcons[type];

  return (
    <Row label="Type" description={saved ? `${typeHints[type]} Fixed once saved.` : typeHints[type]}>
      <Select
        aria-label="Type"
        size="sm"
        className="w-56"
        popoverClassName="w-80"
        isDisabled={saved}
        selectedKey={type}
        onSelectionChange={(next) => edit.set({ dataType: next as ConversationAttributeType })}
        renderValue={
          <span className="flex items-center gap-2">
            <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
            {conversationAttributeTypes[type]}
          </span>
        }
      >
        {(Object.keys(conversationAttributeTypes) as ConversationAttributeType[]).map((t) => (
          <SelectItem key={t} id={t} textValue={conversationAttributeTypes[t]}>
            <Option icon={typeIcons[t]} name={conversationAttributeTypes[t]} hint={typeHints[t]} />
          </SelectItem>
        ))}
      </Select>
    </Row>
  );
}

function BasicsSection({ edit, saved, locked }: { edit: Edit; saved: boolean; locked: boolean }) {
  return (
    <Section title="Basics">
      <Row label="Name">
        <Input
          aria-label="Name"
          size="small"
          className="w-56"
          value={edit.draft.label}
          placeholder="Issue type"
          maxLength={80}
          isDisabled={locked}
          onChange={(label) => edit.set({ label })}
        />
      </Row>
      <Row
        label="Key"
        description={
          saved
            ? `Workflows and views read it as conversation.ai.${edit.key}. Fixed once saved.`
            : `Workflows and views read it as conversation.ai.${edit.key || "key"}.`
        }
      >
        <Input
          aria-label="Key"
          size="small"
          className="w-56"
          value={edit.key}
          maxLength={64}
          isDisabled={saved || locked}
          onChange={edit.setKey}
        />
      </Row>
      <TypeRow edit={edit} saved={saved || locked} />
    </Section>
  );
}

function OptionsSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  const options = optionList(edit.draft.options);

  return (
    <Section title="Options" bare>
      <div className="flex flex-col gap-5 rounded-2xl bg-background-secondary-default p-4">
        <OptionsEditor
          label="The values it may take"
          value={edit.draft.options}
          isDisabled={locked}
          onChange={(next) => edit.set({ options: next })}
        />
        {options.length > 0 ? (
          <OptionDescriptions
            options={options}
            value={edit.draft.descriptions}
            onChange={(descriptions) => edit.set({ descriptions })}
          />
        ) : null}
      </div>
    </Section>
  );
}

function DescriptionSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  return (
    <Section title="Description" description="The AI reads this to decide the value; teammates see it as a hint." bare>
      <Textarea
        aria-label="Description"
        value={edit.draft.description}
        onChange={(description) => edit.set({ description })}
        placeholder="The main reason the customer wrote in."
        rows={3}
        maxLength={500}
        isDisabled={locked}
      />
    </Section>
  );
}

function FillRow({ edit, locked }: { edit: Edit; locked: boolean }) {
  const canAI = aiFillable(edit.draft.dataType);
  const value: FilledBy = canAI ? edit.draft.filledBy : "teammates";
  const order: FilledBy[] = ["both", "teammates", "ai"];

  return (
    <Row
      label="Who fills it"
      description={
        canAI
          ? fillHints[value]
          : `The AI can't fill ${conversationAttributeTypes[edit.draft.dataType].toLowerCase()} values.`
      }
    >
      <Select
        aria-label="Who fills it"
        size="sm"
        className="w-56"
        popoverClassName="w-80"
        isDisabled={locked}
        selectedKey={value}
        disabledKeys={canAI ? [] : ["both", "ai"]}
        onSelectionChange={(next) => edit.set({ filledBy: next as FilledBy })}
        renderValue={filledByLabels[value]}
      >
        {order.map((f) => (
          <SelectItem key={f} id={f} textValue={filledByLabels[f]}>
            <Option name={filledByLabels[f]} hint={fillHints[f]} />
          </SelectItem>
        ))}
      </Select>
    </Row>
  );
}

function TeamsRow({ edit, locked }: { edit: Edit; locked: boolean }) {
  const workspace = useAccount().workspace.id;
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const picked = edit.draft.teams;

  return (
    <Row
      label="Visible to"
      description={
        picked.length === 0 ? "Every team sees and fills it." : "Only these teams see it, in the inbox and the API."
      }
    >
      {locked ? (
        <span className="text-body-2-regular text-text-secondary">
          {picked.length === 0 ? "Every team" : `${picked.length} teams`}
        </span>
      ) : (
        <MultiPick
          label="Teams"
          choices={teams.map((t) => [t.id, t.name] as const)}
          value={picked}
          allLabel="Every team"
          empty="No teams yet"
          className="w-56"
          onChange={(next) => edit.set({ teams: next })}
        />
      )}
    </Row>
  );
}

function BehaviourSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  return (
    <Section title="Filling and visibility">
      <FillRow edit={edit} locked={locked} />
      <Row
        label="Required before closing"
        description="Teammates can't close a conversation in dimmi while it's empty."
      >
        <Switch
          aria-label="Required before closing"
          size="sm"
          isSelected={edit.draft.required}
          isDisabled={locked}
          onChange={(required) => edit.set({ required })}
        />
      </Row>
      <TeamsRow edit={edit} locked={locked} />
    </Section>
  );
}

/*
 * usageLine says what uses an attribute, or that nothing does.
 */
function usageLine(conversations: number, workflows: string[], views: string[]): string {
  const parts: string[] = [];
  if (conversations > 0) {
    parts.push(conversations === 1 ? "1 conversation holds a value" : `${conversations} conversations hold a value`);
  }
  if (workflows.length > 0) {
    parts.push(`${workflows.length === 1 ? "the workflow" : "the workflows"} ${workflows.join(", ")} use it`);
  }
  if (views.length > 0) {
    parts.push(`${views.length === 1 ? "the view" : "the views"} ${views.join(", ")} filter by it`);
  }
  if (parts.length === 0) {
    return "Nothing uses it.";
  }
  const sentence = parts.join("; ");

  return `${sentence.charAt(0).toUpperCase()}${sentence.slice(1)}. Archive it instead.`;
}

/*
 * Confirm asks before archiving or deleting an attribute.
 */
function Confirm({
  attribute,
  kind,
  pending,
  error,
  onClose,
  onConfirm,
}: {
  attribute: ConversationAttribute;
  kind: "archive" | "delete";
  pending: boolean;
  error: Error | null;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const archiving = kind === "archive";

  return (
    <Modal size="sm" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={archiving ? `Archive ${attribute.label}?` : `Delete ${attribute.label}?`}
        description={
          archiving
            ? "It leaves the details panel and nobody fills it any more. Values stay on conversations, so workflows, views and reports keep reading what it last held."
            : "It's removed for good. Nothing uses it, so no conversation, workflow or view changes."
        }
      />
      <ModalBody>
        {error ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {failureText(error)}
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="danger" loading={pending} onClick={onConfirm}>
          {archiving ? "Archive" : "Delete"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

function DangerSection({ attribute }: { attribute: ConversationAttribute }) {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const navigate = useNavigate();
  const [confirming, setConfirming] = useState<"archive" | "delete" | null>(null);
  const { data: usage } = useQuery(conversationAttributeUsageQuery(workspace, attribute.id));
  const archived = attribute.status === "archived";
  const refresh = () => client.invalidateQueries({ queryKey: conversationAttributesQuery(workspace).queryKey });
  const status = useMutation({
    mutationFn: () =>
      updateConversationAttribute(workspace, attribute.id, { status: archived ? "active" : "archived" }),
    onSuccess: async () => {
      setConfirming(null);
      await refresh();
      toast.success(archived ? "Attribute restored" : "Attribute archived");
    },
  });
  const remove = useMutation({
    mutationFn: () => deleteConversationAttribute(workspace, attribute.id),
    onSuccess: async () => {
      setConfirming(null);
      await refresh();
      toast.success("Attribute deleted");
      await navigate({ to: "/d/settings/$section", params: { section: "conversation-attributes" } });
    },
  });
  const used = usage
    ? usageLine(
        usage.conversations,
        usage.workflows.map((w) => w.name),
        usage.views.map((v) => v.name),
      )
    : "Checking what uses it…";

  return (
    <Section title="Danger zone">
      {archived ? (
        <Row label="Restore" description="Teammates and the AI fill it again.">
          <Button variant="secondary" size="small" loading={status.isPending} onClick={() => status.mutate()}>
            Restore
          </Button>
        </Row>
      ) : (
        <Row label="Archive" description="Nobody fills it any more. Conversations keep their values.">
          <Danger onPress={() => setConfirming("archive")}>Archive</Danger>
        </Row>
      )}
      <Row label="Delete" description={used}>
        <Button
          variant="danger-secondary"
          size="small"
          disabled={!usage?.deletable}
          onClick={() => setConfirming("delete")}
        >
          Delete
        </Button>
      </Row>
      {confirming ? (
        <Confirm
          attribute={attribute}
          kind={confirming}
          pending={confirming === "archive" ? status.isPending : remove.isPending}
          error={confirming === "archive" ? status.error : remove.error}
          onClose={() => setConfirming(null)}
          onConfirm={() => (confirming === "archive" ? status.mutate() : remove.mutate())}
        />
      ) : null}
    </Section>
  );
}

function Editor({ attribute }: { attribute: ConversationAttribute | null }) {
  const workspace = useAccount().workspace.id;
  const edit = useAttributeDraft(workspace, attribute);
  const manages = usePermission(workspace, "attributes:manage");
  const managesAI = usePermission(workspace, "ai_agent:manage");
  const locked = !(manages || managesAI);
  useBlocker({
    shouldBlockFn: () =>
      attribute !== null &&
      edit.dirty &&
      !edit.leaving.current &&
      !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => attribute !== null && edit.dirty && !edit.leaving.current,
  });
  const saved = attribute !== null;

  return (
    <SettingsPage
      title={attribute ? attribute.label : "New conversation attribute"}
      description={
        attribute?.status === "archived"
          ? "Archived. Nobody fills it; conversations keep their values."
          : "Teammates and the AI fill it on every conversation."
      }
      footer={
        <SaveBar
          hidden={locked}
          dirty={edit.dirty}
          saving={edit.save.isPending}
          error={edit.save.error ? failureText(edit.save.error) : null}
          message={edit.problem ?? (saved ? "Unsaved changes" : "New attribute")}
          saveLabel={saved ? "Save" : "Create"}
          canSave={edit.dirty && edit.problem === null}
          onSave={() => edit.save.mutate()}
          onDiscard={saved ? edit.discard : undefined}
        />
      }
    >
      <BackToList />
      <BasicsSection edit={edit} saved={saved} locked={locked} />
      {choosable.includes(edit.draft.dataType) ? <OptionsSection edit={edit} locked={locked} /> : null}
      <DescriptionSection edit={edit} locked={locked} />
      <BehaviourSection edit={edit} locked={locked} />
      {attribute && !locked ? <DangerSection attribute={attribute} /> : null}
    </SettingsPage>
  );
}

export function ConversationAttributePage({ attribute: id }: { attribute: string }) {
  const workspace = useAccount().workspace.id;
  const { data, isPending } = useQuery(conversationAttributesQuery(workspace));

  if (id === "new") {
    return <Editor attribute={null} />;
  }
  if (isPending) {
    return <PageSkeleton />;
  }
  const attribute = data?.find((a) => a.id === id);
  if (!attribute) {
    return (
      <Panel label="Conversation attribute" className="h-full flex-1">
        <EmptyState
          icon={RiPriceTag3Line}
          title="This attribute is gone"
          description="It may have been deleted. Go back to the list to pick another."
        />
      </Panel>
    );
  }

  return <Editor key={attribute.id} attribute={attribute} />;
}
