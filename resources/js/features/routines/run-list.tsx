/*
 * A routine's runs, newest first. Each says what came of it, how it
 * started, how long it took and how much it read, and marks the
 * suggestions still waiting for a teammate. The list filters by how a
 * run ended; dry runs stay out of it until asked for, since trying a
 * routine out leaves many of them.
 */
import { RiFlaskLine, RiPlayLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { KindMark } from "@/features/automation/parts";
import { Item, Section } from "@/features/settings/layout";
import { type Routine, type RoutineRun, routineRunsQuery } from "@/lib/api/routines";
import { formatDateTime, formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { type Ended, endings, lastLine, readLine, runLine, shownRuns, tookOf, triggers } from "./runs";
import { describeRoutine } from "./timing";

export function Runs({
  workspace,
  routine,
  onOpen,
}: {
  workspace: string;
  routine: Routine;
  /* Opens a run beside the routine; without it a run opens on its own page. */
  onOpen?: (run: string) => void;
}) {
  const { data, isPending } = useQuery(routineRunsQuery(workspace, routine.id));
  const runs = data ?? [];
  const [ended, setEnded] = useState<Ended>("all");
  /* Dry runs show by default only while the routine has no other run. */
  const [picked, setPicked] = useState<boolean | null>(null);
  const dry = picked ?? (runs.length > 0 && runs.every((r) => r.trigger === "dry_run"));
  const shown = shownRuns(runs, ended, dry);
  const next =
    routine.status === "active" && routine.next_run_at
      ? `Next run ${formatRelative(routine.next_run_at)}. ${describeRoutine(routine)}.`
      : "";

  return (
    <Section
      title="Runs"
      description={[next, lastLine(runs)].filter(Boolean).join(" ") || undefined}
      loading={isPending}
      action={
        runs.length > 0 ? (
          <div className="flex flex-wrap items-center justify-end gap-3">
            <Switch size="sm" isSelected={dry} onChange={setPicked}>
              <span className="text-body-2-regular text-text-secondary">Dry runs</span>
            </Switch>
            <SegmentedControl
              aria-label="Show runs that"
              selectedKeys={new Set([ended])}
              onSelectionChange={(keys) => {
                const key = [...keys][0] as Ended | undefined;
                if (key) {
                  setEnded(key);
                }
              }}
            >
              {endings.map((e) => (
                <SegmentedControlItem key={e.id} id={e.id}>
                  {e.label}
                </SegmentedControlItem>
              ))}
            </SegmentedControl>
          </div>
        ) : null
      }
    >
      {runs.length === 0 ? (
        <Item
          mark={<KindMark icon={RiPlayLine} tint="neutral" />}
          title="No runs yet"
          detail="Try a dry run: it reads real data and shows what it would suggest, without changing anything."
        />
      ) : null}
      {runs.length > 0 && shown.length === 0 ? (
        <Item
          mark={<KindMark icon={RiPlayLine} tint="neutral" />}
          title="No runs like that"
          detail={dry ? "Pick another filter to see the rest." : "Turn on Dry runs, or pick another filter."}
        />
      ) : null}
      {shown.map((r) => (
        <RunRow key={r.id} routine={routine.id} run={r} onOpen={onOpen} />
      ))}
    </Section>
  );
}

/*
 * runTint is the colour of a run's mark: red when it failed, green when
 * it suggested something.
 */
function runTint(r: RoutineRun): "rose" | "lime" | "neutral" {
  if (r.status === "failed") {
    return "rose";
  }
  return r.outcome === "findings" ? "lime" : "neutral";
}

/*
 * RunRow is one run, opening it as it happens or as it went.
 */
function RunRow({ routine, run: r, onOpen }: { routine: string; run: RoutineRun; onOpen?: (run: string) => void }) {
  const live = r.status === "running" || r.status === "queued";
  const row = (
    <Item
      mark={<KindMark icon={r.trigger === "dry_run" ? RiFlaskLine : RiPlayLine} tint={runTint(r)} />}
      title={runLine(r)}
      detail={[triggers[r.trigger], formatDateTime(r.started_at ?? r.created_at), tookOf(r), readLine(r)]
        .filter(Boolean)
        .join(" · ")}
    >
      {live ? (
        <Chip variant="caption" color="blue">
          {r.status === "running" ? "Running" : "Queued"}
        </Chip>
      ) : null}
      {!live && r.suggestions_waiting > 0 ? (
        <Chip variant="caption" color="yellow">
          {r.suggestions_waiting === r.changesets_count
            ? `${r.suggestions_waiting} waiting for review`
            : `${r.suggestions_waiting} of ${r.changesets_count} waiting`}
        </Chip>
      ) : null}
    </Item>
  );
  const shape =
    "-ms-3 block ps-3 transition-colors duration-150 first:rounded-t-2xl last:rounded-b-2xl hover:bg-background-secondary-hover";
  if (onOpen) {
    return (
      <button
        type="button"
        onClick={() => onOpen(r.id)}
        className={cx(shape, "w-[calc(100%+0.75rem)] cursor-pointer text-start")}
      >
        {row}
      </button>
    );
  }
  return (
    <Link to="/d/routines/$routine/runs/$run" params={{ routine, run: r.id }} className={shape}>
      {row}
    </Link>
  );
}
