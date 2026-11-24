/*
 * Everything Operator proposed that still waits for a teammate, from
 * routines and from chats alike, newest first. Each card says what
 * would change and why, and is applied or declined here.
 */
import { RiCheckboxCircleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Glyph, Item, RowsSkeleton, Section } from "@/features/settings/layout";
import { pendingChangesetsQuery } from "@/lib/api/operator";

import { ChangesetCard } from "./changeset-card";
import { useSuggestions } from "./suggestions";

const every = () => true;

export function OperatorSuggestions({ workspace }: { workspace: string }) {
  const { isPending } = useQuery({ ...pendingChangesetsQuery(workspace), enabled: workspace !== "" });
  const { suggestions, decide } = useSuggestions(workspace, every);

  if (isPending) {
    return (
      <Section title="Waiting for review">
        <RowsSkeleton rows={3} />
      </Section>
    );
  }

  if (suggestions.length === 0) {
    return (
      <Section title="Waiting for review">
        <Item
          mark={<Glyph icon={RiCheckboxCircleLine} />}
          title="Nothing waiting for review"
          detail="Changes Operator proposes in a routine or a chat wait here until a teammate decides."
        >
          <Link
            to="/d/operator/$tab"
            params={{ tab: "routines" }}
            className="rounded-md pe-3 text-body-2-medium text-text-secondary outline-none hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            See routines
          </Link>
        </Item>
      </Section>
    );
  }

  return (
    <Section
      title="Waiting for review"
      description="Nothing here has changed anything yet. Apply a suggestion to make the change, or decline it."
      bare
    >
      <ul className="flex flex-col gap-2">
        {suggestions.map((changeset) => (
          <li key={changeset.id}>
            <ChangesetCard changeset={changeset} onDecide={decide} />
          </li>
        ))}
      </ul>
    </Section>
  );
}
