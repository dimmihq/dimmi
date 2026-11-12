/*
 * What the customer edit modal shows without editing: the people who
 * look like the same person, each with a way to merge them in, and what
 * the person did in the team's product, as its events report.
 */
import { RiMergeCellsHorizontal } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";

import { when } from "@/features/inbox/customer/format";
import { type Contact, duplicatesQuery } from "@/lib/api/customers";
import { contactEventsQuery } from "@/lib/api/events";

import { duplicateReasons } from "./duplicate-reasons";
import { MergeContactDialog } from "./merge-customer";
import { personName, useWorkspace } from "./names";

/*
 * Duplicates lists the people who look like this one, each opening in
 * the same modal, with why they look alike, and merges one in when the
 * teammate may change customers.
 */
export function Duplicates({ contact, canMerge }: { contact: Contact; canMerge: boolean }) {
  const workspace = useWorkspace();
  const { data: duplicates = [] } = useQuery(duplicatesQuery(workspace, contact.id));
  const [merging, setMerging] = useState<string | null>(null);
  if (duplicates.length === 0) {
    return null;
  }

  return (
    <section
      aria-label="Possible duplicates"
      className="flex flex-col gap-1 rounded-2xl border border-border-button-default bg-background-secondary-default p-3"
    >
      <h3 className="text-body-medium text-text-primary">Possible duplicates</h3>
      {duplicates.map((d) => (
        <p key={d.contact.id} className="flex min-w-0 items-center gap-2 text-body-2-regular">
          <Link
            to="."
            search={{ contact: d.contact.id }}
            className="min-w-0 truncate rounded-md text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            {personName(d.contact)}
          </Link>
          <span className="min-w-0 flex-1 truncate text-text-tertiary">
            {d.reasons.map((r) => duplicateReasons[r] ?? r).join(", ")}
          </span>
          {canMerge ? (
            <Button
              size="xs"
              variant="ghost"
              leadingIcon={RiMergeCellsHorizontal}
              aria-label={`Merge ${personName(d.contact)} into this customer`}
              onClick={() => setMerging(d.contact.id)}
            >
              Merge
            </Button>
          ) : null}
        </p>
      ))}
      {merging ? <MergeContactDialog contact={contact} preselected={merging} onClose={() => setMerging(null)} /> : null}
    </section>
  );
}

/*
 * Activity is each event the person sent: its total, its count in the
 * last 7 days and when it last happened.
 */
export function Activity({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const { data } = useQuery({ ...contactEventsQuery(workspace, contact.id), enabled: workspace !== "" });
  if (!data || data.length === 0) {
    return null;
  }

  return (
    <section aria-label="Activity" className="flex flex-col gap-3">
      <h3 className="text-body-medium text-text-primary">Activity</h3>
      <ul className="flex flex-col gap-2">
        {data.map((e) => (
          <li key={e.name} className="flex min-w-0 flex-col">
            <span className="truncate text-body-2-medium text-text-primary">{e.label}</span>
            <span className="text-body-2-regular text-text-secondary">
              {e.count === 1 ? "Once" : `${e.count} times`}, {e.last_7_days} in the last 7 days, last {when(e.last_at)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
