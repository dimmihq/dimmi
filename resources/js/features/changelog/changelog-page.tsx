/*
 * The changelog the team writes: its updates a numbered page at a time,
 * narrowed by state, label or title, beside the pane that picks the view
 * and arranges labels, and the people subscribed. Each update shows its
 * cover, labels, authors and where it stands; opening one edits it. The
 * address keeps the view, so coming back from an update returns to it.
 */
import { RiAddLine, RiArticleLine, RiExternalLinkLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useRef, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { DataTable } from "@/components/application/data-table/data-table";
import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { useWide } from "@/features/shell/use-wide";
import {
  type ChangelogEntry,
  createEntry,
  type EntryFilter,
  entriesPerPage,
  entriesQuery,
  labelsQuery,
} from "@/lib/api/changelog";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { formatNumber } from "@/lib/i18n/format";

import { ChangelogPanel, type View } from "./changelog-panel";
import { authorNames, entryState, entryWhen, initialsOf } from "./entry-format";
import { SubscribersList } from "./subscribers-list";

const stateColors = { Draft: "gray", Scheduled: "blue", Published: "lime", "Unpublished changes": "orange" } as const;

const pane = { group: "dimmi:changelog", side: "views", list: "updates" } as const;

const visible = { overflow: "visible" } as const;

/*
 * viewOf reads the view the address keeps; addressOf writes one back.
 */
function viewOf(raw: string | undefined): View {
  if (raw === "draft" || raw === "scheduled" || raw === "published") {
    return { kind: "status", status: raw };
  }
  if (raw === "subscribers") {
    return { kind: "subscribers" };
  }
  if (raw?.startsWith("label:")) {
    return { kind: "label", id: raw.slice(6) };
  }
  return { kind: "all" };
}

function addressOf(view: View): string | undefined {
  switch (view.kind) {
    case "status":
      return view.status;
    case "label":
      return `label:${view.id}`;
    case "subscribers":
      return "subscribers";
    default:
      return undefined;
  }
}

function titleOf(view: View, labels: { id: string; name: string }[]): string {
  switch (view.kind) {
    case "status":
      return { draft: "Drafts", scheduled: "Scheduled", published: "Published" }[view.status];
    case "label":
      return labels.find((l) => l.id === view.id)?.name ?? "Label";
    case "subscribers":
      return "Subscribers";
    default:
      return "All updates";
  }
}

function UpdateCell({ entry }: { entry: ChangelogEntry }) {
  return (
    <span className="flex min-w-0 items-center gap-3">
      {entry.cover_url ? (
        <img
          src={entry.cover_url}
          alt=""
          className="h-10 w-16 shrink-0 rounded-lg border border-separator-border object-cover"
        />
      ) : (
        <span className="flex h-10 w-16 shrink-0 items-center justify-center rounded-lg border border-separator-border bg-background-secondary-default">
          <RiArticleLine aria-hidden className="size-4 text-foreground-icon-quaternary" />
        </span>
      )}
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{entry.title}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{entry.summary || entryWhen(entry)}</span>
      </span>
    </span>
  );
}

const columns: ColumnDef<ChangelogEntry>[] = [
  { id: "update", header: "Update", cell: ({ row }) => <UpdateCell entry={row.original} /> },
  {
    id: "labels",
    header: "Labels",
    cell: ({ row }) => (
      <span className="flex flex-wrap gap-1">
        {row.original.labels.map((l) => (
          <span
            key={l.id}
            className="inline-flex h-6 items-center gap-1.5 rounded-full bg-background-secondary-default px-2 text-caption-1-medium text-text-secondary"
          >
            <span aria-hidden className="size-1.5 rounded-full" style={{ background: l.color }} />
            {l.name}
          </span>
        ))}
      </span>
    ),
  },
  {
    id: "authors",
    header: "Authors",
    cell: ({ row }) => (
      <span className="flex min-w-0 items-center gap-2" title={authorNames(row.original)}>
        <span className="flex -space-x-1.5">
          {row.original.authors.slice(0, 3).map((a) => (
            <Avatar
              key={a.member_id}
              size="xs"
              src={a.avatar_url ?? undefined}
              initials={initialsOf(a.name)}
              className="ring-2 ring-background-primary-default"
            />
          ))}
        </span>
        <span className="truncate text-body-2-regular text-text-secondary">{authorNames(row.original)}</span>
      </span>
    ),
  },
  {
    id: "state",
    header: "Status",
    cell: ({ row }) => {
      const state = entryState(row.original);
      return (
        <span className="flex flex-col items-start gap-0.5">
          <Chip variant="caption" color={stateColors[state]}>
            {state}
          </Chip>
          <span className="text-caption-1-regular text-text-tertiary">{entryWhen(row.original)}</span>
        </span>
      );
    },
  },
];

const widths = { update: "w-[44%]", labels: "w-[18%]", authors: "w-[20%]", state: "w-[18%]" };

/*
 * useNewEntry writes an empty draft, filed under the label being shown,
 * and opens it.
 */
function useNewEntry(workspace: string, helpCenter: string, view: View) {
  const navigate = useNavigate();
  const client = useQueryClient();
  return useMutation({
    mutationFn: () =>
      createEntry(workspace, helpCenter, {
        title: "Untitled update",
        ...(view.kind === "label" ? { label_ids: [view.id] } : {}),
      }),
    onSuccess: (entry) => {
      void client.invalidateQueries({ queryKey: ["changelog", workspace] });
      void navigate({ to: "/d/changelog/$entry", params: { entry: entry.id } });
    },
    onError: (error) => toast.error(error.message),
  });
}

function UpdatesList({
  workspace,
  helpCenter,
  view,
  page,
  query,
  onPage,
  onQuery,
}: {
  workspace: string;
  helpCenter: string;
  view: View;
  page: number;
  query: string;
  onPage: (page: number) => void;
  onQuery: (query: string) => void;
}) {
  const navigate = useNavigate();
  const filter: EntryFilter = {
    page,
    query: query || undefined,
    status: view.kind === "status" ? view.status : undefined,
    label: view.kind === "label" ? view.id : undefined,
  };
  const list = useQuery(entriesQuery(workspace, helpCenter, filter));
  const total = list.data?.meta.total ?? 0;

  return (
    <DataTable
      aria-label="Updates"
      data={list.data?.data ?? []}
      columns={columns}
      getRowId={(e) => e.id}
      total={`${formatNumber(total)} ${total === 1 ? "update" : "updates"}`}
      toolbar={
        <Input
          aria-label="Search updates"
          placeholder="Search updates"
          value={query}
          onChange={onQuery}
          className="w-64"
        />
      }
      page={page}
      pageCount={Math.max(1, Math.ceil(total / entriesPerPage))}
      onPageChange={onPage}
      onRowAction={(id) => void navigate({ to: "/d/changelog/$entry", params: { entry: id } })}
      widths={widths}
      loading={list.isPending}
      skeleton={<TableSkeleton rows={4} />}
      empty={query ? "No update matches." : "No updates here yet. Write the first one."}
    />
  );
}

/*
 * NarrowViews picks the view on a screen without room for the side
 * pane.
 */
function NarrowViews({
  view,
  labels,
  onView,
}: {
  view: string | undefined;
  labels: { id: string; name: string }[];
  onView: (view: string | undefined) => void;
}) {
  const choices: [string, string][] = [
    ["all", "All updates"],
    ["draft", "Drafts"],
    ["scheduled", "Scheduled"],
    ["published", "Published"],
    ...labels.map((l): [string, string] => [`label:${l.id}`, l.name]),
    ["subscribers", "Subscribers"],
  ];

  return (
    <div className="px-4 pb-3">
      <Select
        aria-label="View"
        className="w-full"
        selectedKey={view ?? "all"}
        onSelectionChange={(key) => onView(key === "all" || key === null ? undefined : String(key))}
      >
        {choices.map(([id, name]) => (
          <SelectItem key={id} id={id} textValue={name}>
            {name}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}

export function ChangelogPage() {
  const workspace = useAccount().workspace.id;
  const search = useSearch({ from: "/d/changelog/" });
  const navigate = useNavigate({ from: "/d/changelog/" });
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers[0];
  const hc = center?.id ?? "";
  const { data: labels = [] } = useQuery(labelsQuery(workspace, hc));
  const counts = useQuery(entriesQuery(workspace, hc, { page: 1 })).data?.counts ?? {};
  const view = viewOf(search.view);
  const [query, setQuery] = useState("");
  const page = search.page ?? 1;
  const wide = useWide(1024);
  const make = useNewEntry(workspace, hc, view);
  const typing = useRef<number | undefined>(undefined);
  const go = (next: { view?: string; page?: number }) => void navigate({ search: next, replace: true });

  const side = (
    <ChangelogPanel
      workspace={workspace}
      helpCenter={hc}
      labels={labels}
      counts={counts}
      view={view}
      onView={(next) => go({ view: addressOf(next) })}
    />
  );
  const main = (
    <Panel label="Changelog" ground className="h-full w-full flex-1">
      <PageHeader title={wide ? titleOf(view, labels) : "Changelog"}>
        {center ? (
          <a
            href={`${center.url}/changelog`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
            View changelog
          </a>
        ) : null}
        <Button
          size="small"
          leadingIcon={RiAddLine}
          onClick={() => make.mutate()}
          loading={make.isPending}
          disabled={!center}
        >
          New update
        </Button>
      </PageHeader>
      <ScrollArea ground>
        {wide ? null : <NarrowViews view={search.view} labels={labels} onView={(next) => go({ view: next })} />}
        <div key={search.view ?? "all"} className="flex h-full flex-col px-4 pb-4 animate-page-in">
          {view.kind === "subscribers" ? (
            <SubscribersList workspace={workspace} helpCenter={hc} />
          ) : (
            <UpdatesList
              workspace={workspace}
              helpCenter={hc}
              view={view}
              page={page}
              query={query}
              onPage={(next) => go({ view: search.view, page: next })}
              onQuery={(next) => {
                setQuery(next);
                window.clearTimeout(typing.current);
                typing.current = window.setTimeout(() => go({ view: search.view }), 250);
              }}
            />
          )}
        </div>
      </ScrollArea>
    </Panel>
  );

  if (!wide) {
    return main;
  }
  return (
    <PaneGroup id={pane.group} panelIds={[pane.side, pane.list]}>
      <Pane
        id={pane.side}
        defaultSize={240}
        minSize={200}
        maxSize={360}
        groupResizeBehavior="preserve-pixel-size"
        style={visible}
        className="flex h-full"
      >
        {side}
      </Pane>
      <PaneHandle />
      <Pane id={pane.list} minSize={560} style={visible} className="flex h-full">
        {main}
      </Pane>
    </PaneGroup>
  );
}
