/*
 * Trying a series on yourself: a test journey for the contact with your
 * email, shown step by step as it runs in the studio's Preview, with a
 * button that ends the wait it stands on so a week-long series runs
 * through in a minute. Messages really go out, to you. Test journeys
 * never count in the report.
 */
import { RiSkipForwardLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { KindMark } from "@/features/automation/parts";
import { type SeriesSpec, seriesTraceQuery, skipSeriesWait } from "@/lib/api/series";
import { formatTime } from "@/lib/i18n/format";

import { endReasonName, nodeLook } from "./looks";
import { summary } from "./model";

export function TryJourney({
  workspace,
  series,
  journey,
  spec,
  onDone,
}: {
  workspace: string;
  series: string;
  journey: string;
  spec: SeriesSpec;
  onDone: () => void;
}) {
  const trace = useQuery(seriesTraceQuery(workspace, series, journey)).data;
  const skip = useMutation({ mutationFn: () => skipSeriesWait(workspace, series, journey) });
  const steps = trace?.steps ?? [];
  const done = trace?.status === "finished" || trace?.status === "exited";

  return (
    <section aria-label="Your test journey" className="flex flex-col gap-2">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-body-2-medium text-text-secondary">Your test journey</h3>
        <p className="text-body-2-regular text-text-tertiary">
          {done
            ? `Ended: ${endReasonName[trace?.end_reason ?? ""] ?? trace?.end_reason}.`
            : trace?.status === "waiting"
              ? "Waiting on a step. Skip the wait to move on now."
              : "Running…"}
        </p>
      </div>
      <ol className="flex flex-col gap-2">
        {steps.map((s) => {
          const node = spec.nodes.find((n) => n.id === s.node_id);
          const look = nodeLook[s.kind];

          return (
            <li key={`${s.node_id}:${s.entered_at}`} className="flex items-start gap-2.5">
              <KindMark icon={look.icon} tint={look.tint} size="sm" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-body-2-medium text-text-primary">
                  {node ? summary(node, spec) : s.node_id}
                </span>
                <span className="truncate text-caption-1-regular text-text-tertiary">
                  {s.status === "waiting" ? "Waiting" : (s.outcome ?? s.path_id ?? "Done")} · {formatTime(s.entered_at)}
                </span>
              </span>
            </li>
          );
        })}
      </ol>
      <div className="flex items-center gap-2 pt-1">
        {done ? null : (
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiSkipForwardLine}
            disabled={trace?.status !== "waiting"}
            loading={skip.isPending}
            onClick={() => skip.mutate()}
          >
            Skip the wait
          </Button>
        )}
        <Button size="small" variant="tertiary" onClick={onDone}>
          {done ? "Clear" : "Hide"}
        </Button>
      </div>
    </section>
  );
}
