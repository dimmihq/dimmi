/*
 * One run of a routine, as the thread Operator wrote it on: the prompt
 * it ran, each step it took, its report and the changes it suggested,
 * each to apply or decline right here. Any teammate who may read the
 * routine can reply to ask for more; Operator answers as that teammate.
 */
import { RiArrowLeftSLine, RiCalendarScheduleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { OperatorChat } from "@/features/operator/operator-chat";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import { useAccount } from "@/features/settings/use-account";
import { Panel } from "@/features/shell/page";
import { operatorThreadQuery } from "@/lib/api/operator";
import { routineQuery } from "@/lib/api/routines";

export function RoutineThread({ routine, thread }: { routine: string; thread: string }) {
  const workspace = useAccount().workspace.id;
  const query = operatorThreadQuery(workspace, thread);
  const { data, isError } = useQuery(query);
  const { data: owner } = useQuery(routineQuery(workspace, routine));
  const chat = useOperatorChat(workspace, data, query.queryKey);

  return (
    <Panel label="Routine run" ground className="h-full flex-1">
      <header className="flex h-12 shrink-0 items-center gap-2 ps-2.5 pe-2.5 pt-[7px]">
        <Link
          to="/d/routines/$routine"
          params={{ routine }}
          aria-label="Back to the routine"
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </Link>
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{data?.title ?? owner?.name ?? "Run"}</h1>
      </header>
      {isError ? (
        <p className="m-auto p-8 text-body-2-regular text-text-secondary">
          This run is not here, or you may not read what its routine reads.
        </p>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <OperatorChat
            key={data?.id ?? "opening"}
            thread={data}
            chat={chat}
            wide
            empty={null}
            placeholder="Ask Operator about this run"
            inputId="routine-thread-input"
            scope={{ icon: RiCalendarScheduleLine, label: owner?.name ?? "Routine" }}
          />
        </div>
      )}
    </Panel>
  );
}
