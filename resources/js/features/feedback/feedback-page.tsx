/*
 * The feature requests customers send, the way the team triages them: a
 * pane of views by the workspace's statuses and approval, with the switch
 * that opens the portal to customers, beside the requests a numbered page
 * at a time, narrowed to a board, searched and sorted by trending, top or
 * new. Opening a request shows its votes and conversation.
 */
import {
  RiAddLine,
  RiExternalLinkLine,
  RiInboxLine,
  RiLightbulbLine,
  RiSettings4Line,
  RiShieldCheckLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { type ComponentType, useRef, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { DataTable } from "@/components/application/data-table/data-table";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { useWide } from "@/features/shell/use-wide";
import {
  boardsQuery,
  type FeedbackPost,
  type FeedbackStatus,
  type PostFilter,
  type PostList,
  type PostSort,
  type PostStatus,
  postsPerPage,
  postsQuery,
} from "@/lib/api/feedback";
import { type HelpCenter, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";
import { formatDate, formatNumber } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { NewRequestDialog } from "./new-request";
import { StatusChip, StatusDot } from "./status-chip";
import { useStatuses } from "./use-statuses";

const pane = { group: "dimmi:feedback", side: "views", list: "requests" } as const;
const visible = { overflow: "visible" } as const;

const sorts: [PostSort, string][] = [
  ["trending", "Trending"],
  ["top", "Most votes"],
  ["new", "Newest"],
];

function rowClass(current: boolean): string {
  return cx(
    "flex min-h-8 w-full items-center gap-2 rounded-xl ps-3 pe-2 text-text-primary outline-none",
    "transition-[background-color,box-shadow] duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
    current
      ? "bg-background-primary-default text-body-2-medium shadow-xs ring-1 ring-inset ring-border-button-default"
      : "text-body-2-regular hover:bg-background-secondary-hover",
  );
}

function Pick({
  label,
  icon,
  count,
  current,
  onPress,
}: {
  label: string;
  icon: ComponentType<{ className?: string }> | string;
  count?: number;
  current: boolean;
  onPress: () => void;
}) {
  const Icon = typeof icon === "string" ? null : icon;
  return (
    <button type="button" aria-current={current ? "page" : undefined} onClick={onPress} className={rowClass(current)}>
      {Icon ? (
        <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary opacity-50" />
      ) : (
        <StatusDot color={icon as string} />
      )}
      <span className="min-w-0 flex-1 truncate text-start">{label}</span>
      {count ? <span className="text-caption-1-regular text-text-tertiary tabular-nums">{count}</span> : null}
    </button>
  );
}

/*
 * BoardSwitches opens the portal to customers, saved at once on the help
 * center, and leads to the settings of its boards, which say who sees
 * each and whether it holds posts for approval.
 */
function BoardSwitches({ workspace, center }: { workspace: string; center: HelpCenter }) {
  const client = useQueryClient();
  const save = useMutation({
    mutationFn: (change: { feedback_board?: boolean }) =>
      updateHelpCenter(workspace, center.id, { settings: { ...center.settings, ...change } }),
    onSuccess: () => client.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <div className="mx-1 mt-4 flex flex-col gap-3 rounded-2xl bg-background-primary-default p-3 shadow-xs ring-1 ring-inset ring-border-button-default">
      <Switch
        size="sm"
        isSelected={center.settings.feedback_board ?? false}
        onChange={(on) => save.mutate({ feedback_board: on })}
      >
        Open to customers
      </Switch>
      <Link
        to="/d/settings/$section"
        params={{ section: "feedback" }}
        className="flex items-center gap-1 text-body-2-medium text-text-secondary hover:text-text-primary"
      >
        <RiSettings4Line aria-hidden className="size-4" />
        Boards and statuses
      </Link>
      {center.settings.feedback_board ? (
        <a
          href={`${center.url}/feedback`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1 text-body-2-medium text-text-secondary hover:text-text-primary"
        >
          <RiExternalLinkLine aria-hidden className="size-4" />
          View the board
        </a>
      ) : null}
    </div>
  );
}

/*
 * View is what the list shows: every request, one status, or those
 * waiting for approval.
 */
type View = { kind: "all" } | { kind: "status"; status: PostStatus } | { kind: "pending" };

function viewOf(raw: string | undefined): View {
  if (raw === "pending") {
    return { kind: "pending" };
  }
  return raw ? { kind: "status", status: raw } : { kind: "all" };
}

function titleOf(view: View, list: FeedbackStatus[]): string {
  if (view.kind === "pending") {
    return "Waiting for approval";
  }
  if (view.kind === "status") {
    return list.find((s) => s.id === view.status)?.name ?? "Requests";
  }
  return "All requests";
}

const columns: ColumnDef<FeedbackPost>[] = [
  {
    id: "request",
    header: "Request",
    cell: ({ row }) => (
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">
          <span className="text-text-tertiary">#{row.original.number} </span>
          {row.original.title}
        </span>
        <span className="truncate text-body-2-regular text-text-tertiary">
          {row.original.body_markdown.split("\n")[0] || `${row.original.comment_count} comments`}
        </span>
      </span>
    ),
  },
  {
    id: "votes",
    header: "Votes",
    cell: ({ row }) => (
      <span className="flex h-11 w-11 flex-col items-center justify-center rounded-xl border border-border-button-default bg-background-primary-default text-body-medium tabular-nums text-text-primary">
        {row.original.vote_count}
      </span>
    ),
  },
  {
    id: "status",
    header: "Status",
    cell: ({ row }) => <StatusChip post={row.original} />,
  },
  {
    id: "author",
    header: "Posted by",
    cell: ({ row }) => (
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-2-regular text-text-secondary">{row.original.author}</span>
        <span className="text-caption-1-regular text-text-tertiary">{formatDate(row.original.created_at)}</span>
      </span>
    ),
  },
];

const widths = { request: "w-[48%]", votes: "w-[10%]", status: "w-[20%]", author: "w-[22%]" };

/*
 * FeedbackViews is the pane of views by status and approval, with the
 * board's switches under them.
 */
function FeedbackViews({
  workspace,
  center,
  view,
  counts,
  onView,
}: {
  workspace: string;
  center: HelpCenter | undefined;
  view: View;
  counts: PostList["counts"] | undefined;
  onView: (view: string | undefined) => void;
}) {
  const ordered = useStatuses(workspace);
  const all = ordered.reduce((sum, s) => sum + (counts?.[s.id] ?? 0), 0);

  return (
    <Panel label="Feedback views" className="h-full w-full">
      <header className="flex h-14 shrink-0 items-center ps-4 pe-2.5">
        <h2 className="truncate text-body-medium text-text-primary">Feature requests</h2>
      </header>
      <ScrollArea>
        <nav aria-label="Feedback views" className="flex flex-col gap-0.5 px-2 pb-4">
          <Pick
            label="All requests"
            icon={RiLightbulbLine}
            count={all}
            current={view.kind === "all"}
            onPress={() => onView(undefined)}
          />
          {ordered.map((s) => (
            <Pick
              key={s.id}
              label={s.name}
              icon={s.color}
              count={counts?.[s.id]}
              current={view.kind === "status" && view.status === s.id}
              onPress={() => onView(s.id)}
            />
          ))}
          <div aria-hidden className="mx-3 my-3 h-px bg-separator-border" />
          <Pick
            label="Waiting for approval"
            icon={RiShieldCheckLine}
            count={counts?.pending}
            current={view.kind === "pending"}
            onPress={() => onView("pending")}
          />
          {center ? <BoardSwitches workspace={workspace} center={center} /> : null}
        </nav>
      </ScrollArea>
    </Panel>
  );
}

/*
 * filterOf is the page of requests the address and search ask for.
 */
function filterOf(view: View, search: { board?: string; sort?: string; page?: number }, query: string): PostFilter {
  return {
    board: search.board,
    sort: search.sort === "top" || search.sort === "new" ? search.sort : "trending",
    page: search.page ?? 1,
    query: query || undefined,
    status: view.kind === "status" ? view.status : undefined,
    moderation: view.kind === "pending" ? "pending" : undefined,
  };
}

export function FeedbackPage() {
  const workspace = useAccount().workspace.id;
  const search = useSearch({ from: "/d/feedback/" });
  const navigate = useNavigate({ from: "/d/feedback/" });
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers[0];
  const hc = center?.id ?? "";
  const view = viewOf(search.view);
  const ordered = useStatuses(workspace);
  const { data: boards = [] } = useQuery(boardsQuery(workspace, hc));
  const [query, setQuery] = useState("");
  const [creating, setCreating] = useState(false);
  const typing = useRef<number | undefined>(undefined);
  const filter = filterOf(view, search, query);
  const sort = filter.sort;
  const list = useQuery(postsQuery(workspace, hc, filter));
  const counts = useQuery(postsQuery(workspace, hc, { board: search.board, sort: "new", page: 1 })).data?.counts;
  const total = list.data?.meta.total ?? 0;
  const wide = useWide(1024);
  const go = (next: { board?: string; view?: string; sort?: string; page?: number }) =>
    void navigate({ search: { board: search.board, view: search.view, sort: search.sort, ...next }, replace: true });

  const side = (
    <FeedbackViews
      workspace={workspace}
      center={center}
      view={view}
      counts={counts}
      onView={(next) => go({ view: next, page: undefined })}
    />
  );
  const main = (
    <Panel label="Feature requests" ground className="h-full w-full flex-1">
      <PageHeader title={wide ? titleOf(view, ordered) : "Feature requests"}>
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setCreating(true)} disabled={!center}>
          New request
        </Button>
      </PageHeader>
      <ScrollArea ground>
        <div key={search.view ?? "all"} className="flex h-full flex-col px-4 pb-4 animate-page-in">
          <DataTable
            aria-label="Requests"
            data={list.data?.data ?? []}
            columns={columns}
            getRowId={(p) => p.id}
            total={`${formatNumber(total)} ${total === 1 ? "request" : "requests"}`}
            toolbar={
              <div className="flex items-center gap-2">
                {boards.length > 1 ? (
                  <Select
                    aria-label="Board"
                    className="w-44"
                    selectedKey={search.board ?? "all"}
                    onSelectionChange={(key) => go({ board: key === "all" ? undefined : String(key), page: undefined })}
                  >
                    {[{ id: "all", name: "All boards" }, ...boards].map((b) => (
                      <SelectItem key={b.id} id={b.id} textValue={b.name}>
                        {b.name}
                      </SelectItem>
                    ))}
                  </Select>
                ) : null}
                <Input
                  aria-label="Search requests"
                  placeholder="Search requests"
                  value={query}
                  onChange={(next) => {
                    setQuery(next);
                    window.clearTimeout(typing.current);
                    typing.current = window.setTimeout(() => go({ page: undefined }), 250);
                  }}
                  className="w-56"
                />
                <Select
                  aria-label="Order"
                  className="w-40"
                  selectedKey={sort}
                  onSelectionChange={(key) =>
                    go({ sort: key === "trending" ? undefined : String(key), page: undefined })
                  }
                >
                  {sorts.map(([id, name]) => (
                    <SelectItem key={id} id={id} textValue={name}>
                      {name}
                    </SelectItem>
                  ))}
                </Select>
              </div>
            }
            page={filter.page}
            pageCount={Math.max(1, Math.ceil(total / postsPerPage))}
            onPageChange={(page) => go({ page: page > 1 ? page : undefined })}
            onRowAction={(id) => void navigate({ to: "/d/feedback/$post", params: { post: id } })}
            widths={widths}
            loading={list.isPending}
            skeleton={<TableSkeleton rows={4} />}
            empty={
              <span className="flex flex-col items-center gap-2">
                <RiInboxLine aria-hidden className="size-5 text-foreground-icon-quaternary" />
                {query ? "No request matches." : "No requests here yet."}
              </span>
            }
          />
        </div>
      </ScrollArea>
      {creating && center ? (
        <NewRequestDialog
          workspace={workspace}
          helpCenter={center.id}
          board={search.board}
          onClose={() => setCreating(false)}
        />
      ) : null}
    </Panel>
  );

  if (!wide) {
    return main;
  }
  return (
    <PaneGroup id={pane.group} panelIds={[pane.side, pane.list]}>
      <Pane
        id={pane.side}
        defaultSize={250}
        minSize={210}
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
