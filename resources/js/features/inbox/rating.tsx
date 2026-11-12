/*
 * How the customer rated the conversation once it closed: a chip with
 * the face and word in the conversation's header, and in the details
 * panel the face, the word, the teammate it rated and the customer's
 * comment. Neither shows anything until the customer answered.
 */
import { useQuery } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { type ConversationRating, conversationRatingsQuery } from "@/lib/api/ratings";

import { when } from "./customer/format";

const tones: Record<number, "rose" | "orange" | "neutral" | "lime"> = {
  1: "rose",
  2: "orange",
  3: "neutral",
  4: "lime",
  5: "lime",
};

function useLatest(workspace: string, conversation: string): ConversationRating | undefined {
  const { data } = useQuery(conversationRatingsQuery(workspace, conversation));

  return data?.[0];
}

export function RatingChip({ workspace, conversation }: { workspace: string; conversation: string }) {
  const rating = useLatest(workspace, conversation);
  if (!rating) {
    return null;
  }

  return (
    <Chip
      variant="caption"
      color={tones[rating.score] ?? "neutral"}
      className="shrink-0 gap-0.5"
      title={`Rated ${rating.label}`}
    >
      <span aria-hidden>{rating.face}</span>
      <span>{rating.label}</span>
    </Chip>
  );
}

export function RatingSection({ workspace, conversation }: { workspace: string; conversation: string }) {
  const rating = useLatest(workspace, conversation);
  if (!rating) {
    return null;
  }

  return (
    <section aria-label="Rating" className="flex items-start gap-3 rounded-lg py-2">
      <span aria-hidden className="text-title-2-medium leading-none">
        {rating.face}
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="text-body-2-medium text-text-primary">
          Rated {rating.label}
          {rating.member ? <span className="text-text-secondary"> · {rating.member.name}</span> : null}
        </span>
        {rating.comment ? (
          <span className="whitespace-pre-wrap text-body-2-regular text-text-secondary">“{rating.comment}”</span>
        ) : null}
        {rating.rated_at ? (
          <span className="text-caption-1-regular text-text-tertiary">
            {rating.channel === "email" ? "By email" : "In the messenger"}, {when(rating.rated_at)}
          </span>
        ) : null}
      </span>
    </section>
  );
}
