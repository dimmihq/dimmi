/*
 * Operator on Home: what each routine last said, in its own words, the
 * suggestions that wait for a teammate, and, before any routine exists,
 * three worth starting. A routine's card opens the thread of its run.
 */
import {
  RiArrowRightSLine,
  RiBookOpenLine,
  RiCalendarScheduleLine,
  RiChat3Line,
  RiListOrdered2,
  RiRobot2Line,
} from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import type { ComponentType } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { ago } from "@/features/inbox/conversations";
import { MessageBody } from "@/features/inbox/message-body";
import { ButtonLink } from "@/features/navigation/links";
import { RichLink } from "@/features/operator/rich-link";
import { Glyph, Section } from "@/features/settings/layout";
import { type OperatorChangeset, operatorThreadQuery, pendingChangesetsQuery } from "@/lib/api/operator";
import {
  type Routine,
  type RoutineRun,
  routineRunsQuery,
  routinesQuery,
  routineTemplatesQuery,
} from "@/lib/api/routines";
import { callerQuery } from "@/lib/api/workspace";

import { told } from "./figures";
import { Blank, Card, Row, Rows, type Where } from "./rows";

const reportText = "gap-3 text-body-regular text-text-primary [&_li+li]:mt-1.5 [&_li>p]:inline";

/*
 * clamp keeps a long report to its opening lines, fading out where it is
 * cut; a short one shows whole. The card opens the rest.
 */
const clamp = "max-h-[84px] overflow-hidden [mask-image:linear-gradient(to_bottom,black_48px,transparent_84px)]";

const arrow = <RiArrowRightSLine aria-hidden className="size-5 text-foreground-icon-tertiary rtl:-scale-x-100" />;

/* How many routines' reports Home shows, and how many suggestions. */
const shown = 3;

/* The routines a workspace without one is offered first, in this order. */
const starters = ["tpl_unhappy_customer_watch", "tpl_knowledge_gap_finder", "tpl_qa_procedure_digest"];

/*
 * Report is one routine's latest run: what Operator wrote, or why the
 * run failed. It shows nothing while a routine has never finished a run.
 */
function Report({ workspace, routine }: { workspace: string; routine: Routine }) {
  const { data: runs } = useQuery(routineRunsQuery(workspace, routine.id));
  const run = (runs ?? []).find((r) => r.thread_id && (r.status === "completed" || r.status === "failed"));
  const thread = useQuery({
    ...operatorThreadQuery(workspace, run?.thread_id ?? ""),
    enabled: Boolean(run?.thread_id),
  });

  if (runs === undefined || (run && thread.isPending)) {
    return <Skeleton className="h-[132px] rounded-2xl" />;
  }
  if (!run?.thread_id) {
    return null;
  }
  const said = [...(thread.data?.messages ?? [])].reverse().find((m) => m.role === "assistant" && m.body);

  return (
    <Card
      where={{ to: "/d/routines/$routine/threads/$thread", params: { routine: routine.id, thread: run.thread_id } }}
      mark={<Glyph icon={RiRobot2Line} />}
      title={routine.name}
      detail={ago(run.completed_at ?? run.created_at)}
      aside={<Marks run={run} />}
    >
      {run.status === "failed" ? (
        <p className="text-body-regular text-text-secondary">
          This run failed{run.error_message ? `: ${run.error_message}` : "."}
        </p>
      ) : (
        <div className={clamp}>
          <MessageBody text={said?.body ?? ""} link={RichLink} className={reportText} />
        </div>
      )}
    </Card>
  );
}

/*
 * Marks is what a report's heading ends with: that it was a dry run or
 * failed, how many of its suggestions still wait, and the arrow.
 */
function Marks({ run }: { run: RoutineRun }) {
  return (
    <>
      {run.status === "failed" ? (
        <Chip variant="caption" color="rose" className="py-0">
          Failed
        </Chip>
      ) : null}
      {run.trigger === "dry_run" ? (
        <Chip variant="caption" color="neutral" className="py-0">
          Dry run
        </Chip>
      ) : null}
      {run.suggestions_waiting > 0 ? (
        <Chip variant="caption" color="blue" className="py-0">
          {run.suggestions_waiting === 1 ? "1 suggestion" : `${run.suggestions_waiting} suggestions`}
        </Chip>
      ) : null}
      {arrow}
    </>
  );
}

/*
 * iconOf is the mark of a suggestion: what kind of thing it changes.
 */
function iconOf(changeset: OperatorChangeset): ComponentType<{ className?: string }> {
  const kind = changeset.operations[0]?.kind ?? "";
  if (kind.startsWith("procedure")) {
    return RiListOrdered2;
  }

  return kind.startsWith("article") ? RiBookOpenLine : RiChat3Line;
}

/*
 * whereOf is where a suggestion is reviewed: on its routine's thread, or
 * among all that wait.
 */
function whereOf(changeset: OperatorChangeset): Where {
  if (changeset.routine_id && changeset.thread_id) {
    return {
      to: "/d/routines/$routine/threads/$thread",
      params: { routine: changeset.routine_id, thread: changeset.thread_id },
    };
  }

  return { to: "/d/operator/$tab", params: { tab: "suggestions" } };
}

/*
 * Waiting is the suggestions nobody has applied or declined yet, newest
 * first. It leaves the page when none wait.
 */
function Waiting({ workspace, routines }: { workspace: string; routines: Routine[] }) {
  const { data } = useQuery(pendingChangesetsQuery(workspace));
  const pending = (data ?? []).filter((changeset) => !changeset.preview);
  if (pending.length === 0) {
    return null;
  }
  const names = new Map(routines.map((routine) => [routine.id, routine.name]));

  return (
    <Section
      title="Waiting for your review"
      description="Nothing Operator suggests applies until a teammate approves it."
      bare
      action={
        <ButtonLink variant="tertiary" size="medium" to="/d/operator/suggestions">
          Review all {pending.length}
        </ButtonLink>
      }
    >
      <Rows>
        {pending.slice(0, shown).map((changeset) => (
          <Row
            key={changeset.id}
            where={whereOf(changeset)}
            mark={<Glyph icon={iconOf(changeset)} />}
            title={changeset.title}
            detail={[
              changeset.operations[0]?.label,
              names.get(changeset.routine_id ?? "") ?? "Operator",
              ago(changeset.created_at),
            ]
              .filter(Boolean)
              .join(" · ")}
          >
            {arrow}
          </Row>
        ))}
      </Rows>
    </Section>
  );
}

/*
 * Starters stands where Operator's reports will be, before any routine
 * exists: three routines worth starting first, each one click from its
 * setup page, so the block says what it is for instead of sitting empty.
 */
function Starters({ workspace }: { workspace: string }) {
  const { data } = useQuery(routineTemplatesQuery(workspace));
  const templates = starters.flatMap((key) => (data ?? []).filter((template) => template.key === key));

  return (
    <Section
      title="Put Operator to work"
      description="Operator is your team's own agent. Start a routine and what it finds shows here."
      bare
      action={
        <ButtonLink variant="tertiary" size="medium" to="/d/operator/routines">
          All templates
        </ButtonLink>
      }
    >
      <Rows>
        {templates.map((template) => (
          <Row
            key={template.key}
            where={{ to: "/d/routines/new", search: { template: template.key } }}
            mark={<Glyph icon={RiCalendarScheduleLine} />}
            title={template.name}
            detail={template.description}
          >
            <span className="text-body-2-medium text-text-secondary">Start</span>
            {arrow}
          </Row>
        ))}
        {data && templates.length === 0 ? <Blank>No routine templates are on offer here.</Blank> : null}
      </Rows>
    </Section>
  );
}

/*
 * HomeOperator is Operator's part of Home. A teammate sees the reports
 * of the routines that tell them about their runs, and no others: a
 * routine's findings are for the people it was set to tell. A teammate
 * who may not read routines is shown none of it.
 */
export function HomeOperator({ workspace }: { workspace: string }) {
  const on = { enabled: workspace !== "", retry: false } as const;
  const { data: routines, isError } = useQuery({ ...routinesQuery(workspace), ...on });
  const { data: caller } = useQuery({ ...callerQuery(workspace), ...on });
  if (isError) {
    return null;
  }
  if (routines === undefined || caller === undefined) {
    return <Section title="From Operator" loading rows={3} />;
  }
  if (routines.length === 0) {
    return <Starters workspace={workspace} />;
  }
  const mine = routines.filter((routine) => told(routine, caller.member?.id ?? ""));
  const ran = mine
    .filter((routine) => routine.last_run_at)
    .sort((a, b) => (b.last_run_at ?? "").localeCompare(a.last_run_at ?? ""))
    .slice(0, shown);

  return (
    <>
      <Section
        title="From Operator"
        description="What the routines that report to you last found."
        bare
        action={
          <ButtonLink variant="tertiary" size="medium" to="/d/operator/activity">
            See all
          </ButtonLink>
        }
      >
        {ran.length === 0 ? (
          <Rows>
            <Blank>
              {mine.length === 0
                ? "No routine reports to you. Add yourself to one under who hears about it."
                : "Your routines have not run yet. What they find shows here."}
            </Blank>
          </Rows>
        ) : (
          <div className="flex flex-col gap-2">
            {ran.map((routine) => (
              <Report key={routine.id} workspace={workspace} routine={routine} />
            ))}
          </div>
        )}
      </Section>
      <Waiting workspace={workspace} routines={routines} />
    </>
  );
}
