/*
 * The series' Preview: the working copy read back in words. What still
 * stops it from going live comes first, and picking a problem opens the
 * step it is about. Then each step in the order people reach it, and
 * "Try it on me", which runs the published version on yourself and
 * follows the journey here as it moves.
 */
import { RiErrorWarningLine, RiTestTubeLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { KindMark } from "@/features/automation/parts";
import { ScrollArea } from "@/features/shell/page";
import { type Series, type SeriesNode, type SeriesProblem, type SeriesSpec, trySeries } from "@/lib/api/series";

import { nodeLook } from "./looks";
import { summary } from "./model";
import { TryJourney } from "./try-panel";

function Block({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-2">
      <h3 className="text-body-2-medium text-text-secondary">{title}</h3>
      {children}
    </section>
  );
}

/*
 * reachOrder lists the steps from the entry along every path, each once,
 * then any step no path leads to.
 */
function reachOrder(spec: SeriesSpec): SeriesNode[] {
  const byId = new Map(spec.nodes.map((n) => [n.id, n] as const));
  const seen = new Set<string>();
  const order: SeriesNode[] = [];
  const queue = [spec.nodes.find((n) => n.kind === "entry")?.id ?? spec.nodes[0]?.id];
  while (queue.length > 0) {
    const id = queue.shift();
    const node = id ? byId.get(id) : undefined;
    if (!id || !node || seen.has(id)) {
      continue;
    }
    seen.add(id);
    order.push(node);
    queue.push(...spec.edges.filter((e) => e.from === id).map((e) => e.to));
  }

  return [...order, ...spec.nodes.filter((n) => !seen.has(n.id))];
}

function TryIt({ workspace, series, spec }: { workspace: string; series: Series; spec: SeriesSpec }) {
  const [asking, setAsking] = useState(false);
  const [journey, setJourney] = useState<string | null>(null);
  const start = useMutation({
    mutationFn: () => trySeries(workspace, series.id),
    onSuccess: (j) => setJourney(j.id),
    onError: (error) => void toast.error(error instanceof Error ? error.message : "That did not work."),
  });
  const live = series.status === "live";

  return (
    <Block title="Try it on me">
      <p className="text-body-2-regular text-text-secondary">
        {live
          ? "Run the published version on yourself. Its messages really go out, to your email."
          : "Set the series live to run it on yourself."}
      </p>
      {live ? (
        <div>
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiTestTubeLine}
            loading={start.isPending}
            onClick={() => setAsking(true)}
          >
            Try it on me
          </Button>
        </div>
      ) : null}
      <AlertDialog
        isOpen={asking}
        onOpenChange={setAsking}
        isDismissable
        variant="default"
        title="Try this series on yourself?"
        description={
          "You enter the published version as a test journey, under the contact with your email. Its messages really " +
          "go out, to you. Rules, quiet hours and the daily cap do not hold you back, you can skip each wait, and the " +
          "report never counts you."
        }
        confirmLabel="Start test"
        onConfirm={() => start.mutate()}
      />
      {journey ? (
        <TryJourney
          key={journey}
          workspace={workspace}
          series={series.id}
          journey={journey}
          spec={spec}
          onDone={() => setJourney(null)}
        />
      ) : null}
    </Block>
  );
}

export function SeriesPreview({
  workspace,
  series,
  spec,
  onShow,
}: {
  workspace: string;
  series: Series;
  spec: SeriesSpec;
  onShow: (problem: SeriesProblem) => void;
}) {
  return (
    <ScrollArea className="px-4 pb-6">
      <div className="flex min-w-0 flex-col gap-6 pt-2">
        {series.problems.length > 0 ? (
          <Block title="To fix before going live">
            <ul className="flex flex-col">
              {series.problems.map((problem) => (
                <li key={`${problem.code}-${problem.node_id ?? ""}-${problem.field ?? ""}`}>
                  <button
                    type="button"
                    onClick={() => onShow(problem)}
                    className="-mx-2 flex w-[calc(100%+1rem)] cursor-pointer items-start gap-2 rounded-lg px-2 py-1.5 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                  >
                    <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0 text-text-error-primary" />
                    <span className="text-body-2-regular text-text-primary">{problem.message}</span>
                  </button>
                </li>
              ))}
            </ul>
          </Block>
        ) : null}
        <Block title="What it does">
          <ol className="flex flex-col gap-2">
            {reachOrder(spec).map((node) => {
              const look = nodeLook[node.kind];

              return (
                <li key={node.id} className="flex items-start gap-2.5">
                  <KindMark icon={look.icon} tint={look.tint} size="sm" />
                  <span className="min-w-0 pt-0.5 text-body-2-regular text-text-primary">{summary(node, spec)}</span>
                </li>
              );
            })}
          </ol>
        </Block>
        <TryIt workspace={workspace} series={series} spec={spec} />
      </div>
    </ScrollArea>
  );
}
