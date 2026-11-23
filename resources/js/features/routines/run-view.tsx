/*
 * One run of a routine as it happens, or as it went: waiting to start,
 * skipped and why, or the thread Operator writes on, with its steps and
 * text growing in place while it works. A run opened midway shows the
 * steps it took so far, then the rest as they come. Once it ends, the
 * thread holds its report and the changes it suggested; any teammate
 * who may read the routine can reply to ask for more.
 */
import { RiCalendarScheduleLine, RiLoader4Line, RiSkipForwardLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { OperatorChat } from "@/features/operator/operator-chat";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { operatorThreadQuery } from "@/lib/api/operator";
import { type RoutineRun, routineRunQuery } from "@/lib/api/routines";

import { runLine } from "./runs";
import { useRunLive } from "./use-run-live";

function Waiting({ run }: { run: RoutineRun }) {
  const skipped = run.status === "skipped";
  const Icon = skipped ? RiSkipForwardLine : RiLoader4Line;

  return (
    <div className="m-auto flex max-w-80 flex-col items-center gap-2 p-8 text-center">
      <Icon
        aria-hidden
        className={
          skipped ? "size-6 text-foreground-icon-tertiary" : "size-6 animate-spin text-foreground-icon-tertiary"
        }
      />
      <p className="text-body-medium text-text-primary">{runLine(run)}</p>
      <p className="text-body-2-regular text-text-secondary">
        {skipped
          ? "This run did not happen, so it has no thread. The next one runs on schedule."
          : "Operator picks it up in a moment. Its steps show here as it works."}
      </p>
    </div>
  );
}

function Thread({ workspace, run, label }: { workspace: string; run: RoutineRun; label: string }) {
  const query = operatorThreadQuery(workspace, run.thread_id ?? "");
  const { data } = useQuery({ ...query, enabled: run.thread_id !== null });
  const chat = useOperatorChat(workspace, data, query.queryKey);
  const live = useRunLive(workspace, run);
  const shown = live ? { ...chat, live, busy: true } : chat;

  return (
    <OperatorChat
      key={data?.id ?? "opening"}
      thread={data}
      chat={shown}
      wide
      instructions={label}
      empty={null}
      placeholder={live ? "Operator is still working" : "Ask Operator about this run"}
      inputId={`routine-run-${run.id}`}
      scope={{ icon: RiCalendarScheduleLine, label }}
    />
  );
}

export function RunView({
  workspace,
  routine,
  run,
  label,
}: {
  workspace: string;
  routine: string;
  run: string;
  label: string;
}) {
  const { data, isError } = useQuery(routineRunQuery(workspace, routine, run));

  if (isError) {
    return (
      <p className="m-auto p-8 text-body-2-regular text-text-secondary">
        This run is not here, or you may not read what its routine reads.
      </p>
    );
  }
  if (!data) {
    return null;
  }
  if (!data.thread_id) {
    return <Waiting run={data} />;
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <Thread workspace={workspace} run={data} label={label} />
    </div>
  );
}
