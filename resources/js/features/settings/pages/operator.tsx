/*
 * How Operator, the team's own agent, works in this workspace. It is on
 * by default and spends only when a teammate asks it something; the two
 * things it can do on its own, briefing a conversation as it opens and
 * triaging every new conversation, spend tokens each time, so they start
 * off. Admins write the instructions every run reads and choose, per
 * kind of change, whether Operator's changes apply on their own, wait
 * for a teammate, or are never offered; article and procedure changes
 * always wait. Each teammate sees what they asked Operator to remember
 * and can make it forget. Settings change only when Save is pressed in
 * the bar at the bottom; leaving with unsaved changes asks first.
 */
import { RiDeleteBin6Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { ModelSelect, NoConnection } from "@/features/ai/agent/model-section";
import { modelsFor, operatorModel, usableKeys } from "@/features/ai/format";
import { agentQuery } from "@/lib/api/ai";
import {
  forgetOperatorMemory,
  OPERATOR_GROUPS,
  type OperatorGroup,
  type OperatorPolicy,
  type OperatorSettings,
  type OperatorSettingsChange,
  operatorKeys,
  operatorMemoriesQuery,
  operatorSettingsQuery,
  updateOperatorSettings,
} from "@/lib/api/operator";
import { callerQuery } from "@/lib/api/workspace";

import { Empty, Row, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const groupLabels: Record<OperatorGroup, { label: string; description: string }> = {
  conversations: { label: "Conversations", description: "Tags, assignment, priority, status and subject." },
  notes: { label: "Internal notes", description: "Notes Operator adds to a conversation for the team." },
  contacts: { label: "Customers", description: "A customer's name." },
  articles: { label: "Help articles", description: "New drafts and edits to drafts. Publishing stays yours." },
  procedures: { label: "Procedures", description: "Edits to a procedure's working copy. Publishing stays yours." },
  actions: {
    label: "Workspace actions",
    description: "Actions you gave Operator in Settings, Actions. Lookups run at once; changes wait for you.",
  },
};

const policyLabels: Record<OperatorPolicy, string> = { auto: "On its own", ask: "Ask first", off: "Off" };

function Policy({
  group,
  settings,
  canManage,
  onChange,
}: {
  group: OperatorGroup;
  settings: OperatorSettings;
  canManage: boolean;
  onChange: (change: OperatorSettingsChange) => void;
}) {
  const choices: OperatorPolicy[] = settings.auto_groups.includes(group) ? ["auto", "ask", "off"] : ["ask", "off"];

  return (
    <Row label={groupLabels[group].label} description={groupLabels[group].description}>
      <SegmentedControl
        aria-label={`${groupLabels[group].label} changes`}
        selectedKeys={[settings.policies[group]]}
        isDisabled={!canManage}
        onSelectionChange={(selected) => {
          const next = [...selected][0] as OperatorPolicy | undefined;
          if (next && next !== settings.policies[group]) {
            onChange({ policies: { [group]: next } });
          }
        }}
      >
        {choices.map((choice) => (
          <SegmentedControlItem key={choice} id={choice}>
            {policyLabels[choice]}
          </SegmentedControlItem>
        ))}
      </SegmentedControl>
    </Row>
  );
}

/*
 * ModelRow picks the model Operator answers with, among those the
 * agent's connection offers, each with its price. Without a connection
 * it says where models come from instead of an empty picker.
 */
function ModelRow({
  workspace,
  settings,
  canManage,
  onChange,
}: {
  workspace: string;
  settings: OperatorSettings;
  canManage: boolean;
  onChange: (change: OperatorSettingsChange) => void;
}) {
  const { data: view } = useQuery({ ...agentQuery(workspace), enabled: workspace !== "" });
  const key = view ? usableKeys(view.keys).find((k) => k.id === view.agent.answer_key_id) : undefined;
  const models = view ? modelsFor(key, view.catalog) : [];
  const value = operatorModel(settings.model, models, view?.agent.answer_model ?? null);

  if (view && models.length === 0) {
    return <NoConnection />;
  }

  return (
    <Row
      label="Model"
      description="Operator runs several steps for each answer, so a model's price adds up fast. It starts on GPT-6 Luna, the cheapest that does the job well."
    >
      <ModelSelect
        label="Operator model"
        models={models}
        value={value}
        disabled={!canManage}
        placeholder="Choose a model"
        onPick={(model) => onChange({ model })}
      />
    </Row>
  );
}

function Instructions({
  text,
  canManage,
  onChange,
}: {
  text: string;
  canManage: boolean;
  onChange: (text: string) => void;
}) {
  return (
    <Section
      title="Instructions"
      description="What Operator should know about your team: who owns what, your policies, how you like answers written."
    >
      <div className="flex flex-col gap-3 py-3 pe-3">
        <Textarea
          aria-label="Instructions"
          placeholder="Refunds over $100 go to the Billing team. Answer in short bullet points."
          rows={4}
          autoResize
          maxRows={14}
          value={text}
          isDisabled={!canManage}
          onChange={onChange}
        />
      </div>
    </Section>
  );
}

function Memories({ workspace }: { workspace: string }) {
  const client = useQueryClient();
  const { data: memories = [] } = useQuery(operatorMemoriesQuery(workspace));
  const forget = useMutation({
    mutationFn: (id: string) => forgetOperatorMemory(workspace, id),
    onSuccess: () => client.invalidateQueries({ queryKey: operatorKeys.memories(workspace) }),
  });

  return (
    <Section title="What Operator remembers about you" description="Only you see these. Ask Operator to remember more.">
      {memories.length === 0 ? <Empty>Nothing yet.</Empty> : null}
      {memories.map((memory) => (
        <Row key={memory.id} label={memory.body}>
          <IconButton
            size="small"
            icon={RiDeleteBin6Line}
            aria-label={`Forget ${memory.body}`}
            onClick={() => forget.mutate(memory.id)}
          />
        </Row>
      ))}
    </Section>
  );
}

/*
 * unsaved keeps only the parts of a change that differ from the saved
 * settings, so a value put back as it was leaves the draft.
 */
function unsaved(settings: OperatorSettings, change: OperatorSettingsChange): OperatorSettingsChange {
  const result: OperatorSettingsChange = {};
  for (const key of ["enabled", "brief_on_open", "triage_on_arrival", "instructions", "model"] as const) {
    if (change[key] !== undefined && change[key] !== settings[key]) {
      Object.assign(result, { [key]: change[key] });
    }
  }

  const policies = Object.fromEntries(
    Object.entries(change.policies ?? {}).filter(
      ([group, policy]) => settings.policies[group as OperatorGroup] !== policy,
    ),
  );
  if (Object.keys(policies).length > 0) {
    result.policies = policies;
  }

  return result;
}

/*
 * useOperatorDraft holds the changes made on the page but not saved yet:
 * switches, the model, policies and the instructions. A change that puts
 * a value back as saved drops out of the draft. Save sends them in one
 * call and keeps what came back; Discard forgets them.
 */
function useOperatorDraft(workspace: string, settings: OperatorSettings | undefined) {
  const client = useQueryClient();
  const [pending, setPending] = useState<OperatorSettingsChange>({});
  const save = useMutation({
    mutationFn: () => updateOperatorSettings(workspace, pending),
    onSuccess: async (saved) => {
      client.setQueryData(operatorKeys.settings(workspace), saved);
      await client.invalidateQueries({ queryKey: operatorKeys.settings(workspace) });
      setPending({});
      toast.success("Operator saved");
    },
  });
  const shown = settings
    ? { ...settings, ...pending, policies: { ...settings.policies, ...pending.policies } }
    : undefined;

  return {
    shown,
    dirty: Object.keys(pending).length > 0,
    save,
    change: (next: OperatorSettingsChange) =>
      setPending((current) => {
        if (!settings) {
          return current;
        }

        return unsaved(settings, {
          ...current,
          ...next,
          policies: { ...current.policies, ...next.policies },
        });
      }),
    discard: () => setPending({}),
  };
}

export function OperatorSettingsView() {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery({ ...operatorSettingsQuery(workspace), enabled: workspace !== "" });
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = (caller?.member?.permissions ?? []).includes("ai_agent:manage");
  const draft = useOperatorDraft(workspace, data);
  const settings = draft.shown;
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Operator"
      description="Your team's own agent. It answers teammates, never customers, and every change it makes can be undone."
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
      <Section title="Operator" loading={!settings} rows={4}>
        {settings ? (
          <>
            <Row label="Operator" description="Teammates can ask Operator. It spends only when someone asks.">
              <Switch
                aria-label="Operator"
                size="sm"
                isSelected={settings.enabled}
                isDisabled={!canManage}
                onChange={(on) => draft.change({ enabled: on })}
              />
            </Row>
            <ModelRow workspace={workspace} settings={settings} canManage={canManage} onChange={draft.change} />
            <Row
              label="Brief conversations as they open"
              description="Operator reads each conversation a teammate opens and briefs them. Spends tokens on every open."
            >
              <Switch
                aria-label="Brief conversations as they open"
                size="sm"
                isSelected={settings.brief_on_open}
                isDisabled={!canManage || !settings.enabled}
                onChange={(on) => draft.change({ brief_on_open: on })}
              />
            </Row>
            <Row
              label="Triage new conversations"
              description="Operator proposes tags, an owner and a priority for every conversation a customer starts. Spends tokens on each one."
            >
              <Switch
                aria-label="Triage new conversations"
                size="sm"
                isSelected={settings.triage_on_arrival}
                isDisabled={!canManage || !settings.enabled}
                onChange={(on) => draft.change({ triage_on_arrival: on })}
              />
            </Row>
          </>
        ) : null}
      </Section>
      {settings ? (
        <Section
          title="What Operator may change"
          description="Changes on their own still show in the chat and can be undone. Operator never changes anything on its own after reading what a customer wrote."
        >
          {OPERATOR_GROUPS.map((group) => (
            <Policy key={group} group={group} settings={settings} canManage={canManage} onChange={draft.change} />
          ))}
        </Section>
      ) : null}
      {settings ? (
        <Instructions
          text={settings.instructions}
          canManage={canManage}
          onChange={(instructions) => draft.change({ instructions })}
        />
      ) : null}
      <Memories workspace={workspace} />
    </SettingsPage>
  );
}
