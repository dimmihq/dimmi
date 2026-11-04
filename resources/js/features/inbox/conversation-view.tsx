/*
 * One open conversation across three floating panels, resizable on the
 * gaps between them: the thread in the middle, the details beside it with
 * the moves a teammate makes on the case, and Operator on the right for
 * talking the case over with the team's own agent. Details and Operator
 * fold away from the header or by dragging them past their smallest size;
 * each browser remembers its layout. Details start open. On screens too
 * narrow for the side panels, Operator slides over the thread instead.
 * Opening a conversation reads it for the person, so it stops showing as
 * unread; a reply or note is added to the thread once the server has it.
 * While it is open the conversation follows its realtime channel, so a
 * teammate's reply shows without a reload; the header shows who else has
 * it open, and the thread says when the customer is typing.
 */
import { RiArrowLeftSLine, RiLayoutRightLine, RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Drawer } from "@/components/base/drawer/drawer";
import { ConversationViewers, TypingIndicator } from "@/features/realtime/collision";
import { Panel } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useEasing } from "@/features/shell/use-easing";
import { useWide } from "@/features/shell/use-wide";
import { addNote, type Conversation, markRead, messagesQuery, reply } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";
import { useConversationRealtime } from "@/lib/realtime/use-conversation";
import { cx } from "@/utils/cx";
import { statusLabel, useCase } from "./case";
import { customerName, subjectOf, toEntry, useInbox } from "./conversations";
import { DetailsPanel } from "./customer/details-panel";
import type { Mode } from "./modes";
import { OperatorPanel } from "./operator-panel";
import { OriginalMessage } from "./original-message";
import { PriorityMenu } from "./priority/priority-menu";
import { Thread } from "./thread";
import { useSidePanes } from "./use-side-panes";
import { useTypingReport } from "./use-typing-report";

const visible = { overflow: "visible" } as const;

/*
 * The panes' names, under which each browser remembers their sizes.
 */
const pane = { group: "dimmi:conversation", thread: "thread", details: "details", operator: "operator" } as const;
const pressed = "border-border-button-active bg-background-primary-active";

const notConnected =
  "I can't brief you on this conversation yet: Operator connects later. Ask your question and I'll say what I can.";

/*
 * useTimeline reads the conversation's timeline, marks it read once it
 * has loaded, and sends replies and notes, reading everything the inbox
 * shows again once the server has them.
 */
function useTimeline(workspace: string, conversation: Conversation) {
  const queryClient = useQueryClient();
  const mutationId = useMutationId();
  const { data: messages = [] } = useQuery(messagesQuery(workspace, conversation.id));
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
  const read = useMutation({
    mutationFn: () => markRead({ workspace, mutation: mutationId() }, conversation.id),
    onSuccess: refresh,
  });
  const unread = conversation.unread;
  const mark = read.mutate;
  useEffect(() => {
    if (unread) {
      mark();
    }
  }, [unread, mark]);

  const send = async (mode: Mode, text: string) => {
    const write = { workspace, mutation: mutationId() };
    await (mode === "note" ? addNote(write, conversation.id, text) : reply(write, conversation.id, text));
    await refresh();
  };

  return { entries: messages.map(toEntry), send };
}

export function ConversationView({
  view,
  workspace,
  conversation,
}: {
  view: string;
  workspace: string;
  conversation: Conversation;
}) {
  useConversationRealtime(conversation.id);
  const { userId } = useInbox();
  const typing = useTypingReport(workspace, conversation.id);
  const caze = useCase(workspace, conversation);
  const timeline = useTimeline(workspace, conversation);
  const customer = customerName(conversation.contact);
  const sideBySide = useWide(1024);
  const operatorDefault = useWide(1280);
  const sides = useSidePanes(
    pane.thread,
    { [pane.details]: 360, [pane.operator]: 380 },
    () => document.getElementById(pane.group)?.clientWidth ?? 0,
  );
  const [detailsOpen, setDetailsOpen] = useState(true);
  const [operatorOpen, setOperatorOpen] = useState(operatorDefault);
  const [drawer, setDrawer] = useState(false);
  const [easing, ease] = useEasing();
  const [original, setOriginal] = useState<string | null>(null);

  const toggle = (id: string) => ease(() => sides.toggle(id));
  const toggleOperator = () => (sideBySide ? toggle(pane.operator) : setDrawer(!drawer));
  const operatorShown = sideBySide ? operatorOpen : drawer;

  const thread = (
    <Panel label="Conversation" className="h-full w-full">
      <header className="flex h-12 shrink-0 items-center justify-between gap-3 ps-2.5 pe-2.5 pt-[7px]">
        <div className="flex min-w-0 items-center gap-2">
          <Link
            to="/d/inbox/$view"
            params={{ view }}
            aria-label="Back to the list"
            className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring 2xl:hidden"
          >
            <RiArrowLeftSLine aria-hidden className="size-5" />
          </Link>
          <h1 className="min-w-0 truncate ps-1.5 text-body-medium text-text-primary">{subjectOf(conversation)}</h1>
          <Chip variant="caption" color="neutral">
            #{conversation.number} · {statusLabel[conversation.status]}
          </Chip>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <ConversationViewers conversation={conversation.id} self={userId} />
          <PriorityMenu priority={conversation.priority} onChange={caze.setPriority} />
          {sideBySide ? (
            <IconButton
              size="small"
              icon={RiLayoutRightLine}
              className={cx(detailsOpen && pressed)}
              aria-label={detailsOpen ? "Hide details" : "Show details"}
              aria-pressed={detailsOpen}
              onClick={() => toggle(pane.details)}
            />
          ) : null}
          <IconButton
            size="small"
            icon={RiSparkling2Line}
            className={cx(operatorShown && pressed)}
            aria-label="Operator"
            aria-pressed={operatorShown}
            onClick={toggleOperator}
          />
        </div>
      </header>
      <Thread
        entries={timeline.entries}
        customer={customer}
        channel={conversation.channel.name}
        onSend={timeline.send}
        onAskOperator={toggleOperator}
        onTyping={typing}
        onOpenOriginal={setOriginal}
        status={<TypingIndicator conversation={conversation.id} nameOf={() => customer} />}
      />
      {original ? (
        <OriginalMessage
          workspace={workspace}
          conversation={conversation.id}
          message={original}
          onClose={() => setOriginal(null)}
        />
      ) : null}
    </Panel>
  );

  if (!sideBySide) {
    return (
      <div className="flex h-full min-w-0 flex-1">
        {thread}
        <Drawer
          isOpen={drawer}
          onOpenChange={setDrawer}
          placement="end"
          size="md"
          aria-label="Operator"
          className="p-0"
        >
          <OperatorPanel briefing={notConnected} onClose={() => setDrawer(false)} className="h-full" />
        </Drawer>
      </div>
    );
  }

  return (
    <PaneGroup
      id={pane.group}
      panelIds={[pane.thread, pane.details, pane.operator]}
      easing={easing}
      groupRef={sides.group}
    >
      <Pane id={pane.thread} minSize={360} style={visible} className="flex h-full">
        {thread}
      </Pane>
      <PaneHandle hidden={!detailsOpen} />
      <Pane
        id={pane.details}
        defaultSize={360}
        minSize={300}
        maxSize={560}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          sides.remember(pane.details)(size);
          setDetailsOpen(size.inPixels > 0);
        }}
        style={visible}
        className={cx("flex h-full", !detailsOpen && "invisible")}
      >
        <DetailsPanel
          workspace={workspace}
          conversation={conversation}
          caze={caze}
          onClose={() => toggle(pane.details)}
        />
      </Pane>
      <PaneHandle hidden={!operatorOpen} />
      <Pane
        id={pane.operator}
        defaultSize={operatorDefault ? 380 : 0}
        minSize={300}
        maxSize={600}
        collapsible
        groupResizeBehavior="preserve-pixel-size"
        onResize={(size) => {
          sides.remember(pane.operator)(size);
          setOperatorOpen(size.inPixels > 0);
        }}
        style={visible}
        className={cx("flex h-full", !operatorOpen && "invisible")}
      >
        <OperatorPanel briefing={notConnected} onClose={() => toggle(pane.operator)} className="h-full w-full" />
      </Pane>
    </PaneGroup>
  );
}
