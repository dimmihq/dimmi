/*
 * How Operator, the team's own agent, works in this workspace. It is on
 * by default and spends only when a teammate asks it something; the two
 * things it can do on its own, briefing a conversation as it opens and
 * triaging every new conversation, spend tokens each time, so they start
 * off. Admins write the instructions every run reads and choose, per
 * kind of change, whether Operator's changes apply on their own, wait
 * for a teammate, or are never offered; article and procedure changes
 * always wait. Each teammate sees what they asked Operator to remember
 * and can make it forget.
 */
import { RiDeleteBin6Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
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

const agentModel = "agent";

/*
ModelRow picks the model Operator answers with, among those the
agent's connection offers; the default follows the agent.
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
  const key = view?.keys.find((k) => k.id === view.agent.answer_key_id);
  const choices = key?.models ?? [];
  const agent = view?.agent.answer_model ?? null;

  return (
    <Row
      label="Model"
      description="Operator works through many steps, so a stronger model than the agent's often pays off."
    >
      <Select
        aria-label="Model"
        size="sm"
        isDisabled={!canManage || choices.length === 0}
        selectedKey={settings.model ?? agentModel}
        onSelectionChange={(picked) =>
          picked ? onChange({ model: picked === agentModel ? "" : String(picked) }) : undefined
        }
        popoverClassName="w-64"
      >
        {[agentModel, ...choices].map((choice) => {
          const label = choice === agentModel ? `Same as the agent${agent ? ` (${agent})` : ""}` : choice;

          return (
            <SelectItem key={choice} id={choice} textValue={label}>
              {label}
            </SelectItem>
          );
        })}
      </Select>
    </Row>
  );
}

function Instructions({
  settings,
  canManage,
  onSave,
}: {
  settings: OperatorSettings;
  canManage: boolean;
  onSave: (text: string) => Promise<unknown>;
}) {
  const [text, setText] = useState(settings.instructions);
  const save = useMutation({ mutationFn: () => onSave(text) });

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
          onChange={setText}
        />
        {canManage ? (
          <Button
            size="small"
            variant="secondary"
            className="self-start"
            loading={save.isPending}
            disabled={text === settings.instructions}
            onClick={() => save.mutate()}
          >
            Save instructions
          </Button>
        ) : null}
        {save.error ? <p className="text-body-2-regular text-text-error-primary">{failureText(save.error)}</p> : null}
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

export function OperatorSettingsView() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const { data: settings } = useQuery({ ...operatorSettingsQuery(workspace), enabled: workspace !== "" });
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = (caller?.member?.permissions ?? []).includes("ai_agent:manage");
  const change = useMutation({
    mutationFn: (next: OperatorSettingsChange) => updateOperatorSettings(workspace, next),
    onSuccess: (saved) => client.setQueryData(operatorKeys.settings(workspace), saved),
  });

  return (
    <SettingsPage
      title="Operator"
      description="Your team's own agent. It answers teammates, never customers, and every change it makes can be undone."
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
                onChange={(on) => change.mutate({ enabled: on })}
              />
            </Row>
            <ModelRow
              workspace={workspace}
              settings={settings}
              canManage={canManage}
              onChange={(next) => change.mutate(next)}
            />
            <Row
              label="Brief conversations as they open"
              description="Operator reads each conversation a teammate opens and briefs them. Spends tokens on every open."
            >
              <Switch
                aria-label="Brief conversations as they open"
                size="sm"
                isSelected={settings.brief_on_open}
                isDisabled={!canManage || !settings.enabled}
                onChange={(on) => change.mutate({ brief_on_open: on })}
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
                onChange={(on) => change.mutate({ triage_on_arrival: on })}
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
            <Policy
              key={group}
              group={group}
              settings={settings}
              canManage={canManage}
              onChange={(next) => change.mutate(next)}
            />
          ))}
        </Section>
      ) : null}
      {settings ? (
        <Instructions
          settings={settings}
          canManage={canManage}
          onSave={(instructions) => change.mutateAsync({ instructions })}
        />
      ) : null}
      <Memories workspace={workspace} />
      {change.error ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {failureText(change.error)}
        </p>
      ) : null}
    </SettingsPage>
  );
}
