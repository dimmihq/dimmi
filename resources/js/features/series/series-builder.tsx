/*
 * One series: the canvas of its steps and the inspector of the one
 * picked, under a header with its name, where it stands, a switch
 * between building it and reading its results, and the buttons that
 * publish, pause or resume it. A draft has no results to read. Pausing
 * stops messages at once and keeps everyone where they are; resuming
 * carries on from there. On screens too narrow for both panes, the
 * inspector opens in a drawer when a card is picked.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Drawer } from "@/components/base/drawer/drawer";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { toast } from "@/components/base/toast/toast";
import { Panel } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";

import { statusLook } from "./looks";
import { findNode, onboarding, type SeriesNode, seriesById } from "./sample";
import { SeriesCanvas } from "./series-canvas";
import { SeriesInspector } from "./series-inspector";

const notYet = "This is a preview of the page. Nothing changed.";

/*
 * withoutStats strips what a series did, for one that never ran.
 */
function withoutStats(node: SeriesNode): SeriesNode {
  return { ...node, stats: undefined, next: node.next?.map(withoutStats) };
}

function Actions({ status }: { status: keyof typeof statusLook }) {
  const say = () => void toast.success(notYet);

  if (status === "draft") {
    return (
      <Button size="small" onClick={say}>
        Set live
      </Button>
    );
  }

  return (
    <>
      <Button variant="secondary" size="small" onClick={say}>
        {status === "live" ? "Pause" : "Resume"}
      </Button>
      <Button size="small" disabled onClick={say}>
        Publish changes
      </Button>
    </>
  );
}

const modes = [
  ["build", "Build"],
  ["results", "Results"],
] as const;

function Mode({ results, onChange }: { results: boolean; onChange: (results: boolean) => void }) {
  return (
    <SegmentedControl
      aria-label="Show"
      selectedKeys={new Set([results ? "results" : "build"])}
      onSelectionChange={(keys) => onChange([...keys][0] === "results")}
    >
      {modes.map(([id, name]) => (
        <SegmentedControlItem key={id} id={id}>
          {name}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

export function SeriesBuilder({ id }: { id: string }) {
  const series = seriesById(id);
  const ran = series.status !== "draft";
  const root = ran ? onboarding : withoutStats(onboarding);
  const [selected, setSelected] = useState("entry");
  const [results, setResults] = useState(ran);
  const [drawer, setDrawer] = useState(false);
  const beside = useWide(1280);
  const node = findNode(root, selected) ?? root;
  const look = statusLook[series.status];
  const select = (next: string) => {
    setSelected(next);
    setDrawer(!beside);
  };
  const inspector = { node, series, results: results && ran };

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Series" ground className="h-full flex-1">
        <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-1 ps-2.5 pe-2.5 pt-[7px]">
          <div className="flex min-w-40 flex-1 items-center gap-2">
            <Link
              to="/d/series"
              aria-label="Back to series"
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
            </Link>
            <h1 className="min-w-0 truncate text-body-medium text-text-primary">{series.name}</h1>
            <Chip variant="caption" color={look.tint}>
              {look.name}
            </Chip>
            <span className="hidden truncate text-body-2-regular text-text-tertiary sm:inline">
              Edited {series.edited}
            </span>
          </div>
          <div className="ms-auto flex shrink-0 items-center gap-2">
            {ran ? <Mode results={results} onChange={setResults} /> : null}
            <Actions status={series.status} />
          </div>
        </header>
        <SeriesCanvas root={root} moves={{ selected, results: results && ran, onSelect: select }} />
      </Panel>
      {beside ? (
        <SeriesInspector {...inspector} className="h-full w-[380px] shrink-0" />
      ) : (
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Step settings"
          className="p-0"
        >
          <SeriesInspector {...inspector} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      )}
    </div>
  );
}
