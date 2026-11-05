/*
 * Setting up a workspace, step by step. Each step the owners reach is
 * saved on the workspace before it shows, so leaving and coming back
 * resumes there.
 * Members who cannot change settings never see the steps; they wait for
 * the workspace's owners to finish.
 */
import { RiTimeLine } from "@remixicon/react";
import { useState } from "react";

import type { Me } from "@/lib/api/auth";
import { updateOnboarding, type Workspace } from "@/lib/api/workspace";

import { ChannelsStep } from "./channels-step";
import { OnboardingFrame } from "./frame";
import { KnowledgeStep, LogoStep } from "./later-steps";
import { PracticeStep } from "./practice-step";
import { nextStep, position, previousStep, type Step, stepFrom, steps } from "./steps";
import { SummaryStep } from "./summary-step";
import { TagsStep } from "./tags-step";
import { TeamStep } from "./team-step";
import { WorkspaceStep } from "./workspace-step";

export interface OnboardingProps {
  me: Me;
  workspace: Workspace | null;
  canSetUp: boolean;
}

function SettingUp({ name }: { name: string }) {
  return (
    <div className="flex flex-col items-start gap-3">
      <RiTimeLine aria-hidden className="size-6 text-foreground-icon-secondary" />
      <h1 className="text-title-1-medium text-text-primary">{name} is being set up</h1>
      <p className="text-body-regular text-text-secondary">
        Its owners are still setting it up. Come back once they finish; this page opens the workspace then.
      </p>
    </div>
  );
}

export function Onboarding({ me, workspace: initial, canSetUp }: OnboardingProps) {
  const [workspace, setWorkspace] = useState<string | null>(initial?.id ?? null);
  const [name, setName] = useState(initial?.name ?? "");
  const [step, setStep] = useState<Step>(stepFrom(initial?.onboarding.step));
  const [practiced, setPracticed] = useState(false);

  if (initial && !canSetUp) {
    return (
      <OnboardingFrame email={me.user.email} done={0}>
        <SettingUp name={initial.name} />
      </OnboardingFrame>
    );
  }

  const go = (to: Step, id = workspace) => {
    const save = id ? updateOnboarding(id, to) : Promise.resolve();
    void save.catch(() => undefined).then(() => setStep(to));
  };
  const label = `Step ${position(step)} of ${steps.length}`;
  const done = step === "done" ? steps.length : position(step) - 1;

  return (
    <OnboardingFrame email={me.user.email} done={done}>
      {step === "workspace" || workspace === null ? (
        <WorkspaceStep
          workspace={workspace}
          name={name}
          label={label}
          onSaved={(id) => {
            setWorkspace(id);
            go(nextStep("workspace"), id);
          }}
          onNamed={setName}
        />
      ) : (
        <CurrentStep
          step={step}
          workspace={workspace}
          name={name}
          label={label}
          practiced={practiced}
          onPracticed={() => setPracticed(true)}
          go={go}
        />
      )}
    </OnboardingFrame>
  );
}

function CurrentStep({
  step,
  workspace,
  name,
  label,
  practiced,
  onPracticed,
  go,
}: {
  step: Step;
  workspace: string;
  name: string;
  label: string;
  practiced: boolean;
  onPracticed: () => void;
  go: (to: Step) => void;
}) {
  const props = { workspace, label, onNext: () => go(nextStep(step)), onBack: () => go(previousStep(step)) };

  switch (step) {
    case "logo":
      return <LogoStep {...props} />;
    case "channels":
      return <ChannelsStep {...props} />;
    case "tags":
      return <TagsStep {...props} />;
    case "team":
      return <TeamStep {...props} />;
    case "knowledge":
      return <KnowledgeStep {...props} />;
    case "test_conversations":
      return <PracticeStep {...props} onCreated={onPracticed} />;
    default:
      return <SummaryStep workspace={workspace} name={name} practiced={practiced} onBack={props.onBack} />;
  }
}
