/*
 * The inbox: the list of conversations for the chosen view beside the
 * open conversation, each in its own floating panel, resizable on the gap
 * between them. While a conversation is open the list steps aside unless
 * the screen is very wide, so the thread, its details and Operator have
 * room. On phones one of the two shows at a time. Above the list the
 * person picks the status shown and the order, remembered per view; a
 * search keeps that status.
 */
import { RiChatSmile3Line, RiInbox2Line, RiSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useDeferredValue, useEffect, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Input } from "@/components/base/input/input";
import { Spinner } from "@/components/base/spinner/spinner";
import { PageOutlet } from "@/features/layout/page-outlet";
import { inboxViews, type Page } from "@/features/shell/navigation";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
import { type Draft, draftsQuery } from "@/lib/api/drafts";
import { type ConversationSort, conversationsQuery, countsQuery } from "@/lib/api/inbox";
import { viewsQuery } from "@/lib/api/views";
import { callerQuery } from "@/lib/api/workspace";
import { ConversationList } from "./conversation-list";
import { useInbox } from "./conversations";
import { rememberView } from "./views/last-view";
import { ListControls } from "./views/list-controls";
import { useViewPreferences } from "./views/use-view-preferences";
import { ViewActions } from "./views/view-actions";
import { openCount, useViewName } from "./views/view-groups";
import { ViewPicker } from "./views/view-picker";
import { listStatus, type StatusChoice, sortsFor, viewParams } from "./views/view-route";

const visible = { overflow: "visible" } as const;

/*
 * The panes' names, under which each browser remembers their sizes.
 */
const pane = { group: "dimmi:inbox", list: "list", open: "open" } as const;

/*
 * useArrangement decides which panes show: the list unless a conversation
 * is open on a screen without room for both, and the open side unless a
 * phone is showing the list.
 */
function useArrangement(openId: string | undefined) {
  const roomy = useWide(1536);
  const tablet = useWide(768);
  const list = openId ? roomy : true;
  const open = openId ? true : tablet;
  const ids: string[] = [];
  if (list) {
    ids.push(pane.list);
  }
  if (open) {
    ids.push(pane.open);
  }

  return { list, open, both: list && open, ids };
}

export function InboxPage({ view }: { view: string }) {
  const { conversation: openId } = useParams({ strict: false });
  const shown = useArrangement(openId);
  const fixed = shown.both
    ? { defaultSize: 300, minSize: 240, maxSize: 480, groupResizeBehavior: "preserve-pixel-size" as const }
    : {};

  return (
    <PaneGroup id={pane.group} panelIds={shown.ids}>
      {shown.list ? (
        <Pane id={pane.list} {...fixed} style={visible} className="flex h-full">
          <ListPanel view={view} openId={openId} />
        </Pane>
      ) : null}
      {shown.both ? <PaneHandle /> : null}
      {shown.open ? (
        <Pane id={pane.open} minSize={shown.both ? 480 : undefined} style={visible} className="flex h-full">
          <PageOutlet className="flex h-full min-w-0 flex-1" />
        </Pane>
      ) : null}
    </PaneGroup>
  );
}

/*
 * useListSettings returns the status and order the list shows: the
 * address's status first, then the person's saved choice for the view,
 * and the setters that keep both.
 */
function useListSettings(workspace: string, view: string) {
  const params = viewParams(view) ?? { kind: "fixed" as const, filter: view };
  const { status: fromUrl } = useSearch({ strict: false }) as { status?: StatusChoice };
  const navigate = useNavigate();
  const { preference, save } = useViewPreferences(workspace, view);
  const { data: views } = useQuery({ ...viewsQuery(workspace), enabled: params.kind === "custom" });
  const saved = views?.find((v) => v.id === view);
  const status = listStatus(params, fromUrl ?? preference.status ?? saved?.status_filter ?? undefined);
  const allowed = sortsFor(status).map((s) => s.value as string);
  const chosen = preference.sort ?? saved?.sort ?? undefined;
  const sort = chosen && allowed.includes(chosen) ? chosen : undefined;

  return {
    params,
    status,
    sort,
    setStatus: (next: StatusChoice) => {
      save({ status: next });
      void navigate({ to: ".", search: (prev: Record<string, unknown>) => ({ ...prev, status: next }) });
    },
    setSort: (next: ConversationSort) => save({ sort: next }),
  };
}

function ListPanel({ view, openId }: { view: string; openId: string | undefined }) {
  const fixed = inboxViews[view as keyof typeof inboxViews] as Page | undefined;
  const { workspace } = useInbox();
  const name = useViewName(workspace, view);
  const title = fixed?.label ?? name ?? "Conversations";
  const [search, setSearch] = useState("");
  const query = useDeferredValue(search);
  const settings = useListSettings(workspace, view);
  const {
    data: conversations,
    isPending,
    isError,
  } = useQuery({
    ...conversationsQuery(workspace, {
      view: settings.params.filter,
      status: settings.status,
      sort: settings.sort,
      search: query,
    }),
    enabled: workspace !== "",
  });
  const { data: counts } = useQuery({ ...countsQuery(workspace), enabled: workspace !== "" });
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const { data: drafts } = useQuery({ ...draftsQuery(workspace), enabled: workspace !== "" });
  const count = openCount(counts?.views, view);
  useEffect(() => rememberView(view), [view]);

  return (
    <Panel label={`${title} conversations`} className="h-full w-full">
      <PageHeader title={<ViewPicker workspace={workspace} view={view} title={title} count={count} />}>
        <ViewActions
          workspace={workspace}
          view={view}
          start={{ sort: settings.sort, status: settings.params.status ? undefined : settings.status }}
        />
      </PageHeader>
      <div className="shrink-0 px-[7px] pb-2">
        <Input
          aria-label="Search conversations"
          placeholder="Search conversations"
          leadingIcon={RiSearchLine}
          value={search}
          onChange={setSearch}
        />
      </div>
      <ListControls
        status={settings.status}
        fixedStatus={settings.params.status !== undefined}
        sort={settings.sort ?? "-last_message_at"}
        onStatusChange={settings.setStatus}
        onSortChange={settings.setSort}
      />
      <ListBody
        view={view}
        me={caller?.member?.id}
        description={fixed?.description ?? "Conversations of this view."}
        openId={openId}
        search={query}
        state={isPending ? "loading" : isError ? "error" : "ready"}
        conversations={conversations ?? []}
        drafted={draftedIn(drafts)}
      />
    </Panel>
  );
}

/*
 * draftedIn is the conversations the reader left a reply or note
 * draft in.
 */
function draftedIn(drafts: Draft[] | undefined): ReadonlySet<string> {
  return new Set((drafts ?? []).flatMap((d) => (d.conversation_id && d.body.trim() ? [d.conversation_id] : [])));
}

function ListBody({
  view,
  me,
  description,
  openId,
  search,
  state,
  conversations,
  drafted,
}: {
  view: string;
  me: string | undefined;
  description: string;
  openId: string | undefined;
  search: string;
  state: "loading" | "error" | "ready";
  conversations: NonNullable<Parameters<typeof ConversationList>[0]["conversations"]>;
  drafted: ReadonlySet<string>;
}) {
  if (state === "loading") {
    return (
      <div className="flex flex-1 items-center justify-center py-12">
        <Spinner aria-label="Loading conversations" />
      </div>
    );
  }
  if (state === "error") {
    return (
      <EmptyState
        icon={RiInbox2Line}
        title="Conversations did not load"
        description="Check your connection and try again."
      />
    );
  }
  if (conversations.length === 0) {
    return search.trim() ? (
      <EmptyState icon={RiSearchLine} title="No matches" description={`Nothing in this view matches “${search}”.`} />
    ) : (
      <EmptyState icon={RiInbox2Line} title="No conversations" description={description} />
    );
  }

  return (
    <ScrollArea>
      <ConversationList view={view} conversations={conversations} openId={openId} me={me} drafted={drafted} />
    </ScrollArea>
  );
}

/*
 * NoConversation fills the conversation side of the inbox until one is
 * picked.
 */
export function NoConversation() {
  return (
    <Panel label="Conversation" className="flex-1">
      <EmptyState
        icon={RiChatSmile3Line}
        title="No conversation selected"
        description="Pick one from the list to read and reply."
      />
    </Panel>
  );
}
