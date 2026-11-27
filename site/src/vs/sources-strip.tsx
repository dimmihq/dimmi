/*
 * Attribution and verification bar for /vs pages: author, reviewer,
 * visible updated date (<time dateTime>), and rival pricing check link.
 */
import { RiExternalLinkLine } from "@remixicon/react";

import { getAuthor } from "../data/authors";
import type { Rival } from "../data/rivals";
import { formatIsoDate } from "./rival-tone";

export function SourcesStrip({
  rival,
  authorId,
  reviewedById,
  published,
  updated,
}: {
  rival: Pick<Rival, "name" | "pricingUrl" | "checkedOn">;
  authorId: string;
  reviewedById?: string;
  published: string;
  updated: string;
}) {
  const author = getAuthor(authorId);
  const reviewer = reviewedById ? getAuthor(reviewedById) : undefined;

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 border-t border-separator-border pt-4 font-mono text-[12px] leading-5 text-text-tertiary">
      <span>
        By{" "}
        <a href={author.url} className="text-text-secondary underline-offset-4 hover:text-text-primary hover:underline">
          {author.name}
        </a>
      </span>
      {reviewer ? <span>Reviewed by {reviewer.name}</span> : null}
      <span>
        Updated <time dateTime={updated}>{formatIsoDate(updated)}</time>
        {published !== updated ? (
          <>
            {" "}
            (published <time dateTime={published}>{formatIsoDate(published)}</time>)
          </>
        ) : null}
      </span>
      <a
        href={rival.pricingUrl}
        rel="noopener noreferrer"
        target="_blank"
        className="inline-flex items-center gap-1 text-text-secondary underline-offset-4 hover:text-text-primary hover:underline"
      >
        {rival.name} pricing checked {formatIsoDate(rival.checkedOn)}
        <RiExternalLinkLine className="size-3.5 shrink-0" aria-hidden />
      </a>
    </div>
  );
}
