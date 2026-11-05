/*
 * A quiet warning that the customer may already be known under another
 * record, naming the strongest match. Review opens the customer's page,
 * where the team compares the records and merges them.
 */
import { RiUserSearchLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { duplicatesQuery } from "@/lib/api/customers";

export function DuplicateBanner({ workspace, contact, count }: { workspace: string; contact: string; count: number }) {
  const { data: duplicates = [] } = useQuery({ ...duplicatesQuery(workspace, contact), enabled: count > 0 });
  const first = duplicates[0]?.contact;

  if (count === 0 || !first) {
    return null;
  }

  return (
    <div
      role="status"
      className="flex items-center gap-2 rounded-xl bg-background-secondary-default px-3 py-2 text-body-2-regular text-text-primary"
    >
      <RiUserSearchLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
      <span className="min-w-0 flex-1 truncate">
        Possible duplicate: {first.name ?? first.email ?? "another customer"}
      </span>
      <Link
        to="/d/customers/$contact"
        params={{ contact }}
        search={{ duplicates: true }}
        className="shrink-0 rounded-md text-body-2-medium text-text-primary underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        Review
      </Link>
    </div>
  );
}
