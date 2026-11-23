/*
 * A new routine, filled from the template picked in the gallery: the
 * builder's sections, and Create once nothing stands in the way. A dry
 * run is offered right after, from the routine's own page.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import { createRoutine, type RoutineTemplate, routineKeys, routineTemplatesQuery } from "@/lib/api/routines";
import { callerQuery, workspaceQuery } from "@/lib/api/workspace";

import { createInputOf, draftFromTemplate, problems, type RoutineDraft } from "./draft";
import { RoutineFields } from "./routine-fields";

export function RoutineNew({ template }: { template: string }) {
  const workspace = useAccount().workspace.id;
  const { data: templates } = useQuery(routineTemplatesQuery(workspace));
  const { data: ws } = useQuery(workspaceQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const picked = templates?.find((t) => t.key === template) ?? templates?.find((t) => t.key === "tpl_custom");

  if (!picked || !ws || !caller?.member) {
    return (
      <Panel label="New routine" ground className="h-full flex-1">
        {null}
      </Panel>
    );
  }
  return <Builder workspace={workspace} template={picked} timezone={ws.timezone || "UTC"} me={caller.member.id} />;
}

function Builder({
  workspace,
  template,
  timezone,
  me,
}: {
  workspace: string;
  template: RoutineTemplate;
  timezone: string;
  me: string;
}) {
  const [draft, setDraft] = useState<RoutineDraft>(() => draftFromTemplate(template, timezone, me));
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const blocking = problems(draft);
  const create = useMutation({
    mutationFn: () => createRoutine(workspace, createInputOf(draft)),
    onSuccess: (r) => {
      void queryClient.invalidateQueries({ queryKey: routineKeys.all(workspace) });
      toast.success(`${r.name} is on. Try a dry run to see what it finds.`);
      void navigate({ to: "/d/routines/$routine", params: { routine: r.id } });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Panel label="New routine" ground className="h-full flex-1">
      <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <Link
            to="/d/routines"
            aria-label="Back to routines"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
          </Link>
          <h1 className="min-w-0 truncate text-body-medium text-text-primary">{draft.name.trim() || "New routine"}</h1>
          <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
            {template.key === "tpl_custom" ? "From scratch" : `From ${template.name}`}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          {blocking.length > 0 ? (
            <span className="hidden text-body-2-regular text-text-tertiary md:inline">{blocking[0]}</span>
          ) : null}
          <Button
            size="small"
            disabled={blocking.length > 0}
            loading={create.isPending}
            onClick={() => create.mutate()}
          >
            Create routine
          </Button>
        </div>
      </header>
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[760px] flex-col gap-8 px-4 pt-6 pb-16 sm:px-6">
          <RoutineFields workspace={workspace} draft={draft} disabled={false} onChange={setDraft} />
        </div>
      </ScrollArea>
    </Panel>
  );
}
