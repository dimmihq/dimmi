/*
 * The workflow builder: the canvas of steps on the left and the
 * inspector for the selected step on the right, under a header with the
 * workflow's name, its status and version, and the button that publishes
 * or pauses it. Publishing makes a new version; changes stay in this
 * page until workflows are saved on the server. On screens too narrow
 * for both, the inspector opens in a drawer when a card is picked, or
 * from the header.
 */
import { RiArrowLeftSLine, RiSettings3Line } from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Panel } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";

import { Canvas } from "./canvas";
import { freshStep, withEdited, withoutStep, withStep } from "./edit-steps";
import { Inspector } from "./inspector";
import { StatusChip } from "./parts";
import { findStep, type Step, type StepKind, triggerStep, type Workflow } from "./workflows";

function useDraft(workflow: Workflow) {
  const [steps, setSteps] = useState(workflow.steps);
  const [status, setStatus] = useState(workflow.status);
  const [version, setVersion] = useState(workflow.version);
  const made = useRef(0);

  return {
    steps,
    status,
    version,
    add: (path: string | null, kind: StepKind) => {
      made.current += 1;
      const id = `new-${made.current}`;
      setSteps((before) => withStep(before, path, freshStep(kind, id)));

      return id;
    },
    remove: (id: string) => setSteps((before) => withoutStep(before, id)),
    edit: (id: string, edit: (step: Step) => Step) => setSteps((before) => withEdited(before, id, edit)),
    publish: () => {
      setStatus("live");
      setVersion((before) => before + 1);
    },
    pause: () => setStatus("paused"),
  };
}

export function WorkflowBuilder({ workflow }: { workflow: Workflow }) {
  const draft = useDraft(workflow);
  const [selected, setSelected] = useState<string>(triggerStep);
  const step = selected === triggerStep ? undefined : findStep(draft.steps, selected);
  const beside = useWide(1280);
  const [drawer, setDrawer] = useState(false);
  const select = (id: string) => {
    setSelected(id);
    setDrawer(!beside);
  };
  const remove = (id: string) => {
    draft.remove(id);
    setSelected(triggerStep);
    setDrawer(false);
  };

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Workflow" ground className="h-full flex-1">
        <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-0 items-center gap-2">
            <Link
              to="/d/workflows"
              aria-label="Back to workflows"
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftSLine aria-hidden className="size-5" />
            </Link>
            <h1 className="min-w-0 truncate text-body-medium text-text-primary">{workflow.name}</h1>
            <StatusChip status={draft.status} />
            <span className="hidden text-body-2-regular text-text-tertiary sm:inline">Version {draft.version}</span>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {beside ? null : (
              <IconButton
                size="small"
                icon={RiSettings3Line}
                aria-label="Step settings"
                onClick={() => setDrawer(true)}
              />
            )}
            {draft.status === "live" ? (
              <Button variant="secondary" size="small" onClick={draft.pause}>
                Pause
              </Button>
            ) : (
              <Button size="small" onClick={draft.publish}>
                Publish
              </Button>
            )}
          </div>
        </header>
        <Canvas
          trigger={workflow.trigger}
          steps={draft.steps}
          moves={{ selected, onSelect: select, onAdd: (path, kind) => select(draft.add(path, kind)) }}
        />
      </Panel>
      {beside ? (
        <Inspector
          trigger={workflow.trigger}
          step={step}
          onRemove={remove}
          onEdit={draft.edit}
          className="h-full w-[360px] shrink-0"
        />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Step settings"
          className="p-0"
        >
          <Inspector
            trigger={workflow.trigger}
            step={step}
            onRemove={remove}
            onEdit={draft.edit}
            onClose={() => setDrawer(false)}
            className="h-full"
          />
        </Drawer>
      )}
    </div>
  );
}
