/*
 * The inspector beside the series canvas: the settings of the selected
 * step, laid out like the workflow inspector. The entry step holds the
 * series' own settings; every other step its own, a list of what it
 * still misses, and a button that removes it. With results on, the
 * step's numbers unfold on top; picking another step brings its
 * settings in with a short rise. An archived series reads but no longer
 * changes.
 */
import { RiCloseLine, RiDeleteBinLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Section } from "@/features/settings/layout";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import type { NodeSettings, SeriesNode, SeriesProblem, SeriesReport, SeriesSpec } from "@/lib/api/series";

import { EntrySettings, type Sending } from "./entry-settings";
import { nodeLook } from "./looks";
import { removeStep, setSettings } from "./model";
import { Rise, Unfold } from "./motion";
import { StepResults } from "./step-results";
import { StepSettings } from "./step-settings";

function Problems({ problems }: { problems: SeriesProblem[] }) {
  if (problems.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-1 rounded-2xl bg-background-secondary-default p-3">
      <span className="text-body-2-medium text-text-error-primary">Before this can go live</span>
      {problems.map((p) => (
        <span key={`${p.code}:${p.field ?? ""}`} className="text-body-2-regular text-text-secondary">
          {p.message}
        </span>
      ))}
    </div>
  );
}

export function SeriesInspector({
  workspace,
  series,
  node,
  spec,
  sending,
  account,
  report,
  results,
  editable,
  problems,
  onSpec,
  onSending,
  onSelect,
  onClose,
  className,
}: {
  workspace: string;
  series: string;
  node: SeriesNode;
  spec: SeriesSpec;
  sending: Sending;
  account: boolean;
  report?: SeriesReport;
  results: boolean;
  editable: boolean;
  problems: SeriesProblem[];
  onSpec: (spec: SeriesSpec) => void;
  onSending: (sending: Sending) => void;
  onSelect: (id: string) => void;
  onClose?: () => void;
  className?: string;
}) {
  const mine = problems.filter((p) =>
    node.kind === "entry" ? !p.node_id || p.node_id === "entry" : p.node_id === node.id,
  );
  const set = (settings: NodeSettings) => onSpec(setSettings(spec, node.id, settings));

  return (
    <Panel label="Step settings" ground className={className}>
      <PageHeader title={nodeLook[node.kind].name} level={2}>
        {onClose ? (
          <IconButton size="small" icon={RiCloseLine} aria-label="Close step settings" onClick={onClose} />
        ) : null}
      </PageHeader>
      <ScrollArea ground className="px-4 pb-6">
        <Rise view={node.id} className="flex min-w-0 flex-col gap-6 pt-2">
          <Unfold shown={results}>
            <div className="flex flex-col gap-6 pb-2">
              <StepResults workspace={workspace} series={series} node={node} report={report} />
            </div>
          </Unfold>
          <Problems problems={mine} />
          <div key={node.id} className="flex min-w-0 flex-col gap-6">
            {node.kind === "entry" ? (
              <EntrySettings
                workspace={workspace}
                spec={spec}
                sending={sending}
                account={account}
                editable={editable}
                onSpec={onSpec}
                onSending={onSending}
              />
            ) : (
              <StepSettings
                workspace={workspace}
                series={series}
                node={node}
                spec={spec}
                account={account}
                editable={editable}
                set={set}
              />
            )}
          </div>
          {node.kind !== "entry" && editable ? (
            <Section title="Remove">
              <div className="py-3">
                <Button
                  size="small"
                  variant="danger-secondary"
                  leadingIcon={RiDeleteBinLine}
                  onClick={() => {
                    onSpec(removeStep(spec, node.id));
                    onSelect("entry");
                  }}
                >
                  Remove this step
                </Button>
              </div>
            </Section>
          ) : null}
        </Rise>
      </ScrollArea>
    </Panel>
  );
}
