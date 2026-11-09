/*
 * A thread with Operator: the teammate's questions, Operator's answers in
 * Markdown, the steps it took to get there, reply drafts it wrote with a
 * way into the reply box, and the changes it proposed as cards to
 * decide. While it works, its text and steps grow in place and the
 * composer offers Stop. Nothing said here reaches the customer.
 */
import { RiCheckLine, RiClipboardLine, RiErrorWarningLine, RiReplyLine } from "@remixicon/react";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { AgentComposer } from "@/components/application/agent-chat/agent-composer";
import { Button } from "@/components/base/buttons/button";
import { useCompose } from "@/features/inbox/compose-bridge-context";
import { MessageBody } from "@/features/inbox/message-body";
import { ScrollArea } from "@/features/shell/page";
import type { OperatorChangeset, OperatorMessage, OperatorStep, OperatorThreadView } from "@/lib/api/operator";
import { cx } from "@/utils/cx";

import { ChangesetCard } from "./changeset-card";
import type { OperatorChat as Chat, Live } from "./use-operator-chat";

/*
 * Steps lists what Operator did, one line each.
 */
function Steps({ steps, working }: { steps: OperatorStep[]; working: boolean }) {
  if (steps.length === 0 && !working) {
    return null;
  }

  return (
    <ul aria-label="Steps" className="flex flex-col gap-1">
      {steps.map((step) => (
        <li
          key={`${step.tool}:${step.summary}`}
          className="flex items-start gap-1.5 text-caption-1-regular text-text-tertiary"
        >
          {step.ok ? (
            <RiCheckLine aria-hidden className="mt-px size-3.5 shrink-0" />
          ) : (
            <RiErrorWarningLine aria-hidden className="mt-px size-3.5 shrink-0 text-text-error-primary" />
          )}
          <span className="min-w-0 truncate">{step.summary}</span>
        </li>
      ))}
      {working ? <li className="animate-pulse text-caption-1-regular text-text-tertiary">Working…</li> : null}
    </ul>
  );
}

/*
 * DraftCard is a reply Operator drafted, ready for the reply box.
 */
function DraftCard({ text }: { text: string }) {
  const put = useCompose();
  const [copied, setCopied] = useState(false);

  return (
    <section
      aria-label="Reply draft"
      className="flex flex-col gap-2 rounded-2xl border border-border-button-default bg-background-secondary-default p-3"
    >
      <span className="text-caption-1-semibold text-text-tertiary">Reply draft</span>
      <MessageBody text={text} className="text-body-2-regular text-text-primary" />
      <div className="flex gap-2">
        {put ? (
          <Button size="small" leadingIcon={RiReplyLine} onClick={() => put(text)}>
            Use in reply
          </Button>
        ) : null}
        <Button
          size="small"
          variant="secondary"
          leadingIcon={copied ? RiCheckLine : RiClipboardLine}
          onClick={() => {
            void navigator.clipboard.writeText(text).then(() => setCopied(true));
          }}
        >
          {copied ? "Copied" : "Copy"}
        </Button>
      </div>
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
    <p className="ms-auto w-fit max-w-[85%] rounded-2xl bg-background-primary-default px-3 py-2.5 text-body-regular wrap-break-word whitespace-pre-wrap text-text-primary shadow-card">
      {text}
    </p>
  );
}

/*
 * Worked folds a long list of finished steps behind one line.
 */
function Worked({ steps }: { steps: OperatorStep[] }) {
  if (steps.length <= 3) {
    return <Steps steps={steps} working={false} />;
  }

  return (
    <details className="group/worked">
      <summary className="cursor-pointer list-none text-caption-1-regular text-text-tertiary outline-none hover:text-text-secondary focus-visible:ring-2 focus-visible:ring-border-focus-ring">
        Worked through {steps.length} steps
      </summary>
      <div className="pt-1.5">
        <Steps steps={steps} working={false} />
      </div>
    </details>
  );
}

function Answer({
  message,
  changesets,
  onDecide,
}: {
  message: OperatorMessage;
  changesets: OperatorChangeset[];
  onDecide: Chat["decide"];
}) {
  return (
    <div className="flex flex-col gap-2.5 px-1">
      <Worked steps={message.steps.filter((s) => !s.draft && !s.changeset_id)} />
      <MessageBody
        text={message.body}
        className={cx("text-body-regular text-text-primary", message.status !== "done" && "text-text-tertiary")}
      />
      <Extras steps={message.steps} changesets={changesets} onDecide={onDecide} />
    </div>
  );
}

function Working({ live }: { live: Live }) {
  return (
    <>
      {live.question ? <Question text={live.question} /> : null}
      <div className="flex flex-col gap-2.5 px-1">
        <Steps steps={live.steps} working={live.text === ""} />
        {live.text ? <MessageBody text={live.text} className="text-body-regular text-text-primary" /> : null}
      </div>
    </>
  );
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

export function OperatorChat({
  thread,
  chat,
  empty,
  placeholder,
  inputId,
}: {
  thread: OperatorThreadView | undefined;
  chat: Chat;
  empty: ReactNode;
  placeholder: string;
  inputId: string;
}) {
  const [draft, setDraft] = useState("");
  const messages = thread?.messages ?? [];
  const changesets = thread?.changesets ?? [];
  const own = changesets.filter((changeset) => changeset.proposed_to === null);
  const end = useStickToBottom(`${messages.length}:${chat.live?.text.length ?? 0}:${chat.live?.steps.length ?? 0}`);
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
      <ScrollArea>
        <ol aria-label="Operator chat" className="flex flex-col gap-5 px-4 pt-2 pb-6">
          {own.map((changeset) => (
            <li key={changeset.id} className="flex flex-col gap-1.5">
              <span className="px-1 text-caption-1-semibold text-text-tertiary">Proposed when it arrived</span>
              <ChangesetCard changeset={changeset} onDecide={chat.decide} />
            </li>
          ))}
          {messages.length === 0 && !chat.live ? <li>{empty}</li> : null}
          {messages.map((message) => (
            <li key={message.id}>
              {message.role === "user" ? (
                <Question text={message.body} />
              ) : (
                <Answer message={message} changesets={changesets} onDecide={chat.decide} />
              )}
            </li>
          ))}
          {chat.live ? (
            <li className="flex flex-col gap-5">
              <Working live={chat.live} />
            </li>
          ) : null}
          {chat.failure ? (
            <li role="alert" className="px-1 text-body-2-regular text-text-error-primary">
              {chat.failure}
            </li>
          ) : null}
        </ol>
        <div ref={end} />
      </ScrollArea>
      <div className="shrink-0 px-3 pb-3">
        <AgentComposer
          value={draft}
          onValueChange={setDraft}
          onSubmit={send}
          onStop={chat.stop}
          busy={chat.busy}
          messageCount={messages.length}
          placeholder={placeholder}
          inputId={inputId}
          showStatus={false}
        />
      </div>
    </>
  );
}
