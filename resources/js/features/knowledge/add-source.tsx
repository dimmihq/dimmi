/*
 * Adding a source, in the same narrow dialog as starting a workflow:
 * pick a kind from a list grouped into connected services and your own
 * content, then one short step at a time. A connected service first
 * picks its account; then whatever the source needs, such as the
 * repository and branch, picked from what the account holds; then the
 * items to read, ticked in a tree that fills the dialog; then what the
 * source is for; then a review. The settings are tried against the real
 * service before the content step. Every move, the first one out of the
 * list included, slides in from the side it comes from.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import {
  createSource,
  previewSource,
  type SourceInput,
  type SourcePreview,
  type SourceType,
  sourceTypesQuery,
} from "@/lib/api/knowledge-sources";

import { Gallery, ReviewStep, StepBar, TreeStep, type Usage, UseStep } from "./add-source-steps";
import { AccountStep } from "./connect-field";
import {
  chosen,
  type FormValues,
  initialValues,
  missing,
  nameFor,
  patternTree,
  type StepId,
  settingsOf,
  stepsFor,
} from "./model";
import { SchemaForm } from "./schema-form";
import { StepMotion } from "./step-motion";

/*
 * Resume is where the flow picks up after the browser came back from
 * a service's consent screen: the kind of source and the account just
 * connected.
 */
export interface Resume {
  type: string;
  installation: string | null;
}

/*
 * Draft is everything chosen so far for the kind picked.
 */
interface Draft {
  kind: SourceType;
  values: FormValues;
  include: string[];
  usage: Usage;
}

type View = "pick" | StepId;

export function AddSourceModal({
  workspace,
  isOpen,
  resume,
  onOpenChange,
}: {
  workspace: string;
  isOpen: boolean;
  resume?: Resume | null;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="md" className="h-[min(680px,calc(100dvh-3rem))]">
      {({ close }) => <Flow workspace={workspace} resume={resume ?? null} onDone={close} />}
    </Modal>
  );
}

function draftOf(kind: SourceType, installation: string | null): Draft {
  const mode = kind.modes[0];

  return {
    kind,
    values: { ...initialValues(kind.schema), ...(installation ? { installation } : {}) },
    include: [],
    usage: {
      mode,
      targets: mode === "import" ? ["help_center"] : kind.targets.filter((t) => t !== "help_center"),
      helpCenter: null,
      schedule: mode === "import" ? "manual" : kind.schedules[0],
      name: kind.name,
    },
  };
}

function inputOf(d: Draft): SourceInput {
  return {
    type: d.kind.type,
    name: d.usage.name,
    mode: d.usage.mode,
    targets: d.usage.targets,
    help_center_id: d.usage.helpCenter,
    settings: settingsOf(d.kind.schema, d.values),
    include: d.include,
    exclude: [],
    schedule: d.usage.mode === "import" ? "manual" : d.usage.schedule,
  };
}

function Flow({ workspace, resume, onDone }: { workspace: string; resume: Resume | null; onDone: () => void }) {
  const types = useQuery(sourceTypesQuery(workspace));
  const nav = useNav(resume ? (types.data?.find((t) => t.type === resume.type) ?? null) : null, resume);
  const [preview, setPreview] = useState<SourcePreview | null>(null);
  const { draft, steps, at, showing } = nav;
  const flow = useStepFlow({
    workspace,
    draft: showing === "pick" ? null : draft,
    step: steps[at]?.id ?? "review",
    lastSetup: lastSetupStep(steps),
    preview,
    onPreview: setPreview,
    onChange: nav.change,
    onNext: () => nav.go(steps[at + 1]?.id ?? "review", true),
    onDone,
  });
  const pick = (kind: SourceType) => {
    setPreview(null);
    nav.begin(kind);
  };
  const motionClass = flow.filling ? "flex min-h-0 flex-1 flex-col" : undefined;

  return (
    <>
      <FlowHeader kind={showing === "pick" ? null : (draft?.kind ?? null)} steps={steps} at={at} />
      <ModalBody className={flow.filling ? "flex flex-col" : undefined}>
        <StepMotion view={showing} forward={nav.forward} className={motionClass}>
          {showing === "pick" ? <Gallery types={types.data ?? []} onPick={pick} /> : flow.body}
        </StepMotion>
      </ModalBody>
      {showing === "pick" ? null : (
        <ModalFooter>
          <Button variant="secondary" onClick={nav.back} disabled={flow.busy}>
            Back
          </Button>
          {flow.action}
        </ModalFooter>
      )}
    </>
  );
}

/*
 * useNav keeps the kind being added, what was chosen for it, and the
 * view shown with the direction of the last move.
 */
function useNav(resumed: SourceType | null, resume: Resume | null) {
  const [picked, setDraft] = useState<Draft | null>(null);
  const draft = picked ?? (resumed ? draftOf(resumed, resume?.installation ?? null) : null);
  const [view, setView] = useState<View>(resume ? "account" : "pick");
  const [forward, setForward] = useState(true);
  const steps = draft ? stepsFor(draft.kind) : [];
  const at = Math.max(
    steps.findIndex((s) => s.id === view),
    0,
  );
  const go = (next: View, isForward: boolean) => {
    setForward(isForward);
    setView(next);
  };

  return {
    draft,
    steps,
    at,
    forward,
    go,
    showing: (view === "pick" || !draft ? "pick" : view) as View,
    change: (changes: Partial<Draft>) => draft && setDraft({ ...draft, ...changes }),
    back: () => go(at <= 0 ? "pick" : steps[at - 1].id, false),
    begin: (kind: SourceType) => {
      setDraft(draftOf(kind, null));
      go(stepsFor(kind)[0].id, true);
    },
  };
}

/*
 * FlowHeader names the kind being added with its steps, or asks which
 * kind to add.
 */
function FlowHeader({
  kind,
  steps,
  at,
}: {
  kind: SourceType | null;
  steps: { id: StepId; label: string }[];
  at: number;
}) {
  if (!kind) {
    return <ModalHeader title="Add a source" description="Where should dimmi read knowledge from?" />;
  }

  return (
    <ModalHeader title={`Add ${kind.name}`} description={kind.summary}>
      <StepBar steps={steps} at={at} />
    </ModalHeader>
  );
}

/*
 * useStepFlow draws the step being shown and its main action: trying
 * the settings at the end of setup, adding the source on review, and
 * moving on everywhere else.
 */
function useStepFlow({
  workspace,
  draft,
  step,
  lastSetup,
  preview,
  onPreview,
  onChange,
  onNext,
  onDone,
}: {
  workspace: string;
  draft: Draft | null;
  step: StepId;
  lastSetup: StepId | null;
  preview: SourcePreview | null;
  onPreview: (p: SourcePreview) => void;
  onChange: (changes: Partial<Draft>) => void;
  onNext: () => void;
  onDone: () => void;
}) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const [errors, setErrors] = useState<Record<string, string>>({});

  const check = useMutation({
    mutationFn: (d: Draft) => previewSource(workspace, inputOf(d)),
    onSuccess: (p, d) => {
      setErrors({});
      onPreview(p);
      const name = d.usage.name === d.kind.name ? nameFor(d.kind.name, d.values) : d.usage.name;
      onChange({ usage: { ...d.usage, name } });
      onNext();
    },
    onError: (err) => {
      setErrors(err instanceof ApiError ? err.fields : {});
      toast.error(err instanceof ApiError ? err.message : "dimmi could not connect with these settings.");
    },
  });

  const create = useMutation({
    mutationFn: (d: Draft) => createSource(workspace, { ...inputOf(d), sync_now: true }),
    onSuccess: async (source) => {
      await client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
      toast.success(`${source.name} added. The first sync is running.`);
      onDone();
      await navigate({ to: "/d/settings/knowledge/$source", params: { source: source.id } });
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : "The source could not be added."),
  });

  if (!draft) {
    return { body: null, action: null, filling: false, busy: false };
  }

  const { kind, values, usage } = draft;
  const selection = patternTree(kind) ? draft.include : chosen(values);
  const action =
    step === "review" ? (
      <Button onClick={() => create.mutate(draft)} loading={create.isPending}>
        Add and sync
      </Button>
    ) : (
      <Button
        onClick={() => (step === lastSetup ? check.mutate(draft) : onNext())}
        loading={check.isPending}
        disabled={blockedAt(step, draft)}
      >
        Continue
      </Button>
    );

  const bodies: Record<StepId, ReactNode> = {
    account: (
      <AccountStep workspace={workspace} kind={kind} values={values} onChange={(v) => onChange({ values: v })} />
    ),
    setup: (
      <SchemaForm
        workspace={workspace}
        kind={kind}
        values={values}
        errors={errors}
        skipAccount
        onChange={(v) => onChange({ values: v })}
      />
    ),
    content: (
      <TreeStep
        workspace={workspace}
        input={inputOf(draft)}
        selected={selection}
        onChange={(next) =>
          patternTree(kind) ? onChange({ include: next }) : onChange({ values: { ...values, selected: next } })
        }
      />
    ),
    use: <UseStep workspace={workspace} kind={kind} usage={usage} onChange={(u) => onChange({ usage: u })} />,
    review: <ReviewStep kind={kind} usage={usage} preview={preview} picked={selection.length} />,
  };

  return { body: bodies[step], action, filling: step === "content", busy: check.isPending || create.isPending };
}

/*
 * lastSetupStep is the step after which the settings are tried against
 * the service: the setup step, or the account when there is none.
 */
function lastSetupStep(steps: { id: StepId }[]): StepId | null {
  if (steps.some((s) => s.id === "setup")) {
    return "setup";
  }

  return steps.some((s) => s.id === "account") ? "account" : null;
}

function blockedAt(step: StepId, d: Draft): boolean {
  switch (step) {
    case "account":
      return typeof d.values.installation !== "string" || d.values.installation === "";
    case "setup":
      return missing(d.kind.schema, d.values).length > 0;
    case "use":
      return (d.usage.targets.includes("help_center") && !d.usage.helpCenter) || d.usage.targets.length === 0;
    default:
      return false;
  }
}
