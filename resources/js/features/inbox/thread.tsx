/*
 * The conversation itself, laid out like BoardUI's chat: the customer's
 * messages on the left on the tertiary surface, the team's side on the
 * right, Dimmi's replies tagged as the AI agent, and events as centred
 * pills. What Operator adds is a full-width card across the thread, so
 * the team's own agent never reads as part of the exchange with the
 * customer. Teammates' notes sit on the team's side with a dashed edge
 * and a lock. A message the workspace's redaction rules changed says how
 * many items they removed. Markdown is rendered and a customer's plain
 * text mail is shown as written. An email whose quoted thread was cut, or
 * whose long body was cut short, offers its full message below it. A reply
 * that never reached the customer says so, with the reason, and can be
 * sent again. An action the agent asked to run shows as a card the team
 * approves or declines, and as one line once decided. Hovering a message
 * offers a link to it, editing one's own note and deleting; a deleted
 * message stays as a line saying who deleted
 * it. A reply the customer has seen says when. Every time shows its full
 * date on hover, and reads in the customer's zone when the person
 * chooses; with events hidden, each run of them folds into one line.
 * Below the thread sits the composer; R and N open it for a reply or
 * a note from anywhere, and O opens or closes Operator.
 */
import { RiErrorWarningLine, RiLock2Line, RiSparkling2Line, RiTimeLine } from "@remixicon/react";
import { type ReactNode, useCallback, useEffect, useRef, useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { DimmiIconSmall } from "@/features/brand/logo";
import { avatarColor } from "@/features/shell/avatar-color";
import { ScrollArea } from "@/features/shell/page";
import { cx } from "@/utils/cx";

import { useComposeTarget } from "./compose-bridge-context";
import { Composer, type Send } from "./composer";
import type { ThreadEntry } from "./conversations";
import { LinkPolicyProvider } from "./links/link-policy";
import { MessageBody } from "./message-body";
import { MessageFiles } from "./message-files";
import type { Mode } from "./modes";
import { ThreadSkeleton } from "./skeletons";
import { ActionCallCard } from "./thread/action-call-card";
import { foldEvents, foldedText } from "./thread/fold-events";
import { MessageActions, NoteEditor } from "./thread/message-actions";
import { ReactionChips } from "./thread/reactions";
import { ThreadZone } from "./thread/thread-zone";
import { useMessageMoves } from "./thread/use-message-moves";
import { When } from "./thread/when";
import type { Report } from "./use-typing-report";

type Said = Extract<ThreadEntry, { author: string }>;

const bubble = "w-fit max-w-[min(520px,calc(100%-24px))] rounded-[var(--radius-2-5xl)] p-3";

function Pill({ children }: { children: ReactNode }) {
  return (
    <p className="flex justify-center px-6">
      <span className="rounded-full bg-background-tertiary-default px-[7px] py-1 text-center text-body-2-medium text-text-secondary">
        {children}
      </span>
    </p>
  );
}

function Mark({ entry }: { entry: Said }) {
  if (entry.kind === "dimmi") {
    return <DimmiIconSmall className="size-4 text-text-primary" />;
  }
  if (entry.kind === "note") {
    return <RiLock2Line aria-hidden className="size-4 text-foreground-icon-tertiary" />;
  }

  return <Avatar size="xs" color={avatarColor(entry.author)} initials={entry.author.charAt(0)} />;
}

function At({ entry }: { entry: Said }) {
  const style = "text-body-2-regular text-text-tertiary";

  return entry.iso ? <When iso={entry.iso} className={style} /> : <span className={style}>{entry.at}</span>;
}

function Tag({ entry }: { entry: Said }) {
  return (
    <span className="my-1 flex items-center gap-1.5 text-body-2-medium text-text-secondary">
      <Mark entry={entry} />
      {entry.author}
      {entry.kind === "dimmi" ? (
        <Chip variant="caption" color="neutral" className="py-0">
          AI agent
        </Chip>
      ) : null}
      <At entry={entry} />
      {entry.redacted ? (
        <span className="text-body-2-regular text-text-tertiary">
          · {entry.redacted === 1 ? "1 item redacted" : `${entry.redacted} items redacted`}
        </span>
      ) : null}
      {entry.edited ? <span className="text-body-2-regular text-text-tertiary">Edited</span> : null}
      {entry.held ? (
        <span className="flex items-center gap-0.5 text-body-2-regular text-text-tertiary">
          <RiTimeLine aria-hidden className="size-3.5" />
          Sending
        </span>
      ) : null}
      {entry.seenAt ? (
        <When iso={entry.seenAt} prefix="Seen " className="text-body-2-regular text-text-tertiary" />
      ) : null}
    </span>
  );
}

function OperatorCard({ entry, customer }: { entry: Said; customer: string }) {
  return (
    <article
      aria-label={`Internal note from ${entry.author}`}
      className="mx-3 flex flex-col gap-2 rounded-[var(--radius-2-5xl)] bg-background-primary-default p-3 shadow-card"
    >
      <header className="flex items-center gap-1.5 text-body-2-medium text-text-primary">
        <RiSparkling2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        {entry.author}
        <Chip variant="caption" color="soft" className="py-0">
          Internal
        </Chip>
        <span className="ms-auto">
          <At entry={entry} />
        </span>
      </header>
      <MessageBody text={entry.text} className="text-body-regular text-text-primary" />
      <p className="flex items-center gap-1 text-body-2-regular text-text-tertiary">
        <RiLock2Line aria-hidden className="size-3.5" />
        Not visible to {customer}
      </p>
    </article>
  );
}

type Moves = ReturnType<typeof useMessageMoves>;

/*
 * Follow-ups are what a message offers below it: its full original, and
 * sending it again when it was not delivered.
 */
interface FollowUps {
  onOpenOriginal?: (id: string) => void;
  onRetry?: (id: string) => void;
}

const linkButton =
  "rounded-md text-body-2-medium text-text-secondary underline outline-none hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function More({ entry, onOpen }: { entry: Said; onOpen?: (id: string) => void }) {
  if (!onOpen || !(entry.original || entry.truncated)) {
    return null;
  }

  return (
    <span className="flex items-center gap-1.5 text-body-2-regular text-text-tertiary">
      {entry.truncated ? "Shortened ·" : null}
      <button type="button" onClick={() => onOpen(entry.id)} className={linkButton}>
        Show full message
      </button>
    </span>
  );
}

function Undelivered({ entry, onRetry }: { entry: Said; onRetry?: (id: string) => void }) {
  if (entry.failed === null) {
    return null;
  }

  return (
    <span className="flex max-w-[min(520px,calc(100%-24px))] items-start gap-1.5 text-body-2-regular text-text-error-primary">
      <RiErrorWarningLine aria-hidden className="mt-0.5 size-3.5 shrink-0" />
      <span>
        Not delivered{entry.failed ? `: ${entry.failed}` : "."}
        {onRetry ? (
          <>
            {" "}
            <button type="button" onClick={() => onRetry(entry.id)} className={linkButton}>
              Send again
            </button>
          </>
        ) : null}
      </span>
    </span>
  );
}

function Entry({
  entry,
  conversation,
  customer,
  moves,
  follow,
}: {
  entry: ThreadEntry;
  conversation: string;
  customer: string;
  moves: Moves;
  follow: FollowUps;
}) {
  if (entry.kind === "event") {
    return <Pill>{entry.text}</Pill>;
  }
  if (entry.kind === "call") {
    return <ActionCallCard conversation={conversation} call={entry.call} />;
  }
  if (entry.kind === "note" && entry.author === "Operator") {
    return <OperatorCard entry={entry} customer={customer} />;
  }

  return (
    <>
      <SaidEntry entry={entry} customer={customer} moves={moves} follow={follow} />
      {entry.call ? (
        <div className="mt-3">
          <ActionCallCard conversation={conversation} call={entry.call} />
        </div>
      ) : null}
    </>
  );
}

/*
 * bubbleClass styles a message's bubble by who wrote it.
 */
function bubbleClass(entry: Said, editing: boolean): string {
  const ours = entry.kind !== "customer";
  const note = entry.kind === "note";

  return cx(
    bubble,
    "text-body-regular text-text-primary",
    !ours && "bg-background-tertiary-default",
    ours && "bg-background-primary-default shadow-card",
    note && "border border-status-yellow-text/40",
    editing && "w-full",
  );
}

/*
 * EntryActions sits on the top edge of a message's bubble on hover: a
 * link to it, editing one's own note, deleting, and reacting to a note.
 */
function EntryActions({
  entry,
  customer,
  moves,
  editing,
  onEdit,
}: {
  entry: Said;
  customer: string;
  moves: Moves;
  editing: boolean;
  onEdit: () => void;
}) {
  const ours = entry.kind !== "customer";
  const note = entry.kind === "note";
  const mine = Boolean(entry.authorId) && entry.authorId === moves.me;
  const kind = note ? "note" : ours ? "reply" : "customer";

  return (
    <div className={cx("absolute -top-4 z-10 w-max", ours ? "end-3" : "start-3")}>
      <MessageActions
        kind={kind}
        customer={customer}
        canEdit={note && mine && !editing}
        canDelete={mine || moves.moderates}
        onCopy={() => moves.link(entry.id)}
        onEdit={onEdit}
        onDelete={() => moves.remove(entry.id)}
        onReact={note ? (emoji) => void moves.react(entry.id, emoji, false) : undefined}
      />
    </div>
  );
}

function SaidEntry({
  entry,
  customer,
  moves,
  follow,
}: {
  entry: Said;
  customer: string;
  moves: Moves;
  follow: FollowUps;
}) {
  const [editing, setEditing] = useState(false);
  const ours = entry.kind !== "customer";
  const note = entry.kind === "note";

  return (
    <article
      id={entry.id}
      aria-label={note ? `Internal note from ${entry.author}` : `Message from ${entry.author}`}
      className={cx("group relative flex flex-col px-3", ours ? "items-end" : "items-start")}
    >
      <div
        className={cx(
          "relative flex max-w-[min(520px,calc(100%-24px))] flex-col",
          ours ? "items-end" : "items-start",
          editing && "w-full",
        )}
      >
        <EntryActions
          entry={entry}
          customer={customer}
          moves={moves}
          editing={editing}
          onEdit={() => setEditing(true)}
        />
        <div className={cx(bubbleClass(entry, editing), "max-w-full", !editing && entry.text === "" && "hidden")}>
          {editing ? (
            <NoteEditor
              text={entry.text}
              onCancel={() => setEditing(false)}
              onSave={async (body) => {
                await moves.edit(entry.id, body);
                setEditing(false);
              }}
            />
          ) : (
            <MessageBody text={entry.text} format={entry.format} fromCustomer={!ours} mentions={entry.mentions} />
          )}
          {note ? <span className="sr-only"> Not visible to {customer}.</span> : null}
        </div>
        <MessageFiles files={entry.files} ours={ours} />
      </div>
      <ReactionChips
        reactions={entry.reactions ?? []}
        me={moves.me}
        onToggle={(emoji, remove) => void moves.react(entry.id, emoji, remove)}
      />
      <Tag entry={entry} />
      <More entry={entry} onOpen={follow.onOpenOriginal} />
      <Undelivered entry={entry} onRetry={follow.onRetry} />
      {note ? (
        <span aria-hidden className="text-body-2-regular text-text-tertiary">
          Not visible to {customer}
        </span>
      ) : null}
    </article>
  );
}

/*
 * useComposerKeys switches the composer, or opens Operator, from the
 * keyboard: R, N and O, unless the person is typing somewhere.
 */
/*
 * fileChannels are the channels a reply can carry files on; the composer
 * offers files only there.
 */
const fileChannels = ["widget", "email"];

function useComposerKeys(keys: Record<string, () => void>) {
  useEffect(() => {
    const press = (event: KeyboardEvent) => {
      const target = event.target;
      const typing = target instanceof Element && target.closest("input, textarea, select, [contenteditable='true']");
      const action = keys[event.key.toLowerCase()];
      if (typing || event.metaKey || event.ctrlKey || event.altKey || !action) {
        return;
      }
      event.preventDefault();
      action();
    };
    document.addEventListener("keydown", press);

    return () => document.removeEventListener("keydown", press);
  }, [keys]);
}

export function Thread({
  conversation,
  entries,
  loading = false,
  customer,
  channel,
  channelType,
  onSend,
  onAskOperator,
  onTyping,
  onOpenOriginal,
  onRetry,
  status,
  zone = null,
  hideEvents = false,
}: {
  conversation: string;
  entries: ThreadEntry[];
  loading?: boolean;
  customer: string;
  channel: string;
  channelType?: string;
  onSend: Send;
  onAskOperator: () => void;
  onTyping: Report;
  onOpenOriginal?: (id: string) => void;
  onRetry?: (id: string) => void;
  status?: ReactNode;
  zone?: string | null;
  hideEvents?: boolean;
}) {
  const moves = useMessageMoves(conversation);
  const [mode, setMode] = useState<Mode | null>(null);
  const [incoming, setIncoming] = useState<string | null>(null);
  const [focusKey, setFocusKey] = useState(0);
  const ask = useRef(onAskOperator);
  useEffect(() => {
    ask.current = onAskOperator;
  }, [onAskOperator]);
  const [keys] = useState(() => {
    const write = (next: Mode) => {
      setMode(next);
      setFocusKey((before) => before + 1);
    };

    return { r: () => write("reply"), n: () => write("note"), o: () => ask.current() };
  });
  useComposerKeys(keys);
  useComposeTarget((text) => {
    setIncoming(text);
    keys.r();
  });
  const used = useCallback(() => setIncoming(null), []);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <ScrollArea className="pt-2 pb-4">
        {loading ? (
          <ThreadSkeleton />
        ) : (
          <ThreadZone.Provider value={zone}>
            <LinkPolicyProvider channelType={channelType}>
              <ol className="mx-auto flex max-w-3xl flex-col gap-3">
                {foldEvents(entries, hideEvents).map((entry) => (
                  <li key={entry.id}>
                    {entry.kind === "folded" ? (
                      <Pill>{foldedText(entry.count)}</Pill>
                    ) : (
                      <Entry
                        entry={entry}
                        conversation={conversation}
                        customer={customer}
                        moves={moves}
                        follow={{ onOpenOriginal, onRetry }}
                      />
                    )}
                  </li>
                ))}
              </ol>
            </LinkPolicyProvider>
          </ThreadZone.Provider>
        )}
      </ScrollArea>
      <div className="shrink-0 px-3 pb-3">
        {status ? <div className="mx-auto w-full max-w-3xl px-4 pb-1.5">{status}</div> : null}
        <Composer
          conversation={conversation}
          mode={mode}
          focusKey={focusKey}
          customer={customer}
          channel={channel}
          canAttach={fileChannels.includes(channelType ?? "")}
          onSend={onSend}
          onOpen={(next) => (next === "reply" ? keys.r() : keys.n())}
          onAskOperator={() => ask.current()}
          onTyping={onTyping}
          incoming={incoming}
          onIncomingUsed={used}
        />
      </div>
    </div>
  );
}
