/*
 * The line above an article under review: who asks for the change and
 * why, and how many more requests wait after it.
 */
import { RiSparkling2Line } from "@remixicon/react";

import type { ArticleRequest } from "./change-requests";

/*
 * RequestNote says, above the article, who asked for the change and why.
 */
export function RequestNote({ request, waiting }: { request: ArticleRequest | null; waiting: number }) {
  if (!request) {
    return null;
  }
  return (
    <div className="mx-auto mt-6 flex w-full max-w-[42.5rem] items-start gap-2.5 px-6">
      <RiSparkling2Line aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" />
      <p className="min-w-0 text-body-2-regular text-text-secondary">
        <span className="text-body-2-medium text-text-primary">Operator requests a change: </span>
        {request.changeset.reason || request.changeset.title}
        {request.fresh ? null : (
          <span className="text-text-tertiary">
            {" "}
            Struck-through lines go, green lines come in.
            {waiting > 1 ? ` ${waiting - 1} more request${waiting > 2 ? "s" : ""} after this one.` : ""}
          </span>
        )}
      </p>
    </div>
  );
}
