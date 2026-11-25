/*
 * The series' Preview: the working copy read back in words. What still
 * stops it from going live comes first, and picking a problem opens the
 * step it is about. Then each step in the order people reach it.
 */
import { RiErrorWarningLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { KindMark } from "@/features/automation/parts";
import { ScrollArea } from "@/features/shell/page";
import type { Series, SeriesNode, SeriesProblem, SeriesSpec } from "@/lib/api/series";

import { nodeLook } from "./looks";
import { summary } from "./model";

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

export function SeriesPreview({
  series,
  spec,
  onShow,
}: {
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
      </div>
    </ScrollArea>
  );
}
