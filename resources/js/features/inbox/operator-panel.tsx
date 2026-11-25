/*
 * Operator is the team's own agent: it reads the case and talks it over
 * with the teammate, and nothing said here reaches the customer. Each
 * teammate keeps their own threads about a conversation: the latest
 * opens first, past ones are a click away, and a new one starts when the
 * subject changes. Asked for a brief, it reads the conversation and says
 * what the customer needs, what happened and the next step, and keeps
 * the brief on the conversation as a note for the whole team; it briefs
 * on its own when the conversation opens only if the workspace turned
 * that on, since every brief spends tokens, and keeps that one private.
 * Replies it drafts go into the reply box on request, and changes it
 * proposes wait as cards to apply, decline or undo.
 */
import { RiHistoryLine, RiPriceTag3Line, RiQuillPenLine, RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { OperatorSidePanel } from "@/features/operator/operator-side-panel";
import { conversationScope } from "@/features/operator/scope";
import { useOperatorChat } from "@/features/operator/use-operator-chat";
import {
  briefConversation,
  conversationThreadQuery,
  conversationThreadsQuery,
  operatorKeys,
  operatorSettingsQuery,
  operatorThreadQuery,
  startConversationThread,
} from "@/lib/api/operator";

export function OperatorPanel({
  workspace,
  conversation,
  onClose,
  className,
}: {
  workspace: string;
  conversation: string;
  onClose: () => void;
  className?: string;
}) {
  const client = useQueryClient();
  const [picked, setPicked] = useState<string | null>(null);
  const latest = useQuery({ ...conversationThreadQuery(workspace, conversation), enabled: picked === null });
  const chosen = useQuery({ ...operatorThreadQuery(workspace, picked ?? ""), enabled: picked !== null });
  const thread = picked === null ? latest.data : chosen.data;
  const key =
    picked === null ? operatorKeys.conversation(workspace, conversation) : operatorKeys.thread(workspace, picked);
  const { data: past = [] } = useQuery(conversationThreadsQuery(workspace, conversation));
  const { data: settings } = useQuery(operatorSettingsQuery(workspace));
  const chat = useOperatorChat(workspace, thread, key);
  const refreshList = () => client.invalidateQueries({ queryKey: operatorKeys.conversation(workspace, conversation) });
  const brief = (attach: boolean) =>
    void chat
      .run(null, (signal) => briefConversation(workspace, conversation, { thread: thread?.id, attach, signal }))
      .then(refreshList);
  const fresh = useMutation({
    mutationFn: () => startConversationThread(workspace, conversation),
    onSuccess: (started) => {
      client.setQueryData(operatorKeys.thread(workspace, started.id), started);
      setPicked(started.id);
    },
  });
  const briefed = useRef(false);
  const autoBrief = settings?.enabled && settings.brief_on_open && picked === null && thread?.messages.length === 0;

  useEffect(() => {
    if (autoBrief && !briefed.current) {
      briefed.current = true;
      void chat.run(null, (signal) => briefConversation(workspace, conversation, { signal }));
    }
  }, [autoBrief, chat, workspace, conversation]);

  return (
    <OperatorSidePanel
      thread={thread}
      chat={chat}
      past={past}
      enabled={settings?.enabled ?? true}
      starting={fresh.isPending}
      heading="Catch up on this conversation"
      blurb="Operator reads it and briefs you, drafts replies and proposes changes for you to apply."
      suggestions={[
        {
          icon: RiSparkling2Line,
          title: "Brief me",
          detail: "Says what the customer needs, what happened and the next step, and keeps it as a note for the team.",
          onPick: () => brief(true),
        },
        {
          icon: RiQuillPenLine,
          title: "Draft a reply",
          detail: "Writes a reply you can put in the reply box.",
          onPick: () => chat.ask("Draft a reply"),
        },
        {
          icon: RiPriceTag3Line,
          title: "Suggest tags and an owner",
          detail: "Proposes tags and who should take it, for you to apply.",
          onPick: () => chat.ask("Suggest tags and an owner"),
        },
        {
          icon: RiHistoryLine,
          title: "Has this customer written before?",
          detail: "Looks through the customer's earlier conversations.",
          onPick: () => chat.ask("Has this customer written before?"),
        },
      ]}
      placeholder="Ask Operator about this conversation"
      inputId="operator-composer-input"
      scope={conversationScope}
      onPick={setPicked}
      onNew={() => fresh.mutate()}
      onClose={onClose}
      className={className}
    />
  );
}
