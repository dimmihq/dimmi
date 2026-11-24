/*
 * A routine's run on a page of its own: back to the routine, what the
 * run is, and the run as it happens or as it went.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Chip } from "@/components/base/badges/chip";
import { useAccount } from "@/features/settings/use-account";
import { Panel } from "@/features/shell/page";
import { routineQuery, routineRunQuery } from "@/lib/api/routines";
import { formatDateTime } from "@/lib/i18n/format";

import { RunView } from "./run-view";
import { triggers } from "./runs";

export function RunPage({ routine, run }: { routine: string; run: string }) {
  const workspace = useAccount().workspace.id;
  const { data: owner } = useQuery(routineQuery(workspace, routine));
  const { data } = useQuery(routineRunQuery(workspace, routine, run));
  const live = data?.status === "running" || data?.status === "queued";

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
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">{owner?.name ?? "Run"}</h1>
        {live ? (
          <Chip variant="caption" color="blue">
            {data?.status === "running" ? "Running" : "Queued"}
          </Chip>
        ) : null}
        {data ? (
          <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
            {triggers[data.trigger]} · {formatDateTime(data.started_at ?? data.created_at)}
          </span>
        ) : null}
      </header>
      <RunView workspace={workspace} routine={routine} run={run} label={owner?.name ?? "Routine"} />
    </Panel>
  );
}
