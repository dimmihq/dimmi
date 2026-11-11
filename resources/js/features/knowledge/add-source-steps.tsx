/*
 * The parts of adding a source. The kinds are listed like the triggers
 * of a new workflow, connected services first. The steps show as bars
 * filled up to the current one. Content is a tree of what the source
 * holds that fills the dialog and scrolls inside it, with a count of
 * what the ticks read. Use picks what the source becomes, where its
 * articles go or what it feeds, and how often it syncs, each choice a
 * card. Review stacks each answer under its name before anything is
 * saved.
 */
import { RiArrowRightSLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { CheckboxCard } from "@/components/base/checkbox/checkbox-card";
import { Input } from "@/components/base/input/input";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { helpCentersQuery } from "@/lib/api/help-centers";
import {
  previewTree,
  type SourceInput,
  type SourceMode,
  type SourcePreview,
  type SourceSchedule,
  type SourceTarget,
  type SourceType,
} from "@/lib/api/knowledge-sources";

import { SourceLogo } from "./logos";
import { countChosen, type StepId, scheduleLabels, targetLabels } from "./model";
import { TreePicker, TreeSkeleton } from "./tree-picker";

/*
 * Usage is what a source is for: its mode, what it feeds, the help
 * center articles go to, how often it syncs and its name.
 */
export interface Usage {
  mode: SourceMode;
  targets: SourceTarget[];
  helpCenter: string | null;
  schedule: SourceSchedule;
  name: string;
}

/*
 * Gallery lists the kinds of source the server offers, the services
 * connected through an account first, each with its logo, what it does
 * and a chevron, as picking one moves on.
 */
export function Gallery({ types, onPick }: { types: SourceType[]; onPick: (type: SourceType) => void }) {
  const groups = [
    {
      id: "services",
      name: "Connected services",
      detail: "Sign in to the service, then pick what dimmi reads.",
      kinds: types.filter((t) => t.auth === "oauth"),
    },
    {
      id: "own",
      name: "Your own content",
      detail: "A public website or Markdown files you upload.",
      kinds: types.filter((t) => t.auth !== "oauth"),
    },
  ].filter((g) => g.kinds.length > 0);

  return (
    <div className="flex flex-col gap-5 pb-2">
      {groups.map((group) => (
        <section key={group.id} aria-label={group.name} className="flex flex-col gap-2">
          <div className="flex flex-col">
            <h3 className="text-body-2-medium text-text-primary">{group.name}</h3>
            <p className="text-body-2-regular text-text-secondary">{group.detail}</p>
          </div>
          <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
            {group.kinds.map((t) => (
              <button
                key={t.type}
                type="button"
                onClick={() => onPick(t)}
                className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <SourceLogo logo={t.logo} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="text-body-regular text-text-primary">{t.name}</span>
                  <span className="text-body-2-regular text-text-secondary">{t.summary}</span>
                </span>
                <RiArrowRightSLine
                  aria-hidden
                  className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100"
                />
              </button>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

/*
 * StepBar shows the steps as a bar each, filled up to the current one,
 * with the current step's name.
 */
export function StepBar({ steps, at }: { steps: { id: StepId; label: string }[]; at: number }) {
  return (
    <div className="mt-4 flex flex-col gap-2">
      <ol className="flex gap-1.5" aria-label="Steps">
        {steps.map((s, i) => (
          <li key={s.id} aria-current={i === at ? "step" : undefined} className="flex flex-1 flex-col">
            <span className="sr-only">{s.label}</span>
            <span
              aria-hidden
              className={
                i <= at
                  ? "h-1 rounded-full bg-accent-500 transition-colors duration-300"
                  : "h-1 rounded-full bg-background-tertiary-default transition-colors duration-300"
              }
            />
          </li>
        ))}
      </ol>
      <span className="text-body-2-regular text-text-secondary">
        Step {at + 1} of {steps.length} · {steps[at]?.label}
      </span>
    </div>
  );
}

/*
 * TreeStep shows what the source holds to tick, filling the dialog,
 * with how many documents the ticks read.
 */
export function TreeStep({
  workspace,
  input,
  selected,
  onChange,
}: {
  workspace: string;
  input: SourceInput;
  selected: string[];
  onChange: (selected: string[]) => void;
}) {
  const settings = Object.fromEntries(Object.entries(input.settings ?? {}).filter(([key]) => key !== "selected"));
  const tree = useQuery({
    queryKey: ["workspace", workspace, "knowledge", "tree-preview", input.type, settings],
    queryFn: () => previewTree(workspace, { ...input, include: [] }),
  });

  if (tree.isError) {
    return (
      <p className="text-body-regular text-text-error-primary">The items could not be read. Go back and try again.</p>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <p className="text-body-regular text-text-secondary" aria-live="polite">
        {treeLine(tree.data ? countChosen(tree.data, selected) : null, selected.length)}
      </p>
      {tree.data ? <TreePicker nodes={tree.data} selected={selected} onChange={onChange} fill /> : <TreeSkeleton />}
    </div>
  );
}

function treeLine(count: number | null, ticked: number): string {
  if (count === null) {
    return "Reading what this account holds…";
  }

  return ticked === 0
    ? `Nothing ticked: all ${count} documents will be read.`
    : `${count} documents will be read. You can change this any time.`;
}

const modeChoices: { id: SourceMode; title: string; description: string }[] = [
  { id: "sync", title: "A synced copy", description: "dimmi reads it to answer and keeps it up to date." },
  {
    id: "import",
    title: "Help center articles",
    description: "Imported as articles you can edit, with redirects from their old addresses.",
  },
];

const targetDetails: Record<SourceTarget, string> = {
  agent: "Answers your customers.",
  operator: "Helps your team in the inbox.",
  help_center: "Becomes articles.",
};

export function UseStep({
  workspace,
  kind,
  usage,
  onChange,
}: {
  workspace: string;
  kind: SourceType;
  usage: Usage;
  onChange: (usage: Usage) => void;
}) {
  const syncTargets = kind.targets.filter((t) => t !== "help_center");
  const set = (changes: Partial<Usage>) => onChange({ ...usage, ...changes });

  return (
    <div className="flex flex-col gap-5 pb-2">
      <Input label="Name" value={usage.name} onChange={(name) => set({ name })} maxLength={80} />
      {kind.modes.length > 1 ? (
        <ModeChoice
          modes={kind.modes}
          value={usage.mode}
          onChange={(mode) =>
            set({
              mode,
              targets: mode === "import" ? ["help_center"] : syncTargets,
              schedule: mode === "import" ? "manual" : kind.schedules[0],
            })
          }
        />
      ) : null}
      {usage.mode === "sync" ? (
        <TargetChoice targets={syncTargets} value={usage.targets} onChange={(targets) => set({ targets })} />
      ) : (
        <HelpCenterChoice workspace={workspace} value={usage.helpCenter} onChange={(id) => set({ helpCenter: id })} />
      )}
      {usage.mode === "sync" && kind.schedules.length > 1 ? (
        <ScheduleChoice schedules={kind.schedules} value={usage.schedule} onChange={(schedule) => set({ schedule })} />
      ) : null}
    </div>
  );
}

function ModeChoice({
  modes,
  value,
  onChange,
}: {
  modes: SourceMode[];
  value: SourceMode;
  onChange: (mode: SourceMode) => void;
}) {
  return (
    <Choice label="What should it become?">
      <RadioGroup aria-label="What should it become?" value={value} onChange={(v) => onChange(v as SourceMode)}>
        {modeChoices
          .filter((c) => modes.includes(c.id))
          .map((c) => (
            <RadioCard key={c.id} value={c.id} title={c.title} description={c.description} />
          ))}
      </RadioGroup>
    </Choice>
  );
}

function TargetChoice({
  targets,
  value,
  onChange,
}: {
  targets: SourceTarget[];
  value: SourceTarget[];
  onChange: (targets: SourceTarget[]) => void;
}) {
  return (
    <Choice label="Answer from it">
      <div className="flex flex-col gap-2">
        {targets.map((target) => (
          <CheckboxCard
            key={target}
            title={targetLabels[target]}
            description={targetDetails[target]}
            isSelected={value.includes(target)}
            onChange={(on) => onChange(on ? [...value, target] : value.filter((t) => t !== target))}
          />
        ))}
      </div>
    </Choice>
  );
}

function ScheduleChoice({
  schedules,
  value,
  onChange,
}: {
  schedules: SourceSchedule[];
  value: SourceSchedule;
  onChange: (schedule: SourceSchedule) => void;
}) {
  return (
    <Choice label="Sync">
      <SegmentedControl
        aria-label="Sync"
        selectedKeys={[value]}
        onSelectionChange={(keys) => {
          const [key] = [...keys];
          if (key !== undefined) {
            onChange(String(key) as SourceSchedule);
          }
        }}
      >
        {schedules.map((s) => (
          <SegmentedControlItem key={s} id={s}>
            {scheduleLabels[s]}
          </SegmentedControlItem>
        ))}
      </SegmentedControl>
    </Choice>
  );
}

function HelpCenterChoice({
  workspace,
  value,
  onChange,
}: {
  workspace: string;
  value: string | null;
  onChange: (id: string) => void;
}) {
  const centers = useQuery(helpCentersQuery(workspace));

  return (
    <Choice label="Help center">
      <RadioGroup aria-label="Help center" value={value} onChange={onChange}>
        {(centers.data ?? []).map((c) => (
          <RadioCard key={c.id} value={c.id} title={c.name} />
        ))}
      </RadioGroup>
    </Choice>
  );
}

function Choice({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="text-body-2-medium text-text-primary">{label}</span>
      {children}
    </div>
  );
}

export function ReviewStep({
  kind,
  usage,
  preview,
  picked,
}: {
  kind: SourceType;
  usage: Usage;
  preview: SourcePreview | null;
  picked: number;
}) {
  const lines = [
    ["Source", `${kind.name}${preview ? ` · ${preview.account}` : ""}`],
    ["Name", usage.name],
    [
      "Becomes",
      usage.mode === "import"
        ? "Help center articles, with redirects from their old addresses"
        : `A synced copy for ${usage.targets.map((t) => targetLabels[t]).join(" and ")}`,
    ],
    ["Sync", usage.mode === "import" ? "Once now; sync again to update" : scheduleLabels[usage.schedule]],
    ["Content", contentLine(picked)],
  ];

  return (
    <dl className="flex flex-col divide-y divide-separator-border">
      {lines.map(([term, detail]) => (
        <div key={term} className="flex flex-col gap-0.5 py-3 first:pt-0">
          <dt className="text-body-2-regular text-text-secondary">{term}</dt>
          <dd className="text-body-regular text-text-primary">{detail}</dd>
        </div>
      ))}
    </dl>
  );
}

function contentLine(picked: number): string {
  if (picked === 0) {
    return "Everything it can read";
  }

  return picked === 1
    ? "1 chosen item, with everything under it"
    : `${picked} chosen items, with everything under them`;
}
