/*
 * The saved audiences, in the data table block, everything about each in
 * its row: its name and what it is for, its rule in a line, how many
 * people it matches, where it is used and when it changed. The server
 * searches them by name and serves them a numbered page at a time. Each
 * row's menu edits the rule, renames, tests a person, freezes it into a
 * tag or deletes it, each in a modal; clicking a row edits its rule,
 * kept in the address. New audience names it and gives it a rule in one
 * modal.
 */
import {
  RiAddLine,
  RiDeleteBinLine,
  RiEditLine,
  RiFilter3Line,
  RiGroupLine,
  RiPriceTag3Line,
  RiSearchLine,
  RiUserSearchLine,
} from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import type { ColumnDef } from "@tanstack/react-table";
import { useDeferredValue, useEffect, useRef, useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { InputBase } from "@/components/base/input/input";
import { usePermission } from "@/features/settings/inboxes";
import { EmptyState } from "@/features/shell/page";
import { type RowAction, RowMenu } from "@/features/shell/row-menu";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { type Audience, audiencesPageQuery, audiencesQuery, previewSavedAudience } from "@/lib/api/audiences";
import { peopleCap, peoplePerPage } from "@/lib/api/customers";
import { tagsQuery } from "@/lib/api/tags";
import { formatDate, formatNumber } from "@/lib/i18n/format";

import { CustomersActions } from "../customers/frame";
import { useWorkspace } from "../customers/names";
import { AudienceModal, DeleteModal, FreezeModal, TestModal } from "./audience-modals";
import { freshAudienceName } from "./fresh-name";
import { ListUploadButton } from "./list-upload";
import { summary } from "./rules";
import { blankAudience } from "./use-audience-draft";

function size(a: Audience): string {
  const e = a.last_estimate;
  if (!e) {
    return "Counting…";
  }
  const people = e.count === 1 ? "1 person" : `${e.count.toLocaleString()} people`;

  return e.approximate ? `About ${people}` : people;
}

const widths: Record<string, string> = {
  name: "w-[260px]",
  rule: "w-[340px]",
  size: "w-[150px]",
  used: "w-[120px]",
  updated: "w-[140px]",
  actions: "w-[64px]",
};

/*
 * useFreshCounts counts again each audience never counted or changed since
 * it last was, once per visit, so the Matches column says how many match
 * now. The server keeps each count with the audience. It waits while an
 * audience is open, so the list never refreshes under the editor.
 */
function useFreshCounts(workspace: string, audiences: Audience[] | undefined, editing: boolean) {
  const queryClient = useQueryClient();
  const asked = useRef(new Set<string>());

  useEffect(() => {
    if (editing) {
      return;
    }
    const stale = (audiences ?? []).filter(
      (a) =>
        !asked.current.has(`${a.id}@${a.updated_at}`) &&
        (!a.last_estimate || new Date(a.last_estimate.at) < new Date(a.updated_at)),
    );
    if (stale.length === 0) {
      return;
    }
    for (const a of stale) {
      asked.current.add(`${a.id}@${a.updated_at}`);
    }
    void Promise.allSettled(stale.map((a) => previewSavedAudience(workspace, a.id))).then(() =>
      queryClient.invalidateQueries({ queryKey: audiencesQuery(workspace).queryKey }),
    );
  }, [workspace, audiences, queryClient, editing]);
}

function totalText(total: number | undefined): string {
  if (total === undefined) {
    return "—";
  }

  return `${formatNumber(total)}${total >= peopleCap ? "+" : ""} ${total === 1 ? "audience" : "audiences"}`;
}

/*
 * Doing is the modal open on an audience, or a new one.
 */
type Doing = { kind: "new" } | { kind: "rules" | "details" | "test" | "freeze" | "delete"; audience: Audience };

const menu: { kind: Exclude<Doing["kind"], "new">; label: string; icon: typeof RiEditLine; manage: boolean }[] = [
  { kind: "rules", label: "Edit rule", icon: RiFilter3Line, manage: false },
  { kind: "details", label: "Rename", icon: RiEditLine, manage: true },
  { kind: "test", label: "Test a person", icon: RiUserSearchLine, manage: false },
  { kind: "freeze", label: "Freeze into a tag", icon: RiPriceTag3Line, manage: true },
];

/*
 * AudienceMenu is a row's actions: those the reader may take, with
 * delete apart for those who manage audiences.
 */
function AudienceMenu({
  audience,
  canManage,
  onPick,
}: {
  audience: Audience;
  canManage: boolean;
  onPick: (doing: Doing) => void;
}) {
  const actions: RowAction<Exclude<Doing["kind"], "new">>[] = [
    ...menu.filter((m) => canManage || !m.manage),
    ...(canManage ? [{ kind: "delete" as const, label: "Delete", icon: RiDeleteBinLine, danger: true }] : []),
  ];

  return <RowMenu label={audience.name} actions={actions} onPick={(kind) => onPick({ kind, audience })} />;
}

/*
 * Doings shows the modal open, if any.
 */
function Doings({ workspace, doing, onClose }: { workspace: string; doing: Doing | null; onClose: () => void }) {
  const saved = useQuery(audiencesQuery(workspace)).data ?? [];
  if (!doing) {
    return null;
  }
  if (doing.kind === "new") {
    return (
      <AudienceModal
        workspace={workspace}
        audience={blankAudience(freshAudienceName(saved.map((a) => a.name)))}
        mode="new"
        onClose={onClose}
      />
    );
  }
  const props = { workspace, audience: doing.audience, onClose };
  const modals = {
    rules: <AudienceModal key={doing.audience.id} {...props} mode="rules" />,
    details: <AudienceModal key={doing.audience.id} {...props} mode="details" />,
    test: <TestModal {...props} />,
    freeze: <FreezeModal {...props} />,
    delete: <DeleteModal {...props} />,
  };

  return modals[doing.kind];
}

export function AudienceList() {
  const workspace = useWorkspace();
  const canManage = usePermission(workspace, "audiences:manage");
  const navigate = useNavigate();
  const { data } = useQuery({ ...audiencesQuery(workspace), enabled: workspace !== "" });
  const tags = useQuery({ ...tagsQuery(workspace), enabled: workspace !== "" }).data ?? [];
  const names = Object.fromEntries([...tags.map((t) => [t.id, t.name]), ...(data ?? []).map((a) => [a.id, a.name])]);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const q = useDeferredValue(search.trim());
  const list = useQuery({ ...audiencesPageQuery(workspace, q, page), enabled: workspace !== "" });
  const shown = list.data?.items ?? [];
  const total = list.data?.total ?? 0;
  const [doing, setDoing] = useState<Doing | null>(null);
  const { audience: linked } = useSearch({ strict: false }) as { audience?: string };
  const linkedAudience = data?.find((a) => a.id === linked);
  const current: Doing | null = doing ?? (linkedAudience ? { kind: "rules", audience: linkedAudience } : null);
  useFreshCounts(workspace, list.data?.items, current !== null);
  const close = () => {
    setDoing(null);
    if (linked) {
      void navigate({ to: ".", search: {}, replace: true });
    }
  };
  const columns: ColumnDef<Audience>[] = [
    {
      id: "name",
      header: "Audience",
      cell: ({ row }) => (
        <span className="flex min-w-0 items-center gap-2.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background-tertiary-default">
            <RiGroupLine aria-hidden className="size-4 text-foreground-icon-secondary" />
          </span>
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body-medium text-text-primary">{row.original.name}</span>
            {row.original.description ? (
              <span className="truncate text-body-2-regular text-text-tertiary">{row.original.description}</span>
            ) : null}
          </span>
        </span>
      ),
    },
    {
      id: "rule",
      header: "Rule",
      cell: ({ row }) => (
        <span className="block truncate text-body-medium text-text-secondary">
          {summary(row.original.rules, names)}
        </span>
      ),
    },
    {
      id: "size",
      header: "Matches",
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-body-medium text-text-primary">{size(row.original)}</span>
      ),
    },
    {
      id: "used",
      header: "Used in",
      cell: ({ row }) => {
        const used = row.original.used_by?.length ?? 0;

        return (
          <Chip variant="caption" color={used > 0 ? "blue" : "soft"}>
            {used === 0 ? "Not used" : `${used} ${used === 1 ? "place" : "places"}`}
          </Chip>
        );
      },
    },
    {
      id: "updated",
      header: "Updated",
      cell: ({ row }) => (
        <span className="whitespace-nowrap text-body-medium text-text-secondary">
          {formatDate(row.original.updated_at)}
        </span>
      ),
    },
    {
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      cell: ({ row }) => (
        <span className="flex justify-end">
          <AudienceMenu audience={row.original} canManage={canManage} onPick={setDoing} />
        </span>
      ),
    },
  ];

  return (
    <>
      <CustomersActions>
        <ListUploadButton workspace={workspace} />
        {canManage ? (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setDoing({ kind: "new" })}>
            New audience
          </Button>
        ) : null}
      </CustomersActions>
      {list.data && total === 0 && q === "" ? (
        <EmptyState
          icon={RiGroupLine}
          title="No audiences yet"
          description="Name a group of customers once, then use it in workflows and everywhere else that asks who."
        />
      ) : (
        <div className="h-full px-4 pb-4">
          <DataTable
            aria-label="Audiences"
            data={shown}
            columns={columns}
            getRowId={(a) => a.id}
            total={list.isError ? list.error.message : totalText(list.data?.total)}
            toolbar={
              <InputBase
                aria-label="Search audiences"
                placeholder="Search"
                leadingIcon={RiSearchLine}
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                }}
                fieldClassName="min-w-[153px] flex-1 rounded-full bg-background-secondary-default sm:w-[200px] sm:min-w-0 sm:flex-none"
                className="text-body-medium"
              />
            }
            page={page}
            pageCount={Math.ceil(Math.min(total, peopleCap) / peoplePerPage)}
            onPageChange={setPage}
            onRowAction={(id) => {
              const audience = shown.find((a) => a.id === id);
              if (audience) {
                setDoing({ kind: "rules", audience });
              }
            }}
            widths={widths}
            loading={list.isPending}
            skeleton={<TableSkeleton rows={4} />}
            empty="No audience matches."
          />
        </div>
      )}
      <Doings workspace={workspace} doing={current} onClose={close} />
    </>
  );
}
