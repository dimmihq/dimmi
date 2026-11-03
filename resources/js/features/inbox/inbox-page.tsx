/*
 * The inbox: the list of conversations for the chosen view beside the
 * open conversation, each in its own floating panel, resizable on the gap
 * between them. While a conversation is open the list steps aside unless
 * the screen is very wide, so the thread, its details and Operator have
 * room. On phones one of the two shows at a time.
 */
import { RiChatSmile3Line, RiInbox2Line } from "@remixicon/react";
import { useParams } from "@tanstack/react-router";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { PageOutlet } from "@/features/layout/page-outlet";
import { type InboxView, inboxViews } from "@/features/shell/navigation";
import { EmptyState, PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
import { ConversationList } from "./conversation-list";
import { conversationsIn } from "./conversations";

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

function ListPanel({ view, openId }: { view: InboxView; openId: string | undefined }) {
  const page = inboxViews[view];
  const conversations = conversationsIn(view);

  return (
    <Panel label={`${page.label} conversations`} className="h-full w-full">
      <PageHeader title={page.label}>
        <Chip variant="caption" color="neutral">
          Open
        </Chip>
      </PageHeader>
      {conversations.length > 0 ? (
        <ScrollArea>
          <ConversationList view={view} conversations={conversations} openId={openId} />
        </ScrollArea>
      ) : (
        <EmptyState icon={RiInbox2Line} title="No conversations" description={page.description} />
      )}
    </Panel>
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
