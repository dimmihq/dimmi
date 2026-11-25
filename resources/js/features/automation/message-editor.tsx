/*
 * A workflow's message, written in the inbox's reply editor: the same
 * formatting, emoji after a colon, saved replies after a hash, and the
 * add menu for help articles, feature requests, changelog entries,
 * saved replies, emoji and GIFs; an at sign searches all of them, and a
 * colon finds emoji. An open brace offers the details the
 * workflow fills in for each customer, such as their first name, and
 * keeps each as a chip. A note names teammates after an at sign, as a
 * note in the inbox does. What is written is kept as Markdown, with the
 * variables in the {{ key }} form the server fills in.
 */

import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { AddMenu } from "@/features/inbox/add-menu";
import { ComposerField } from "@/features/inbox/composer/composer-field";
import { linkMarkdown } from "@/features/inbox/composer/link-token";
import type { Mentionable } from "@/features/inbox/composer/mentionable";
import { useGifs } from "@/features/inbox/composer/use-gifs";
import { useInbox } from "@/features/inbox/conversations";
import { searchLibrary } from "@/features/inbox/library";
import { savedRepliesQuery } from "@/lib/api/saved-replies";

import { variablesFor } from "./message-variables";
import type { TriggerKind } from "./triggers";

export function MessageEditor({
  label,
  value,
  trigger,
  replyTime = false,
  placeholder = "Write the message, or type { for a variable",
  teammates,
  onChange,
}: {
  label: string;
  value: string;
  trigger: TriggerKind;
  replyTime?: boolean;
  placeholder?: string;
  /* For a note: the teammates an at sign can name, by id and name. */
  teammates?: ReadonlyArray<readonly [string, string]>;
  onChange: (markdown: string) => void;
}) {
  const { workspace, userId } = useInbox();
  const queryClient = useQueryClient();
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  const insert = (text: string) => onChange(value ? `${value}\n\n${text}` : text);
  const gifs = useGifs(workspace, userId, insert);

  return (
    <div className="py-2.5 pe-2.5">
      <div className="flex flex-col gap-2 rounded-xl bg-background-secondary-default p-3 focus-within:ring-2 focus-within:ring-border-focus-ring">
        <ComposerField
          value={value}
          label={label}
          placeholder={placeholder}
          focusKey=""
          autoFocus={false}
          variables={variablesFor(trigger, replyTime)}
          onChange={onChange}
          onSubmit={() => undefined}
          onBlur={() => undefined}
          onFiles={() => undefined}
          findMentions={teammates ? async (query) => findTeammates(teammates, query) : undefined}
          findLibrary={(query) => searchLibrary(queryClient, workspace, query)}
          findSavedReplies={(query) => queryClient.fetchQuery(savedRepliesQuery(workspace, query))}
          takeSavedReply={(reply) => reply.body}
          toolbarSlot={slot}
        />
        <div className="flex items-center gap-1">
          <AddMenu
            onInsert={insert}
            onEmoji={(emoji) => onChange(`${value}${emoji}`)}
            onSavedReply={(reply) => insert(reply.body)}
            onLink={(title, url) =>
              onChange(`${value.trimEnd()}${value.trim() ? " " : ""}${linkMarkdown(title, url)} `)
            }
            gifs={gifs.gifs}
          />
          <span ref={setSlot} className="contents" />
        </div>
      </div>
    </div>
  );
}

/*
 * findTeammates narrows the teammates to those whose name holds what
 * was typed after the at sign.
 */
function findTeammates(teammates: ReadonlyArray<readonly [string, string]>, query: string): Mentionable[] {
  const needle = query.trim().toLowerCase();

  return teammates
    .filter(([, name]) => name.toLowerCase().includes(needle))
    .slice(0, 8)
    .map(([id, name]) => ({ kind: "member" as const, id, name, detail: "", warning: null }));
}
