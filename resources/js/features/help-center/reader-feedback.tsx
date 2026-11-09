/*
 * What readers said about an article, in a pane beside the writer: how
 * many found it helpful, and the notes left by those who did not, so the
 * team sees what was missing while they fix it. A note from a signed-in
 * reader names them; others are anonymous.
 */
import { RiChatQuoteLine, RiCloseLine, RiThumbDownLine, RiThumbUpLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Panel, ScrollArea } from "@/features/shell/page";
import { ListSkeleton } from "@/features/shell/table-skeleton";
import { type ArticleFeedback, articleFeedbackQuery } from "@/lib/api/help-centers";
import { formatDateTime } from "@/lib/i18n/format";

/*
 * share is the part of all answers that found the article helpful, as a
 * whole percentage, or null before anyone answered.
 */
function share(f: ArticleFeedback): number | null {
  const total = f.helpful + f.unhelpful;
  return total === 0 ? null : Math.round((f.helpful / total) * 100);
}

function Counts({ feedback }: { feedback: ArticleFeedback }) {
  const helpful = share(feedback);

  return (
    <div className="grid grid-cols-2 gap-2">
      <div className="rounded-2xl border border-border-button-default bg-background-secondary-default p-3">
        <p className="flex items-center gap-1.5 text-caption-1-medium text-text-tertiary">
          <RiThumbUpLine aria-hidden className="size-3.5" />
          Helpful
        </p>
        <p className="mt-1 text-title-3-semibold text-text-primary tabular-nums">
          {helpful === null ? "—" : `${helpful}%`}
        </p>
        <p className="text-caption-1-regular text-text-tertiary tabular-nums">
          {feedback.helpful} yes · {feedback.unhelpful} no
        </p>
      </div>
      <div className="rounded-2xl border border-border-button-default bg-background-secondary-default p-3">
        <p className="flex items-center gap-1.5 text-caption-1-medium text-text-tertiary">
          <RiChatQuoteLine aria-hidden className="size-3.5" />
          Notes
        </p>
        <p className="mt-1 text-title-3-semibold text-text-primary tabular-nums">{feedback.comments}</p>
        <p className="text-caption-1-regular text-text-tertiary">left after a no</p>
      </div>
    </div>
  );
}

export function ReaderFeedback({
  workspace,
  article,
  onClose,
  className,
}: {
  workspace: string;
  article: string;
  onClose: () => void;
  className?: string;
}) {
  const { data, isPending } = useQuery({ ...articleFeedbackQuery(workspace, article), enabled: workspace !== "" });

  return (
    <Panel label="Reader feedback" className={className}>
      <header className="flex h-12 shrink-0 items-center justify-between gap-2 ps-4 pe-2.5 pt-[7px]">
        <span className="flex min-w-0 items-center gap-2">
          <RiChatQuoteLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
          <h2 className="truncate text-body-medium text-text-primary">Reader feedback</h2>
        </span>
        <IconButton size="small" icon={RiCloseLine} aria-label="Close reader feedback" onClick={onClose} />
      </header>
      <ScrollArea>
        <div className="flex flex-col gap-3 px-3 pt-2 pb-6">
          {isPending ? <ListSkeleton rows={3} /> : null}
          {data ? <Counts feedback={data} /> : null}
          {data && data.recent.length === 0 ? (
            <p className="px-1 text-body-2-regular text-text-secondary">
              No notes yet. Readers who say the article did not help can tell you what was missing.
            </p>
          ) : null}
          <ol aria-label="Notes from readers" className="flex flex-col gap-2">
            {(data?.recent ?? []).map((note) => (
              <li
                key={`${note.created_at}-${note.comment}`}
                className="flex flex-col gap-1.5 rounded-2xl border border-border-button-default bg-background-secondary-default p-3"
              >
                <p className="flex items-center gap-1.5 text-caption-1-medium text-text-tertiary">
                  <RiThumbDownLine aria-hidden className="size-3.5" />
                  <span className="min-w-0 flex-1 truncate">
                    {note.contact ? (note.contact.name ?? note.contact.email ?? "A customer") : "Anonymous reader"}
                  </span>
                  <time dateTime={note.created_at}>{formatDateTime(note.created_at)}</time>
                </p>
                <p className="whitespace-pre-line text-body-2-regular text-text-primary">{note.comment}</p>
              </li>
            ))}
          </ol>
        </div>
      </ScrollArea>
    </Panel>
  );
}
