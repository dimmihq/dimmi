/*
 * A thread with Operator: the teammate's questions, Operator's answers in
 * Markdown, the log of what it did to get there, reply drafts it wrote
 * with a way into the reply box, and the changes it proposed as cards to
 * decide. While it works, its log and text grow in place and the
 * composer offers Stop. Nothing said here reaches the customer.
 */
import { RiCheckLine, RiFileCopyLine, RiLockLine, RiReplyLine } from "@remixicon/react";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { AgentComposer } from "@/components/application/agent-chat/agent-composer";
import { Button } from "@/components/base/buttons/button";
import { useCompose } from "@/features/inbox/compose-bridge-context";
import { MessageBody } from "@/features/inbox/message-body";
import { ScrollArea } from "@/features/shell/page";
import { Reveal } from "@/features/shell/reveal";
import type { OperatorChangeset, OperatorMessage, OperatorStep, OperatorThreadView } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { ActivityLog } from "./activity-log";
import { ChangesetCard } from "./changeset-card";
import type { Scope } from "./scope";
import type { OperatorChat as Chat, Live } from "./use-operator-chat";

/*
 * useCopy copies a text and says so for a moment.
 */
function useCopy(text: string) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = setTimeout(() => setCopied(false), 1600);

    return () => clearTimeout(timer);
  }, [copied]);
  const copy = () => {
    void navigator.clipboard.writeText(text).then(() => setCopied(true));
  };

  return { copied, copy };
}

/*
 * DraftCard is a reply Operator drafted, framed like a file it wrote:
 * a header naming it, the text, and a way into the reply box.
 */
function DraftCard({ text }: { text: string }) {
  const put = useCompose();
  const { copied, copy } = useCopy(text);

  return (
    <section
      aria-label="Reply draft"
      className="flex flex-col overflow-hidden rounded-2xl border border-border-button-default bg-background-primary-default shadow-xs"
    >
      <header className="flex h-10 items-center gap-2 border-b border-separator-border ps-3 pe-1.5">
        <RiReplyLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="min-w-0 flex-1 truncate text-body-2-medium text-text-primary">Reply draft</span>
        <span className="text-caption-1-regular text-text-tertiary">Not sent</span>
        <button
          type="button"
          aria-label={copied ? "Copied" : "Copy draft"}
          onClick={copy}
          className="flex size-7 cursor-pointer items-center justify-center rounded-md text-foreground-icon-tertiary outline-none transition-colors duration-150 hover:bg-background-secondary-hover hover:text-foreground-icon-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {copied ? <RiCheckLine aria-hidden className="size-4" /> : <RiFileCopyLine aria-hidden className="size-4" />}
        </button>
      </header>
      <MessageBody text={text} className="px-3 py-2.5 text-body-2-regular text-text-primary" />
      {put ? (
        <footer className="flex px-3 pb-3">
          <Button size="small" leadingIcon={RiReplyLine} onClick={() => put(text)}>
            Use in reply
          </Button>
        </footer>
      ) : null}
    </section>
  );
}

/*
 * Extras are the drafts and changes behind an answer's steps.
 */
function Extras({
  steps,
  changesets,
  onDecide,
}: {
  steps: OperatorStep[];
  changesets: OperatorChangeset[];
  onDecide: Chat["decide"];
}) {
  return steps.map((step) => {
    if (step.draft) {
      return <DraftCard key={`draft:${step.summary}:${step.draft.length}`} text={step.draft} />;
    }
    const changeset = step.changeset_id ? changesets.find((c) => c.id === step.changeset_id) : undefined;

    return changeset ? <ChangesetCard key={changeset.id} changeset={changeset} onDecide={onDecide} /> : null;
  });
}

function Question({ text }: { text: string }) {
  return (
    <p className="-me-1.5 ms-auto w-fit max-w-[85%] rounded-2xl bg-background-primary-default px-3 py-[11px] text-start text-body-regular wrap-break-word whitespace-pre-wrap text-text-primary shadow-card">
      {text}
    </p>
  );
}

/*
 * Actions sit under a finished answer: copy, and why the answer is
 * short when it stopped or failed.
 */
function Actions({ message }: { message: OperatorMessage }) {
  const { copied, copy } = useCopy(message.body);

  return (
    <div className="flex items-center gap-1">
      {message.body ? (
        <button
          type="button"
          aria-label={copied ? "Copied" : "Copy answer"}
          onClick={copy}
          className="group flex cursor-pointer items-center rounded-lg bg-background-tertiary-default p-1.5 text-foreground-icon-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {copied ? <RiCheckLine aria-hidden className="size-4" /> : <RiFileCopyLine aria-hidden className="size-4" />}
        </button>
      ) : null}
      {message.status === "stopped" ? (
        <span className="ms-1 text-caption-1-regular text-text-tertiary">Stopped</span>
      ) : null}
    </div>
  );
}

function Answer({
  message,
  changesets,
  onDecide,
  fresh,
}: {
  message: OperatorMessage;
  changesets: OperatorChangeset[];
  onDecide: Chat["decide"];
  fresh: boolean;
}) {
  return (
    <div className="group/answer flex flex-col gap-3">
      <ActivityLog steps={message.steps.filter((s) => !s.draft && !s.changeset_id)} working={false} />
      {message.body ? (
        <MessageBody
          text={message.body}
          className={cx("text-body-regular text-text-primary", message.status === "failed" && "text-text-tertiary")}
        />
      ) : null}
      <Reveal animate={fresh} className="flex flex-col gap-3">
        <Extras steps={message.steps} changesets={changesets} onDecide={onDecide} />
      </Reveal>
      <Actions message={message} />
    </div>
  );
}

function Working({ live }: { live: Live }) {
  return (
    <>
      {live.question ? (
        <Reveal>
          <Question text={live.question} />
        </Reveal>
      ) : null}
      <div className="flex flex-col gap-3">
        <ActivityLog steps={live.steps.filter((s) => !s.draft && !s.changeset_id)} working={live.text === ""} />
        {live.text ? <MessageBody text={live.text} className="text-body-regular text-text-primary" /> : null}
      </div>
    </>
  );
}

/*
 * useFresh tells what arrived in the thread after it was first shown, so
 * new cards land with motion while the history shows at once.
 */
function useFresh(thread: OperatorThreadView | undefined) {
  const [known, setKnown] = useState<Set<string> | null>(null);
  useEffect(() => {
    if (thread && known === null) {
      setKnown(new Set([...thread.messages.map((m) => m.id), ...thread.changesets.map((c) => c.id)]));
    }
  }, [thread, known]);

  return (id: string) => known !== null && !known.has(id);
}

/*
 * useStickToBottom keeps the newest turn in view as it grows.
 */
function useStickToBottom(dependency: unknown) {
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (dependency !== undefined) {
      end.current?.scrollIntoView({ block: "end" });
    }
  }, [dependency]);

  return end;
}

function Status({ scope }: { scope: Scope }) {
  const Glyph = scope.icon;

  return (
    <>
      <span className="flex min-w-0 items-center gap-1">
        <RiLockLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="truncate text-body-2-medium text-text-secondary">Only your team sees this</span>
      </span>
      <span className="flex shrink-0 items-center gap-1">
        <Glyph aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="text-body-2-medium whitespace-nowrap text-text-secondary">{scope.label}</span>
      </span>
    </>
  );
}

export function OperatorChat({
  thread,
  chat,
  empty,
  placeholder,
  inputId,
  scope,
  wide = false,
}: {
  thread: OperatorThreadView | undefined;
  chat: Chat;
  empty: ReactNode;
  placeholder: string;
  inputId: string;
  scope: Scope;
  wide?: boolean;
}) {
  const [draft, setDraft] = useState("");
  const isFresh = useFresh(thread);
  const messages = thread?.messages ?? [];
  const lastQuestion = messages.filter((message) => message.role === "user").at(-1)?.body;
  const changesets = thread?.changesets ?? [];
  const own = changesets.filter((changeset) => changeset.proposed_to === null);
  const end = useStickToBottom(`${messages.length}:${chat.live?.text.length ?? 0}:${chat.live?.steps.length ?? 0}`);
  const blank = messages.length === 0 && !chat.live && own.length === 0;
  const column = cx("mx-auto w-full", wide ? "max-w-[720px] px-6" : "px-4");
  const send = () => {
    const question = draft.trim();
    if (question === "" || chat.busy) {
      return;
    }
    chat.ask(question);
    setDraft("");
  };

  return (
    <>
      <ScrollArea className="flex flex-col">
        <ol
          aria-label="Operator chat"
          className={cx(column, "flex flex-1 flex-col justify-end gap-5 pt-4 pb-6", blank && "justify-center")}
        >
          {own.map((changeset) => (
            <li key={changeset.id} className="flex flex-col gap-1.5">
              <span className="text-caption-1-semibold text-text-tertiary">Proposed when it arrived</span>
              <Reveal animate={isFresh(changeset.id)}>
                <ChangesetCard changeset={changeset} onDecide={chat.decide} />
              </Reveal>
            </li>
          ))}
          {messages.length === 0 && !chat.live ? <li>{empty}</li> : null}
          {messages.map((message) => (
            <li key={message.id}>
              {message.role === "user" ? (
                <Question text={message.body} />
              ) : (
                <Answer message={message} changesets={changesets} onDecide={chat.decide} fresh={isFresh(message.id)} />
              )}
            </li>
          ))}
          {chat.live ? (
            <li className="flex flex-col gap-5">
              <Working live={chat.live} />
            </li>
          ) : null}
          {chat.unsent && lastQuestion !== chat.unsent ? (
            <li>
              <Question text={chat.unsent} />
            </li>
          ) : null}
          {chat.failure ? (
            <li role="alert" className="text-body-2-regular text-text-error-primary">
              {chat.failure}
            </li>
          ) : null}
        </ol>
        <div ref={end} />
      </ScrollArea>
      <div className={cx(column, "shrink-0 pb-3", wide && "pb-4")}>
        <AgentComposer
          value={draft}
          onValueChange={setDraft}
          onSubmit={send}
          onStop={chat.stop}
          busy={chat.busy}
          messageCount={messages.length}
          placeholder={placeholder}
          inputId={inputId}
          attach={false}
          status={<Status scope={scope} />}
        />
      </div>
    </>
  );
}
