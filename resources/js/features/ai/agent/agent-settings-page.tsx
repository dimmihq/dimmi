/*
 * The agent's settings, from whether it answers at all down to what it
 * may spend. Every control saves on its own as soon as it changes, with
 * a toast to say so, so there is no form to submit and nothing to lose.
 * People the server does not let change the agent see the same page
 * with every control locked and a line saying who can change it.
 */
import { RiLockLine } from "@remixicon/react";
import { useRef } from "react";

import { PageSkeleton, SettingsPage } from "@/features/settings/layout";

import { useAgent, useAgentSave } from "../use-agent";
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

export function AgentSettings() {
  const { workspace, view, canWrite } = useAgent();
  const save = useAgentSave(workspace);
  const where = useRef<HTMLDivElement>(null);
  const model = useRef<HTMLDivElement>(null);

  if (!view.data) {
    return <PageSkeleton />;
  }
  const { agent } = view.data;

  return (
    <SettingsPage
      title="dimmi agent"
      description="Answers customers from your knowledge, and hands over to the team when it should."
    >
      {canWrite ? null : <ReadOnlyNote />}
      <StatusSection
        view={view.data}
        canWrite={canWrite}
        save={save}
        onJump={(step) => (step === "model" ? model : where).current?.scrollIntoView({ behavior: "smooth" })}
      />
      <IdentitySection agent={agent} canWrite={canWrite} save={save} />
      <div ref={where} className="scroll-mt-6">
        <WhereSection agent={agent} canWrite={canWrite} save={save} />
      </div>
      <div ref={model} className="scroll-mt-6">
        <ModelSection view={view.data} canWrite={canWrite} save={save} />
      </div>
      <AnswersSection agent={agent} canWrite={canWrite} save={save} />
      <HandoverSection workspace={workspace} agent={agent} canWrite={canWrite} save={save} />
      <PrivacySection agent={agent} canWrite={canWrite} save={save} />
      <SpendSection workspace={workspace} agent={agent} canWrite={canWrite} save={save} />
    </SettingsPage>
  );
}
