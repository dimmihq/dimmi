/*
 * Writing one message of a series on a page of its own, in the same
 * writer broadcasts use: the subject or title as the heading, the
 * preview line under it for email, who it comes from, then the message.
 * Every change saves to the series' working copy a moment after the
 * last keystroke; people on the series get it once the series is
 * published. Back returns to the builder with this step picked and its
 * settings open, where the teammate left it.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { useActiveMembers } from "@/features/broadcasts/sources";
import { WriteStep, type Written } from "@/features/broadcasts/write-step";
import { useWorkspace } from "@/features/customers/names";
import { Panel, ScrollArea } from "@/features/shell/page";
import type { Channel } from "@/lib/api/broadcasts";
import { type Series, saveSeries, seriesQuery } from "@/lib/api/series";

import { sendingOf } from "./entry-settings";
import { nodeLook } from "./looks";
import { isMessage, setSettings } from "./model";

/*
 * useWrittenSave saves what is written into the step's settings a moment
 * after the last change, one save at a time, over the lock version the
 * last save returned.
 */
function useWrittenSave(workspace: string, series: Series, step: string) {
  const client = useQueryClient();
  const lock = useRef(series.lock_version);
  const spec = useRef(series.spec);
  const [dirty, setDirty] = useState<Written | null>(null);
  const save = useMutation({
    mutationFn: (written: Written) => {
      const node = spec.current.nodes.find((n) => n.id === step);
      const next = setSettings(spec.current, step, { ...node?.settings, ...written });

      return saveSeries(workspace, series.id, {
        lock_version: lock.current,
        name: series.name,
        spec: next,
        ...sendingOf(series),
      });
    },
    onSuccess: (saved) => {
      lock.current = saved.lock_version;
      spec.current = saved.spec;
      client.setQueryData(seriesQuery(workspace, series.id).queryKey, saved);
    },
    onError: (error) => void toast.error(error instanceof Error ? error.message : "The message was not saved."),
  });
  const { mutate } = save;

  useEffect(() => {
    if (series.lock_version > lock.current) {
      lock.current = series.lock_version;
      spec.current = series.spec;
    }
  }, [series]);

  useEffect(() => {
    if (!dirty) {
      return;
    }
    const timer = setTimeout(() => {
      setDirty(null);
      mutate(dirty);
    }, 700);

    return () => clearTimeout(timer);
  }, [dirty, mutate]);

  return { saving: save.isPending || dirty !== null, change: setDirty };
}

function Composer({ workspace, series, step }: { workspace: string; series: Series; step: string }) {
  const node = series.spec.nodes.find((n) => n.id === step);
  const settings = node?.settings ?? {};
  const [written, setWritten] = useState<Written>({
    subject: settings.subject ?? "",
    preview: settings.preview ?? "",
    body: settings.body ?? "",
  });
  const save = useWrittenSave(workspace, series, step);
  const sender = useActiveMembers(workspace).find((m) => m.id === series.sender_id);
  const name = sender ? sender.user.name || sender.user.email : "your team";

  if (!node || !isMessage(node.kind)) {
    return <Missing series={series.id} />;
  }

  return (
    <Panel label="Compose" ground className="h-full w-full flex-1">
      <header className="flex min-h-12 shrink-0 items-center gap-2 ps-2.5 pe-2.5 pt-[7px]">
        <Link
          to="/d/series/$series"
          params={{ series: series.id }}
          search={{ step }}
          aria-label={`Back to ${series.name}`}
          className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </Link>
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">
          {series.name} · {nodeLook[node.kind].name}
        </h1>
        <span className="truncate text-body-2-regular text-text-tertiary">
          {save.saving ? "Saving…" : "Saved. Publish the series to send it."}
        </span>
      </header>
      <ScrollArea ground>
        <div className="mx-auto flex w-full max-w-[42.5rem] flex-col px-6 pt-10 pb-16 md:pt-14">
          <WriteStep
            workspace={workspace}
            channel={node.kind as Channel}
            draft={written}
            sender={name}
            onChange={(change) => {
              const next = { ...written, ...change };
              setWritten(next);
              save.change(next);
            }}
          />
        </div>
      </ScrollArea>
    </Panel>
  );
}

function Missing({ series }: { series: string }) {
  return (
    <Panel label="Compose" ground className="h-full flex-1">
      <div className="flex h-full flex-col items-center justify-center gap-2 text-body-2-regular text-text-tertiary">
        This step is not a message any more.
        <Link to="/d/series/$series" params={{ series }} className="text-text-primary underline">
          Back to the series
        </Link>
      </div>
    </Panel>
  );
}

export function SeriesComposePage({ id, step }: { id: string; step: string }) {
  const workspace = useWorkspace();
  const series = useQuery(seriesQuery(workspace, id));

  if (!series.data) {
    return (
      <Panel label="Compose" ground className="h-full flex-1">
        <div className="flex h-full items-center justify-center text-body-2-regular text-text-tertiary">
          {series.isError ? "This series does not exist or you cannot see it." : "Loading…"}
        </div>
      </Panel>
    );
  }

  return <Composer workspace={workspace} series={series.data} step={step} />;
}
