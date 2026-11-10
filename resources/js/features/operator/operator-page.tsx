/*
 * Operator's own page, for questions about the whole workspace: how busy
 * the team is, how fast it answers, why the AI agent handed
 * conversations over, which articles are missing or wrong, or changes
 * across many conversations. The teammate's threads are listed on the
 * left, newest first, with a new one a click away; above the chat wait
 * the changes Operator proposed on its own, such as when it triages new
 * conversations, for any teammate who sees them to decide.
 */
import { RiAddLine, RiDeleteBin6Line, RiSparkling2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Badge } from "@/components/base/badges/badge";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { waitedFor } from "@/features/inbox/conversations";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import {
  type ChangesetDecision,
  decideChangeset,
  deleteOperatorThread,
  type OperatorThread,
  operatorKeys,
  operatorSettingsQuery,
  operatorThreadQuery,
  operatorThreadsQuery,
  pendingChangesetsQuery,
  startWorkspaceThread,
} from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { ChangesetCard } from "./changeset-card";
import { OperatorChat } from "./operator-chat";
import { workspaceScope } from "./scope";
import { failureText, useOperatorChat } from "./use-operator-chat";

const starters = [
  "How did support go over the last 30 days?",
  "Why did the AI agent hand conversations over this week?",
  "Which help articles do readers find least helpful?",
  "What are customers writing about most?",
];

function Threads({
  threads,
  current,
  onPick,
  onDelete,
}: {
  threads: OperatorThread[];
  current: string | null;
  onPick: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  if (threads.length === 0) {
    return <p className="px-4 py-2 text-body-2-regular text-text-tertiary">Your chats with Operator show here.</p>;
  }

  return (
    <ul aria-label="Chats" className="flex flex-col gap-0.5 px-2">
      {threads.map((thread) => (
        <li key={thread.id} className="group/thread relative flex items-center">
          <button
            type="button"
            onClick={() => onPick(thread.id)}
            aria-current={thread.id === current ? "true" : undefined}
            className={cx(
              "flex h-8 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-lg ps-2.5 pe-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              thread.id === current && "bg-background-secondary-hover",
            )}
          >
            <span
              className={cx(
                "min-w-0 flex-1 truncate text-body-2-regular",
                thread.id === current ? "text-text-primary" : "text-text-secondary",
              )}
            >
              {thread.title || "New chat"}
            </span>
            <Badge className="shrink-0 group-hover/thread:invisible group-focus-within/thread:invisible">
              {thread.last_message_at ? waitedFor(thread.last_message_at) : "new"}
            </Badge>
          </button>
          <IconButton
            size="small"
            icon={RiDeleteBin6Line}
            aria-label={`Delete ${thread.title || "chat"}`}
            className="absolute end-1 opacity-0 group-hover/thread:opacity-100 focus-visible:opacity-100"
            onClick={() => onDelete(thread.id)}
          />
        </li>
      ))}
    </ul>
  );
}

/*
 * Welcome opens a new chat: what Operator is for, and questions to start
 * with.
 */
function Welcome({ onAsk }: { onAsk: (question: string) => void }) {
  return (
    <Reveal>
      <div className="flex flex-col items-center gap-5 py-6 text-center">
        <span className="flex size-11 items-center justify-center rounded-full bg-background-primary-default shadow-card">
          <RiSparkling2Line aria-hidden className="size-5 text-foreground-icon-secondary" />
        </span>
        <div className="flex max-w-md flex-col gap-1">
          <h2 className="text-title-3-semibold text-text-primary">What should we look into?</h2>
          <p className="text-body-regular text-text-secondary">
            Ask about your workspace. Operator reads what you can see, and proposes changes for you to approve.
          </p>
        </div>
        <div className="flex max-w-xl flex-wrap justify-center gap-2">
          {starters.map((starter) => (
            <Button key={starter} size="small" variant="secondary" onClick={() => onAsk(starter)}>
              {starter}
            </Button>
          ))}
        </div>
      </div>
    </Reveal>
  );
}

function Chat({ workspace, thread }: { workspace: string; thread: string }) {
  const key = operatorKeys.thread(workspace, thread);
  const { data } = useQuery(operatorThreadQuery(workspace, thread));
  const chat = useOperatorChat(workspace, data, key);

  return (
    <OperatorChat
      thread={data}
      chat={chat}
      empty={<Welcome onAsk={chat.ask} />}
      placeholder="Ask Operator about your workspace"
      inputId="operator-page-input"
      scope={workspaceScope}
      wide
    />
  );
}

function Waiting({ workspace }: { workspace: string }) {
  const client = useQueryClient();
  const { data: pending = [] } = useQuery(pendingChangesetsQuery(workspace));
  const onDecide = async (id: string, decision: ChangesetDecision) => {
    await decideChangeset(workspace, id, decision);
    await client.invalidateQueries({ queryKey: ["workspace", workspace] });
  };
  const own = pending.filter((changeset) => changeset.proposed_to === null);
  if (own.length === 0) {
    return null;
  }

  return (
    <section
      aria-label="Waiting for review"
      className="mx-auto flex w-full max-w-[720px] flex-col gap-2 px-6 pt-2 pb-3"
    >
      <h2 className="text-caption-1-semibold text-text-tertiary">Proposed on its own · {own.length}</h2>
      {own.slice(0, 3).map((changeset) => (
        <ChangesetCard key={changeset.id} changeset={changeset} onDecide={onDecide} />
      ))}
    </section>
  );
}

export function OperatorPage() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const { data: threads = [] } = useQuery({ ...operatorThreadsQuery(workspace), enabled: workspace !== "" });
  const { data: settings } = useQuery({ ...operatorSettingsQuery(workspace), enabled: workspace !== "" });
  const [current, setCurrent] = useState<string | null>(null);
  const shown = current ?? threads[0]?.id ?? null;
  const refresh = () => client.invalidateQueries({ queryKey: operatorKeys.threads(workspace) });
  const start = useMutation({
    mutationFn: () => startWorkspaceThread(workspace),
    onSuccess: (thread) => {
      setCurrent(thread.id);
      void refresh();
    },
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteOperatorThread(workspace, id),
    onSuccess: (_, id) => {
      if (id === shown) {
        setCurrent(null);
      }
      void refresh();
    },
  });

  return (
    <div className="flex h-full min-w-0 flex-1 gap-2">
      <Panel label="Chats" className="hidden h-full w-64 shrink-0 md:flex">
        <PageHeader title="Operator">
          <IconButton size="small" icon={RiAddLine} aria-label="New chat" onClick={() => start.mutate()} />
        </PageHeader>
        <ScrollArea>
          <Threads threads={threads} current={shown} onPick={setCurrent} onDelete={(id) => remove.mutate(id)} />
        </ScrollArea>
      </Panel>
      <Panel label="Operator" className="h-full min-w-0 flex-1">
        <PageHeader title={threads.find((t) => t.id === shown)?.title || "Operator"}>
          <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={() => start.mutate()}>
            New chat
          </Button>
        </PageHeader>
        {settings && !settings.enabled ? (
          <p className="px-4 pt-2 text-body-2-regular text-text-secondary">
            Operator is switched off for this workspace. An admin can turn it on in Settings, Operator.
          </p>
        ) : (
          <>
            <Waiting workspace={workspace} />
            {shown ? (
              <Chat key={shown} workspace={workspace} thread={shown} />
            ) : (
              <div className="flex flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
                <RiSparkling2Line aria-hidden className="size-6 text-foreground-icon-tertiary" />
                <p className="max-w-sm text-body-2-regular text-text-secondary">
                  Operator is your team's own agent. It answers questions about your workspace, finds what is missing
                  from your help center and proposes changes you approve.
                </p>
                <Button size="small" leadingIcon={RiAddLine} loading={start.isPending} onClick={() => start.mutate()}>
                  Start a chat
                </Button>
                {start.error ? (
                  <p role="alert" className="text-body-2-regular text-text-error-primary">
                    {failureText(start.error)}
                  </p>
                ) : null}
              </div>
            )}
          </>
        )}
      </Panel>
    </div>
  );
}
