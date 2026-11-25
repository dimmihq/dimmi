/*
 * The reply and note composer, on BoardUI's composer panel. It rests as a
 * slim bar under the thread so the conversation keeps the room; choosing
 * Reply or Note (or pressing R or N) unfolds it into the full panel, which
 * folds back into the bar once its message is sent. The panel has the
 * BoardUI tab strip behind its top edge saying who will see the message,
 * a field that grows with what is typed with formatting kept as
 * Markdown, and a toolbar with the add menu (saved replies, help articles,
 * feature requests, changelog entries, emoji and GIFs, dropped into the
 * text) and a way to ask Operator. Typing # and a few letters finds a
 * saved reply too, an at sign searches all of that (and, in a note,
 * teammates to mention), and a colon finds emoji. A saved reply's variables are filled in for the
 * customer as it is inserted, and its macro actions wait beside the
 * draft and apply to the conversation once the message is sent. Replies cannot carry files yet, so the composer offers none
 * rather than show files that would never be sent. Sending goes to the
 * server; a sent draft empties, a refused one stays with the reason under
 * it. Each draft, reply and note apart, is kept on the server and in the
 * browser, so it survives leaving, reloading and another device. Each
 * time Reply or Note is chosen the field
 * takes the focus, sized to the draft it holds. While a reply is being
 * written the composer says so through onTyping, and says it stopped
 * once the reply is sent or the field is left.
 */
import { RiArrowUpLine, RiChatQuoteLine, RiLock2Line, RiSparkling2Line, RiStickyNoteLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { type ReactNode, useEffect, useState } from "react";

import { Kbd } from "@/components/base/kbd/kbd";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { QueuedMacro } from "@/features/saved-replies/queued-macro";
import { useMacro } from "@/features/saved-replies/use-macro";
import { type Message, mentionables } from "@/lib/api/inbox";
import { savedRepliesQuery } from "@/lib/api/saved-replies";
import { cx } from "@/utils/cx";

import { AddMenu } from "./add-menu";
import { ComposerField } from "./composer/composer-field";
import { preloadEditor } from "./composer/load-editor";
import { toMentionables } from "./composer/mentionable";
import { useGifs } from "./composer/use-gifs";
import { useInbox } from "./conversations";
import { DraftConflict } from "./drafts/draft-conflict";
import { useDraft } from "./drafts/use-draft";
import { searchLibrary } from "./library";
import { type Mode, modes } from "./modes";
import { AttachButton, ReplyFileChips } from "./reply-files";
import { SendMenu } from "./send-menu";
import { useSocialWindow } from "./social-state";
import { SocialChip, SocialNotice } from "./social-window";
import { UndoToast } from "./undo/undo-toast";
import { type Held, useHeld } from "./undo/use-held";
import { type ReplyFiles, useReplyFiles } from "./use-reply-files";
import type { Report } from "./use-typing-report";
import { outgoing, useTemplateReply, useWhatsAppWindow } from "./whatsapp-state";
import { ClosedWindow, TemplateCard, WindowChip } from "./whatsapp-window";

/*
 * Send sends what the composer holds as a reply or a note, and for a
 * reply what then happens to the conversation: close or snooze.
 */
export type Send = (
  mode: Mode,
  text: string,
  then?: "close" | "snooze",
  files?: string[],
) => Promise<Message | undefined>;

const surface =
  "rounded-3xl bg-background-primary-default shadow-[0_1px_0.5px_rgba(0,0,0,0.02),0_4px_2px_rgba(0,0,0,0.02)]";
const settle = [0.22, 1, 0.36, 1] as const;

export function Composer({
  conversation,
  mode,
  focusKey,
  customer,
  channel,
  channelType = "",
  canAttach = false,
  onSend,
  onOpen,
  onAskOperator,
  onTyping,
  incoming = null,
  onIncomingUsed,
  onSent,
}: {
  conversation: string;
  mode: Mode | null;
  focusKey: number;
  customer: string;
  channel: string;
  channelType?: string;
  canAttach?: boolean;
  onSend: Send;
  onOpen: (mode: Mode) => void;
  onAskOperator: () => void;
  onTyping: Report;
  incoming?: string | null;
  onIncomingUsed?: () => void;
  onSent?: () => void;
}) {
  const still = useReducedMotion();
  const { workspace } = useInbox();
  const [restored, setRestored] = useState<string | null>(null);
  const held = useHeld(workspace, conversation, (body) => {
    setRestored(body);
    onOpen("reply");
  });
  const taken = () => {
    setRestored(null);
    onIncomingUsed?.();
  };
  useEffect(preloadEditor, []);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-2">
      <AnimatePresence initial={false}>
        {held.message || held.failure ? (
          <motion.div
            key="held"
            initial={still ? false : { opacity: 0, height: 0, y: 4, filter: "blur(6px)" }}
            animate={{ opacity: 1, height: "auto", y: 0, filter: "blur(0px)" }}
            exit={still ? { opacity: 0 } : { opacity: 0, height: 0, filter: "blur(6px)" }}
            transition={{ duration: 0.38, ease: settle }}
          >
            <UndoToast held={held} />
          </motion.div>
        ) : null}
      </AnimatePresence>
      <AnimatePresence initial={false} mode="wait">
        {mode ? (
          <motion.div
            key="open"
            initial={{ opacity: 0, height: 52, filter: "blur(2px)" }}
            animate={{ opacity: 1, height: "auto", filter: "blur(0px)" }}
            transition={still ? { duration: 0 } : { duration: 0.3, ease: settle }}
            className="overflow-hidden"
          >
            <Panel
              conversation={conversation}
              mode={mode}
              focusKey={focusKey}
              customer={customer}
              channel={channel}
              channelType={channelType}
              canAttach={canAttach}
              onSend={onSend}
              onModeChange={onOpen}
              onAskOperator={onAskOperator}
              onTyping={onTyping}
              incoming={restored ?? incoming}
              onIncomingUsed={taken}
              held={held}
              onSent={onSent}
            />
          </motion.div>
        ) : (
          <motion.div
            key="closed"
            exit={{ opacity: 0, filter: "blur(2px)" }}
            transition={still ? { duration: 0 } : { duration: 0.12, ease: "easeIn" }}
          >
            <Bar customer={customer} onOpen={onOpen} />
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function Bar({ customer, onOpen }: { customer: string; onOpen: (mode: Mode) => void }) {
  return (
    <div className={cx(surface, "flex items-center gap-1 p-2")}>
      <button
        type="button"
        onClick={() => onOpen("reply")}
        className="flex h-9 min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-full ps-2.5 pe-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiChatQuoteLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="min-w-0 flex-1 truncate text-body-regular text-text-tertiary">Reply to {customer}…</span>
        <Kbd>R</Kbd>
      </button>
      <button
        type="button"
        onClick={() => onOpen("note")}
        className="flex h-9 shrink-0 cursor-pointer items-center gap-2 rounded-full ps-2.5 pe-2 outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiStickyNoteLine aria-hidden className="size-4 text-foreground-icon-secondary" />
        <span className="text-body-medium text-text-secondary">Note</span>
        <Kbd>N</Kbd>
      </button>
    </div>
  );
}

/*
 * useSend sends what the composer holds: the text and, for a reply, its
 * uploaded files, and tells onSent what text went. It waits for every
 * file to finish uploading, and keeps the text when sending fails so
 * nothing typed is lost.
 */
function useSend({
  mode,
  draft,
  files,
  held,
  onSend,
  onTyping,
  onSent,
}: {
  mode: Mode;
  draft: ReturnType<typeof useDraft>;
  files: ReplyFiles;
  held: Held;
  onSend: Send;
  onTyping: Report;
  onSent: (text: string, then?: "close" | "snooze") => void;
}) {
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const note = mode === "note";
  const attaching = !note && files.files.length > 0;
  const text = draft.body.trim();
  const ready = (text !== "" || (attaching && files.ids.length > 0)) && !(attaching && files.uploading);

  const send = async (then?: "close" | "snooze") => {
    if (!ready || sending) {
      return;
    }
    setSending(true);
    setFailure(null);
    onTyping(false);
    draft.pause();
    try {
      const sent = await onSend(mode, text, note ? undefined : then, attaching ? files.ids : undefined);
      onSent(text, note ? undefined : then);
      draft.sent();
      files.clear();
      held.hold(sent);
    } catch (error) {
      draft.edit(draft.body);
      setFailure(error instanceof Error ? error.message : "Sending failed. Try again.");
    } finally {
      setSending(false);
    }
  };

  return { send, sending, failure, ready };
}

function Panel({
  conversation,
  mode,
  focusKey,
  customer,
  channel,
  channelType = "",
  canAttach = false,
  onSend,
  onModeChange,
  onAskOperator,
  onTyping,
  incoming,
  onIncomingUsed,
  held,
  onSent,
}: {
  conversation: string;
  mode: Mode;
  focusKey: number;
  customer: string;
  channel: string;
  channelType?: string;
  canAttach?: boolean;
  onSend: Send;
  onModeChange: (mode: Mode) => void;
  onAskOperator: () => void;
  onTyping: Report;
  incoming: string | null;
  onIncomingUsed?: () => void;
  held: Held;
  onSent?: () => void;
}) {
  const { workspace, userId, me } = useInbox();
  const client = useQueryClient();
  const macro = useMacro(workspace, conversation, me);
  const drafts = { reply: useDraft(workspace, conversation, "reply"), note: useDraft(workspace, conversation, "note") };
  const draft = drafts[mode];
  const note = mode === "note";
  const insert = (text: string) => draft.edit(draft.body ? `${draft.body}\n\n${text}` : text);
  const takeIncoming = drafts.reply.edit;
  useEffect(() => {
    if (incoming !== null) {
      takeIncoming(incoming);
      onIncomingUsed?.();
    }
  }, [incoming, takeIncoming, onIncomingUsed]);
  const addEmoji = (emoji: string) => draft.edit(`${draft.body}${emoji}`);
  const gifs = useGifs(workspace, userId, insert);
  const files = useReplyFiles(workspace);
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  const attachable = !note && canAttach;
  const whatsapp = useWhatsAppWindow(workspace, conversation, channelType, note);
  const social = useSocialWindow(workspace, conversation, channelType, note);
  const text = useSend({
    mode,
    draft,
    files,
    held,
    onSend,
    onTyping,
    onSent: (text, then) => {
      gifs.sent(text);
      void macro.apply(then);
      onSent?.();
    },
  });
  const template = useTemplateReply(workspace, conversation, whatsapp, onSent);
  const out = outgoing(template, whatsapp, text);

  return (
    <Shell
      label={note ? "Note composer" : "Reply composer"}
      tab={
        <SegmentedControl
          aria-label="Write"
          variant="plain"
          selectedKeys={[mode]}
          onSelectionChange={(selected) => {
            const next = [...selected][0];
            if (next === modes.reply || next === modes.note) {
              draft.flush();
              onModeChange(next);
            }
          }}
          className="[&_button]:py-0.5 [&_button]:text-body-2-medium"
        >
          <SegmentedControlItem id={modes.reply}>Reply</SegmentedControlItem>
          <SegmentedControlItem id={modes.note}>Note</SegmentedControlItem>
        </SegmentedControl>
      }
      aside={
        <>
          {note ? <RiLock2Line aria-hidden className="size-3.5 shrink-0" /> : null}
          <span className="truncate">{note ? "Only your team sees this" : `To ${customer} · ${channel}`}</span>
          <WindowChip state={whatsapp} />
          <SocialChip state={social} />
        </>
      }
    >
      <ClosedWindow customer={customer} state={whatsapp} reply={template} />
      <SocialNotice customer={customer} state={social} />
      <TemplateCard reply={template} customer={customer} />
      <div className={template.picked ? "hidden" : "flex flex-col gap-3 px-1.5"}>
        <DraftConflict draft={draft} />
        <ComposerField
          value={draft.body}
          label={note ? "Internal note" : `Reply to ${customer}`}
          placeholder={note ? "Write a note only your team sees…" : `Write your reply to ${customer}…`}
          focusKey={`${mode}-${focusKey}`}
          onChange={(value) => {
            draft.edit(value);
            if (!note) {
              onTyping(value.trim() !== "");
            }
          }}
          onSubmit={out.send}
          onBlur={() => onTyping(false)}
          onFiles={(picked) => files.add(picked, attachable)}
          findMentions={note ? (query) => mentionables(workspace, conversation, query).then(toMentionables) : undefined}
          findLibrary={(query) => searchLibrary(client, workspace, query)}
          findSavedReplies={(query) =>
            client.fetchQuery(savedRepliesQuery(workspace, query)).then((found) => found.slice(0, 8))
          }
          takeSavedReply={macro.take}
          findTemplates={template.find}
          takeTemplate={template.pick}
          toolbarSlot={slot}
        />
      </div>
      <ReplyFileChips files={files} shown={attachable} />
      <QueuedMacro workspace={workspace} macro={macro} />
      {out.failure ? (
        <p role="alert" className="px-1.5 text-body-2-regular text-text-error-primary">
          {out.failure}
        </p>
      ) : null}
      <div className="flex items-center gap-2">
        <AddMenu
          onInsert={insert}
          onEmoji={addEmoji}
          onSavedReply={(reply) => insert(macro.take(reply))}
          gifs={gifs.gifs}
        />
        <span ref={setSlot} className="contents" />
        <AttachButton onPick={files.add} shown={attachable} />
        <button
          type="button"
          onClick={onAskOperator}
          className="flex h-[30px] shrink-0 cursor-pointer items-center gap-1 rounded-full ps-2 pe-2.5 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiSparkling2Line aria-hidden className="size-4" />
          Ask Operator
        </button>
        <span className="ms-auto hidden min-w-0 truncate text-body-2-regular text-text-tertiary sm:inline">
          {note ? "Only your team sees notes" : `Sends on ${channel}`}
        </span>
        <button
          type="button"
          disabled={out.disabled}
          onClick={() => out.send()}
          aria-label={note ? "Add note" : "Send"}
          className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-button-primary p-2 text-text-white outline-none transition-opacity duration-200 ease focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed disabled:opacity-40"
        >
          <RiArrowUpLine aria-hidden className="size-5" />
        </button>
        {note ? null : <SendMenu disabled={out.disabled} onSend={out.send} />}
      </div>
    </Shell>
  );
}

/*
 * Shell is the composer's frame: the tab strip behind its top edge, with
 * what is being written on one side and who sees it on the other, over
 * the panel that holds the field and its toolbar.
 */
function Shell({
  label,
  tab,
  aside,
  children,
}: {
  label: string;
  tab: ReactNode;
  aside: ReactNode;
  children: ReactNode;
}) {
  return (
    <div>
      <div className="mx-7 flex h-[34px] items-center justify-between gap-3 rounded-t-2xl bg-composer-panel-tab-background px-1 py-1">
        {tab}
        <span className="flex min-w-0 items-center gap-1 pe-1.5 text-body-2-medium text-text-secondary">{aside}</span>
      </div>
      <fieldset
        aria-label={label}
        className={cx(surface, "m-0 flex min-w-0 flex-col gap-5 border-0 p-2.5 pt-4 transition-shadow duration-150")}
      >
        {children}
      </fieldset>
    </div>
  );
}

/*
 * ReplySide is the reply half of a small composer: what sends a reply,
 * who it goes to and what the empty field says.
 */
export interface ReplySide {
  onReply: (text: string) => Promise<unknown>;
  to: string;
  placeholder: string;
}

/*
 * NoteComposer is the composer for something outside a conversation: a
 * ticket with no conversation of its own, which takes notes only, or a
 * feature request, which also takes replies when reply is given and
 * then opens on its Reply tab. It keeps the same field, add menu and
 * send button; what is typed stays until it has been taken.
 */
export function NoteComposer({ onAdd, reply }: { onAdd: (text: string) => Promise<unknown>; reply?: ReplySide }) {
  const { workspace, userId } = useInbox();
  const client = useQueryClient();
  const [body, setBody] = useState("");
  const [mode, setMode] = useState<Mode>(reply ? modes.reply : modes.note);
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [round, setRound] = useState(0);
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  const insert = (text: string) => setBody((was) => (was ? `${was}\n\n${text}` : text));
  const gifs = useGifs(workspace, userId, insert);
  const ready = body.trim() !== "" && !sending;
  const replying = reply !== undefined && mode === modes.reply;
  useEffect(preloadEditor, []);

  const add = async () => {
    if (!ready) {
      return;
    }
    setSending(true);
    setFailure(null);
    try {
      await (replying ? reply.onReply(body.trim()) : onAdd(body.trim()));
      setBody("");
      setRound((was) => was + 1);
    } catch (error) {
      const fallback = replying ? "Sending the reply failed. Try again." : "Adding the note failed. Try again.";
      setFailure(error instanceof Error ? error.message : fallback);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="mx-auto w-full max-w-3xl">
      <Shell
        label={replying ? "Reply composer" : "Note composer"}
        tab={
          reply ? (
            <SegmentedControl
              aria-label="Write"
              variant="plain"
              selectedKeys={[mode]}
              onSelectionChange={(selected) => {
                const next = [...selected][0];
                if (next === modes.reply || next === modes.note) {
                  setMode(next);
                }
              }}
              className="[&_button]:py-0.5 [&_button]:text-body-2-medium"
            >
              <SegmentedControlItem id={modes.reply}>Reply</SegmentedControlItem>
              <SegmentedControlItem id={modes.note}>Note</SegmentedControlItem>
            </SegmentedControl>
          ) : (
            <span className="px-2.5 text-body-2-medium text-text-primary">Note</span>
          )
        }
        aside={
          replying ? (
            <span className="truncate">{reply.to}</span>
          ) : (
            <>
              <RiLock2Line aria-hidden className="size-3.5 shrink-0" />
              <span className="truncate">Only your team sees this</span>
            </>
          )
        }
      >
        <div className="flex flex-col gap-3 px-1.5">
          <ComposerField
            value={body}
            label={replying ? "Reply" : "Internal note"}
            placeholder={replying ? reply.placeholder : "Write a note only your team sees…"}
            focusKey={`${mode}-${round}`}
            onChange={setBody}
            onSubmit={() => void add()}
            onBlur={noop}
            onFiles={noop}
            findLibrary={(query) => searchLibrary(client, workspace, query)}
            findSavedReplies={(query) =>
              client.fetchQuery(savedRepliesQuery(workspace, query)).then((found) => found.slice(0, 8))
            }
            takeSavedReply={(reply) => reply.body}
            toolbarSlot={slot}
          />
        </div>
        <div className="flex items-center gap-2">
          <AddMenu
            onInsert={insert}
            onEmoji={(emoji) => setBody((was) => `${was}${emoji}`)}
            onSavedReply={(reply) => insert(reply.body)}
            gifs={gifs.gifs}
          />
          <span ref={setSlot} className="contents" />
          <span
            role={failure ? "alert" : undefined}
            className={cx(
              "ms-auto min-w-0 truncate text-body-2-regular",
              failure ? "text-text-error-primary" : "hidden text-text-tertiary sm:inline",
            )}
          >
            {failure ?? (replying ? "" : "Only your team sees notes")}
          </span>
          <button
            type="button"
            disabled={!ready}
            onClick={() => void add()}
            aria-label={replying ? "Send reply" : "Add note"}
            className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-button-primary p-2 text-text-white outline-none transition-opacity duration-200 ease focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed disabled:opacity-40"
          >
            <RiArrowUpLine aria-hidden className="size-5" />
          </button>
        </div>
      </Shell>
    </div>
  );
}

function noop() {
  return undefined;
}
