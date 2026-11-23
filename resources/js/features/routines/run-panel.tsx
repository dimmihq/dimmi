/*
 * A routine's run in the Operator panel beside the routine: the same
 * panel a teammate talks to Operator in, showing the run as it works
 * and, once it ends, its message and suggestions. The past chats menu
 * lists the routine's other runs.
 */
import { RiCalendarScheduleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { OperatorSidePanel } from "@/features/operator/operator-side-panel";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { operatorThreadQuery } from "@/lib/api/operator";
import { type Routine, routineRunQuery, routineRunsQuery } from "@/lib/api/routines";
import { formatDateTime } from "@/lib/i18n/format";

import { runLine, triggers } from "./runs";
import { useRunLive } from "./use-run-live";

export function RunPanel({
  workspace,
  routine,
  run,
  onPick,
  onClose,
}: {
  workspace: string;
  routine: Routine;
  run: string;
  onPick: (run: string) => void;
  onClose: () => void;
}) {
  const { data } = useQuery(routineRunQuery(workspace, routine.id, run));
  const { data: runs = [] } = useQuery(routineRunsQuery(workspace, routine.id));
  const query = operatorThreadQuery(workspace, data?.thread_id ?? "");
  const { data: thread } = useQuery({ ...query, enabled: Boolean(data?.thread_id) });
  const chat = useOperatorChat(workspace, thread, query.queryKey);
  const live = useRunLive(workspace, data);
  const past = runs
    .filter((r) => r.thread_id)
    .map((r) => ({
      id: r.thread_id ?? "",
      conversation_id: null,
      title: `${triggers[r.trigger]} · ${formatDateTime(r.started_at ?? r.created_at)}`,
      last_message_at: r.completed_at,
      created_at: r.created_at,
    }));

  return (
    <OperatorSidePanel
      thread={thread}
      chat={live ? { ...chat, live, busy: true } : chat}
      past={past}
      enabled
      starting={false}
      heading={routine.name}
      blurb={data ? runLine(data) : "Opening the run."}
      suggestions={[]}
      placeholder={live ? "Operator is still working" : "Ask Operator about this run"}
      inputId={`routine-run-panel-${run}`}
      scope={{ icon: RiCalendarScheduleLine, label: routine.name }}
      instructions={routine.name}
      onPick={(picked) => {
        const found = runs.find((r) => r.thread_id === picked);
        if (found) {
          onPick(found.id);
        }
      }}
      onClose={onClose}
      className="h-full w-[420px] shrink-0"
    />
  );
}
