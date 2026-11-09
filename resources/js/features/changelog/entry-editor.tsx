/*
 * Writing one changelog update, laid out the way readers will see it,
 * in the same writer help center articles use. Saving keeps a working
 * copy readers never see; Publish puts it live now or at a set time, and
 * can email the subscribers once. Authors are everyone who edits it
 * unless the team picks them. A save made over someone else's newer one
 * is refused and says so, so nobody's words are lost.
 */
import { RiArrowLeftLine, RiExternalLinkLine, RiSettings3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type ChangelogEntry,
  changelogKeys,
  deleteEntry,
  entryQuery,
  labelsQuery,
  type PublishOptions,
  publishEntry,
  saveEntry,
  unpublishEntry,
} from "@/lib/api/changelog";
import { defaultBrand, helpCentersQuery } from "@/lib/api/help-centers";

import { AuthorsDialog, EntrySettings, PublishDialog } from "./entry-dialogs";
import { changeOf, type Draft, draftOf, entryState } from "./entry-format";
import { EntryPage } from "./entry-page";

type Open = "publish" | "unpublish" | "authors" | "settings" | null;

const stateColors = { Draft: "gray", Scheduled: "blue", Published: "lime", "Unpublished changes": "orange" } as const;

function EntryHeader({
  entry,
  live,
  saving,
  onOpen,
  onSave,
}: {
  entry: ChangelogEntry;
  live: string | null;
  saving: boolean;
  onOpen: (open: Open) => void;
  onSave: () => void;
}) {
  const state = entryState(entry);
  const published = entry.status === "published";

  return (
    <PageHeader
      title={
        <span className="flex min-w-0 items-center gap-2">
          <Link
            to="/d/changelog"
            aria-label="Back to changelog"
            className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiArrowLeftLine aria-hidden className="size-4" />
          </Link>
          <span className="truncate">{entry.title || "Untitled update"}</span>
          <Chip variant="caption" color={stateColors[state]}>
            {state}
          </Chip>
        </span>
      }
    >
      {live ? (
        <a
          href={live}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
          View
        </a>
      ) : null}
      {entry.status !== "draft" ? (
        <Button size="small" variant="secondary" onClick={() => onOpen("unpublish")}>
          {published ? "Unpublish" : "Unschedule"}
        </Button>
      ) : null}
      <IconButton size="small" icon={RiSettings3Line} aria-label="Update settings" onClick={() => onOpen("settings")} />
      <Button size="small" variant="secondary" onClick={onSave} disabled={saving}>
        Save draft
      </Button>
      <Button size="small" onClick={() => onOpen("publish")} disabled={saving}>
        {published ? "Publish changes" : "Publish"}
      </Button>
    </PageHeader>
  );
}

/*
 * useEntry loads the entry and keeps every list and cached copy in step
 * with what the server last answered.
 */
function useEntry(workspace: string, id: string) {
  const queryClient = useQueryClient();
  const { data: entry } = useQuery(entryQuery(workspace, id));
  const settle = (next: ChangelogEntry) => {
    queryClient.setQueryData(entryQuery(workspace, id).queryKey, next);
    void queryClient.invalidateQueries({ queryKey: changelogKeys.entries(workspace, next.help_center_id) });
  };
  return { entry, settle };
}

export function EntryEditor({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const { entry, settle } = useEntry(workspace, id);
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.id === entry?.help_center_id);
  const { data: labels = [] } = useQuery(labelsQuery(workspace, entry?.help_center_id ?? ""));
  const [draft, setDraft] = useState<Draft | null>(null);
  const [open, setOpen] = useState<Open>(null);

  useEffect(() => {
    if (entry && draft === null) {
      setDraft(draftOf(entry));
    }
  }, [entry, draft]);

  const save = useMutation({
    mutationFn: (current: { entry: ChangelogEntry; draft: Draft }) =>
      saveEntry(workspace, id, {
        ...changeOf(current.draft, current.entry),
        lock_version: current.entry.lock_version,
      }),
    onSuccess: (saved, current) => {
      settle(saved);
      setDraft((d) => (d && d.slug === current.entry.slug ? { ...d, slug: saved.slug } : d));
    },
  });
  const unpublish = useMutation({ mutationFn: () => unpublishEntry(workspace, id), onSuccess: settle });

  if (!entry || !draft) {
    return (
      <Panel label="Update" className="h-full flex-1">
        <PageHeader title="Update" />
      </Panel>
    );
  }

  const set = (change: Partial<Draft>) => setDraft({ ...draft, ...change });
  const live = entry.status === "published" && center ? `${center.url}/changelog/${entry.slug}` : null;
  const publish = async (options: PublishOptions) => {
    const saved = await save.mutateAsync({ entry, draft });
    if (saved.status === "published" && !saved.has_unpublished_changes && !options.notify) {
      toast.success("Already live: there is nothing new to publish");
      return;
    }
    settle(await publishEntry(workspace, id, options));
    toast.success(options.scheduled_for ? "Update scheduled" : "Update published");
  };
  const saveAuthors = async (authors: string[]) => {
    setDraft({ ...draft, authors });
    await save.mutateAsync({ entry, draft: { ...draft, authors } });
  };

  return (
    <Panel label="Update" className="h-full w-full flex-1">
      <EntryHeader
        entry={entry}
        live={live}
        saving={save.isPending}
        onOpen={setOpen}
        onSave={() => save.mutate({ entry, draft }, { onSuccess: () => toast.success("Draft saved") })}
      />
      <ScrollArea>
        <EntryPage
          workspace={workspace}
          entry={entry}
          draft={draft}
          labels={labels}
          brand={center?.settings.primary_color || defaultBrand}
          revision={0}
          failure={save.error ?? unpublish.error}
          onChange={set}
          onAuthors={() => setOpen("authors")}
        />
      </ScrollArea>
      {open === "publish" ? (
        <PublishDialog workspace={workspace} entry={entry} onPublish={publish} onClose={() => setOpen(null)} />
      ) : null}
      {open === "authors" ? (
        <AuthorsDialog
          workspace={workspace}
          chosen={draft.authors}
          onSave={saveAuthors}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open === "settings" ? (
        <EntrySettings
          entry={entry}
          draft={draft}
          onChange={set}
          onDate={async (day) => settle(await publishEntry(workspace, id, { published_at: day }))}
          onDelete={async () => {
            await deleteEntry(workspace, id);
            settle({ ...entry });
            await navigate({ to: "/d/changelog" });
          }}
          onClose={() => setOpen(null)}
        />
      ) : null}
      <AlertDialog
        isOpen={open === "unpublish"}
        onOpenChange={(next) => (next ? undefined : setOpen(null))}
        title={entry.status === "published" ? "Unpublish this update?" : "Stop the scheduled publish?"}
        description="It goes back to a draft that readers do not see. Nothing is emailed."
        confirmLabel={entry.status === "published" ? "Unpublish" : "Unschedule"}
        onConfirm={() => unpublish.mutateAsync()}
      />
    </Panel>
  );
}
