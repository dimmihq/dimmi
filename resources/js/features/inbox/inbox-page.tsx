/*
 * The inbox: the list of conversations for the chosen view beside the
 * open conversation, each in its own floating panel, resizable on the gap
 * between them. While a conversation is open the list steps aside unless
 * the screen is very wide, so the thread, its details and Operator have
 * room. On phones one of the two shows at a time. The list's search
 * looks through every conversation of the view, closed ones included.
 */
import { RiChatSmile3Line, RiInbox2Line, RiSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useParams } from "@tanstack/react-router";
import { useDeferredValue, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { Input } from "@/components/base/input/input";
import { Spinner } from "@/components/base/spinner/spinner";
import { PageOutlet } from "@/features/layout/page-outlet";
import { type InboxView, inboxViews } from "@/features/shell/navigation";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
import { type Counts, conversationsQuery, countsQuery } from "@/lib/api/inbox";
import { ConversationList } from "./conversation-list";
import { useInbox } from "./conversations";

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

export function InboxPage({ view }: { view: InboxView }) {
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
 * openCount is how many open conversations wait in the view, as the
 * sidebar counts them.
 */
function openCount(counts: Counts | undefined, view: InboxView): number | undefined {
  if (!counts) {
    return undefined;
  }
  if (view === "mentions") {
    return counts.views.mentions.unread;
  }

  return counts.views[view].open;
}

function ListPanel({ view, openId }: { view: InboxView; openId: string | undefined }) {
  const page = inboxViews[view];
  const { workspace } = useInbox();
  const [search, setSearch] = useState("");
  const query = useDeferredValue(search);
  const {
    data: conversations,
    isPending,
    isError,
  } = useQuery({
    ...conversationsQuery(workspace, view, query),
    enabled: workspace !== "",
  });
  const { data: counts } = useQuery({ ...countsQuery(workspace), enabled: workspace !== "" });
  const count = openCount(counts, view);

  return (
    <Panel label={`${page.label} conversations`} className="h-full w-full">
      <PageHeader title={page.label}>
        <Chip variant="caption" color="neutral">
          {count === undefined ? "Open" : `${count} open`}
        </Chip>
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
      <ListBody
        view={view}
        openId={openId}
        search={query}
        state={isPending ? "loading" : isError ? "error" : "ready"}
        conversations={conversations ?? []}
      />
    </Panel>
  );
}

function ListBody({
  view,
  openId,
  search,
  state,
  conversations,
}: {
  view: InboxView;
  openId: string | undefined;
  search: string;
  state: "loading" | "error" | "ready";
  conversations: NonNullable<Parameters<typeof ConversationList>[0]["conversations"]>;
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
      <EmptyState icon={RiInbox2Line} title="No conversations" description={inboxViews[view].description} />
    );
  }

  return (
    <ScrollArea>
      <ConversationList view={view} conversations={conversations} openId={openId} />
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
