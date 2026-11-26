/*
 * One conversation attribute, laid out flat: its name and key, its type
 * (fixed once saved), its options and what each means, the description
 * the AI reads, who fills it, whether teammates must fill it before
 * closing, the teams that see it, and a danger zone that archives or
 * restores it, or deletes it while nothing uses it. "new" makes one.
 * Every change waits for Save in the bar at the bottom; Discard puts it
 * back, and leaving with changes not saved asks first.
 */
import { RiArrowLeftLine, RiPriceTag3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
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
import { Empty, PageSkeleton, Row, Section, SettingsPage } from "../layout";
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
  everyTeam: boolean;
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
  everyTeam: true,
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
    everyTeam: a.team_ids.length === 0,
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
    team_ids: d.everyTeam ? [] : d.teams,
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
  if (!d.everyTeam && d.teams.length === 0) {
    return "Pick at least one team, or let every team see it.";
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
    </Section>
  );
}

function TypeSection({ edit, saved }: { edit: Edit; saved: boolean }) {
  const type = edit.draft.dataType;

  if (saved) {
    const Icon = typeIcons[type];
    return (
      <Section title="Type" description="Fixed once saved.">
        <Row label={conversationAttributeTypes[type]} description={typeHints[type]}>
          <Icon aria-hidden className="size-5 text-foreground-icon-secondary" />
        </Row>
      </Section>
    );
  }

  return (
    <Section title="Type" description="What the value is. You can't change it once saved." bare>
      <RadioGroup
        aria-label="Type"
        value={type}
        onChange={(next) => edit.set({ dataType: next as ConversationAttributeType })}
        className="grid grid-cols-1 gap-2 sm:grid-cols-2"
      >
        {(Object.keys(conversationAttributeTypes) as ConversationAttributeType[]).map((t) => (
          <RadioCard
            key={t}
            value={t}
            icon={typeIcons[t]}
            title={conversationAttributeTypes[t]}
            description={typeHints[t]}
          />
        ))}
      </RadioGroup>
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

function FillSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  const canAI = aiFillable(edit.draft.dataType);
  const value: FilledBy = canAI ? edit.draft.filledBy : "teammates";

  return (
    <Section
      title="Who fills it"
      description={
        canAI ? undefined : `The AI can't fill ${conversationAttributeTypes[edit.draft.dataType].toLowerCase()} values.`
      }
      bare
    >
      <RadioGroup
        aria-label="Who fills it"
        value={value}
        isDisabled={locked}
        onChange={(next) => edit.set({ filledBy: next as FilledBy })}
        className="flex flex-col gap-2"
      >
        <RadioCard
          value="both"
          isDisabled={!canAI}
          title={filledByLabels.both}
          description="The AI fills it as the customer writes. A teammate's value always wins."
        />
        <RadioCard value="teammates" title={filledByLabels.teammates} description="The AI leaves it alone." />
        <RadioCard
          value="ai"
          isDisabled={!canAI}
          title={filledByLabels.ai}
          description="Teammates see the value but can't change it."
        />
      </RadioGroup>
    </Section>
  );
}

function ClosingSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  return (
    <Section title="Closing">
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
    </Section>
  );
}

function TeamsSection({ edit, locked }: { edit: Edit; locked: boolean }) {
  const workspace = useAccount().workspace.id;
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const toggle = (id: string, on: boolean) =>
    edit.set({ teams: on ? [...edit.draft.teams, id] : edit.draft.teams.filter((t) => t !== id) });

  return (
    <Section title="Visible to" description="Hidden attributes stay out of the inbox and the API for other teams." bare>
      <RadioGroup
        aria-label="Visible to"
        value={edit.draft.everyTeam ? "every" : "some"}
        isDisabled={locked}
        onChange={(next) => edit.set({ everyTeam: next === "every" })}
        className="flex flex-col gap-2"
      >
        <RadioCard value="every" title="Every team" description="Everyone sees and fills it." />
        <RadioCard value="some" title="Only these teams" description="Only members of the teams you pick." />
      </RadioGroup>
      {edit.draft.everyTeam ? null : (
        <div className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">
          {teams.length === 0 ? <Empty>No teams yet. Add one in Settings → Teams.</Empty> : null}
          {teams.map((team) => (
            <Row
              key={team.id}
              label={team.name}
              description={team.member_count === 1 ? "1 member" : `${team.member_count} members`}
            >
              <Checkbox
                aria-label={team.name}
                isDisabled={locked}
                isSelected={edit.draft.teams.includes(team.id)}
                onChange={(on) => toggle(team.id, on)}
              />
            </Row>
          ))}
        </div>
      )}
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
      <TypeSection edit={edit} saved={saved} />
      {choosable.includes(edit.draft.dataType) ? <OptionsSection edit={edit} locked={locked} /> : null}
      <DescriptionSection edit={edit} locked={locked} />
      <FillSection edit={edit} locked={locked} />
      <ClosingSection edit={edit} locked={locked} />
      <TeamsSection edit={edit} locked={locked} />
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
