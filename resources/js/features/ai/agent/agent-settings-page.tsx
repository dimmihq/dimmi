/*
 * The agent's settings, from whether it answers at all down to what it
 * may spend. Every change goes into one draft for the whole page, shown
 * at once in the controls, and is sent only when the save bar's Save is
 * pressed; Discard puts the saved settings back, and leaving with
 * changes unsaved asks first. People the server does not let change the
 * agent see the same page with every control locked and a line saying
 * who can change it.
 */
import { RiLockLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { PageSkeleton, SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { type AgentView, type AiAgent, agentQuery, aiKey, updateAgent } from "@/lib/api/ai";

import { type AgentChange, applyChange } from "../format";
import { refusal, useAgent } from "../use-agent";
import { AnswersSection, PrivacySection } from "./answers-section";
import { HandoverSection } from "./handover-section";
import { IdentitySection } from "./identity-section";
import { ModelSection } from "./model-section";
import { SpendSection } from "./spend-section";
import { StatusSection } from "./status-section";
import { WhereSection } from "./where-section";

function ReadOnlyNote() {
  return (
    <p className="flex items-center gap-2 rounded-2xl bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-secondary">
      <RiLockLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
      Only admins can change the agent. You can see how it is set up.
    </p>
  );
}

/*
 * changed keeps the parts of a draft that differ from the saved agent,
 * so a setting changed and changed back is not sent.
 */
function changed(saved: AiAgent, pending: AgentChange): AgentChange {
  const before = JSON.stringify(saved);

  return Object.fromEntries(
    Object.entries(pending).filter(
      ([key, value]) => JSON.stringify(applyChange(saved, { [key]: value } as AgentChange)) !== before,
    ),
  ) as AgentChange;
}

/*
 * tidied is a change as it is sent: names without the spaces around
 * them.
 */
function tidied(change: AgentChange): AgentChange {
  return {
    ...change,
    ...(change.name !== undefined ? { name: change.name.trim() } : {}),
    ...(typeof change.product_name === "string" ? { product_name: change.product_name.trim() } : {}),
  };
}

/*
 * useAgentDraft holds the changes made on the page but not saved yet.
 * Save sends only what differs from the saved agent in one change, and
 * keeps whatever was changed again while it was being sent. The name
 * cannot be saved empty.
 */
function useAgentDraft(workspace: string, view: AgentView | undefined) {
  const client = useQueryClient();
  const [pending, setPending] = useState<AgentChange>({});
  const changes = view ? changed(view.agent, pending) : {};
  const dirty = Object.keys(changes).length > 0;
  const unnamed = changes.name !== undefined && changes.name.trim() === "";
  const save = useMutation({
    mutationFn: (sent: AgentChange) => updateAgent(workspace, tidied(sent)),
    onSuccess: (saved, sent) => {
      client.setQueryData(agentQuery(workspace).queryKey, saved);
      setPending((current) =>
        Object.fromEntries(
          Object.entries(current).filter(
            ([key, value]) => JSON.stringify(value) !== JSON.stringify(sent[key as keyof AgentChange]),
          ),
        ),
      );
      toast.success("Saved");
    },
    onSettled: () => client.invalidateQueries({ queryKey: aiKey(workspace) }),
  });

  return {
    view: view ? { ...view, agent: applyChange(view.agent, pending) } : undefined,
    dirty,
    unnamed,
    save,
    set: (change: AgentChange) => setPending((current) => ({ ...current, ...change })),
    submit: () => save.mutate(changes),
    discard: () => {
      setPending({});
      save.reset();
    },
  };
}

export function AgentSettings() {
  const { workspace, view, canWrite } = useAgent();
  const draft = useAgentDraft(workspace, view.data);
  const where = useRef<HTMLDivElement>(null);
  const model = useRef<HTMLDivElement>(null);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  if (!draft.view) {
    return <PageSkeleton />;
  }
  const { agent } = draft.view;
  const save = draft.set;

  return (
    <SettingsPage
      title="dimmi agent"
      description="Answers customers from your knowledge, and hands over to the team when it should."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? refusal(draft.save.error, "Not saved. Check your connection and try again.") : null}
          message={draft.unnamed ? "Give the agent a name to save" : undefined}
          canSave={draft.dirty && !draft.unnamed}
          onSave={draft.submit}
          onDiscard={draft.discard}
        />
      }
    >
      {canWrite ? null : <ReadOnlyNote />}
      <StatusSection
        view={draft.view}
        canWrite={canWrite}
        save={save}
        onJump={(step) => (step === "model" ? model : where).current?.scrollIntoView({ behavior: "smooth" })}
      />
      <IdentitySection agent={agent} canWrite={canWrite} save={save} />
      <div ref={where} className="scroll-mt-6">
        <WhereSection agent={agent} canWrite={canWrite} save={save} />
      </div>
      <div ref={model} className="scroll-mt-6">
        <ModelSection view={draft.view} canWrite={canWrite} save={save} />
      </div>
      <AnswersSection agent={agent} canWrite={canWrite} save={save} />
      <HandoverSection workspace={workspace} agent={agent} canWrite={canWrite} save={save} />
      <PrivacySection agent={agent} canWrite={canWrite} save={save} />
      <SpendSection workspace={workspace} agent={agent} canWrite={canWrite} save={save} />
    </SettingsPage>
  );
}
