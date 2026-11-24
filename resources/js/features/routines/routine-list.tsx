/*
 * The routines page: what Operator does on its own, when each routine
 * runs next, how its last run went and what it spent this month. A new
 * routine starts from the template gallery: four routines that have
 * proved themselves, or a blank one. Teammates who may only read see
 * the list without the button.
 */
import { RiAddLine, RiCalendarScheduleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { KindMark, ListPage } from "@/features/automation/parts";
import { usePermission } from "@/features/settings/inboxes";
import { Item, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { type Routine, type RoutineTemplate, routinesQuery, routineTemplatesQuery } from "@/lib/api/routines";
import { formatRelative } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { lookOf, needs } from "./looks";
import { describeRoutine, summarize, timingOf } from "./timing";

/*
 * outcomeChip says how the last run went.
 */
export function OutcomeChip({ routine }: { routine: Routine }) {
  if (routine.status === "paused") {
    return (
      <Chip variant="caption" color="yellow">
        Paused
      </Chip>
    );
  }
  if (routine.status === "completed") {
    return (
      <Chip variant="caption" color="neutral">
        Done
      </Chip>
    );
  }
  const look = {
    findings: { color: "lime", text: "Suggested changes" },
    noop: { color: "neutral", text: "Nothing to change" },
    failed: { color: "rose", text: "Failed" },
    skipped: { color: "neutral", text: "Skipped" },
  } as const;
  const last = routine.last_outcome ? look[routine.last_outcome] : { color: "blue" as const, text: "Not run yet" };
  return (
    <Chip variant="caption" color={last.color}>
      {last.text}
    </Chip>
  );
}

function detailOf(r: Routine): string {
  const next = r.status === "active" && r.next_run_at ? `next ${formatRelative(r.next_run_at)}` : null;
  return [describeRoutine(r), next].filter(Boolean).join(" · ");
}

export function RoutineList() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "ai_agent:manage");
  const { data, isPending } = useQuery(routinesQuery(workspace));
  const [gallery, setGallery] = useState(false);

  return (
    <ListPage
      title="Routines"
      description="Operator on a clock: it reads what changed, suggests fixes and tells the team, without anyone asking."
      action={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setGallery(true)}>
            New routine
          </Button>
        ) : null
      }
    >
      {data?.length === 0 && canManage ? (
        <Section title="Start from a template" bare>
          <Templates workspace={workspace} />
        </Section>
      ) : (
        <Section title="Routines" loading={isPending}>
          {data?.length === 0 ? (
            <Item
              mark={<KindMark icon={RiCalendarScheduleLine} tint="neutral" />}
              title="No routines yet"
              detail="An admin sets them up."
            />
          ) : null}
          {(data ?? []).map((r) => {
            const look = lookOf(r.template_key);
            return (
              <Link
                key={r.id}
                to="/d/routines/$routine"
                params={{ routine: r.id }}
                className={cx(
                  "-ms-3 flex min-h-[60px] items-center gap-3 border-b border-separator-border py-2.5 ps-3 pe-2.5 outline-none last:border-b-0",
                  "transition-colors duration-150 first:rounded-t-2xl last:rounded-b-2xl hover:bg-background-secondary-hover",
                  "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-inset",
                )}
              >
                <KindMark icon={look.icon} tint={look.tint} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-body-regular text-text-primary">{r.name}</span>
                  <span className="truncate text-body-2-regular text-text-secondary">{detailOf(r)}</span>
                </span>
                {r.suggestions.pending > 0 ? (
                  <span className="hidden shrink-0 text-body-2-regular text-text-tertiary sm:inline">
                    {`${r.suggestions.pending.toLocaleString()} waiting for review`}
                  </span>
                ) : null}
                <OutcomeChip routine={r} />
              </Link>
            );
          })}
        </Section>
      )}
      <Modal isOpen={gallery} onOpenChange={setGallery} size="wide">
        <ModalHeader title="New routine" description="Pick what Operator should do. You can change anything next." />
        <ModalBody>
          <Templates workspace={workspace} />
        </ModalBody>
      </Modal>
    </ListPage>
  );
}

/*
 * Templates is the gallery: each template's mark, what it does, when it
 * runs and what it needs, opening the builder filled from it.
 */
export function Templates({ workspace }: { workspace: string }) {
  const { data } = useQuery(routineTemplatesQuery(workspace));
  const navigate = useNavigate();

  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
      {(data ?? []).map((t: RoutineTemplate) => {
        const look = lookOf(t.key);
        const blank = t.key === "tpl_custom";
        return (
          <button
            key={t.key}
            type="button"
            onClick={() => void navigate({ to: "/d/routines/new", search: { template: t.key } })}
            className={cx(
              "flex flex-col items-start gap-3 rounded-2xl bg-background-secondary-default p-4 text-start outline-none",
              "transition-colors duration-150 hover:bg-background-secondary-hover",
              "focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              blank && "sm:col-span-2",
            )}
          >
            <span className="flex w-full items-center gap-3">
              <KindMark icon={look.icon} tint={look.tint} />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="text-body-medium text-text-primary">{t.name}</span>
                {blank ? null : (
                  <span className="text-body-2-regular text-text-tertiary">
                    {summarize(timingOf(t.cadence, t.cron_expression, null, "UTC"))}
                  </span>
                )}
              </span>
            </span>
            <span className="text-body-2-regular text-text-secondary">{t.description}</span>
            {t.requires.length > 0 ? (
              <span className="flex flex-wrap gap-1">
                {t.requires.map((r) => (
                  <Chip key={r} variant="caption" color={r === "github" ? "purple" : "neutral"}>
                    {needs[r] ?? r}
                  </Chip>
                ))}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
