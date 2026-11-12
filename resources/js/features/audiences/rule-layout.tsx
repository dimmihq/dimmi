/*
 * The look every conditions builder shares, whatever it filters: groups
 * as cards that match all, any or none of their conditions, each
 * condition one row of fixed columns (field, comparison, value, remove)
 * with a word in the gutter saying how it joins the row above, and a
 * pill between cards saying how they join. Audiences and saved inbox
 * views lay their own fields into these parts.
 */
import { RiAddLine, RiCloseLine, RiDeleteBinLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { cx } from "@/utils/cx";

import type { Match } from "./rules";

const matchLabels: Record<Match, string> = { all: "All", any: "Any", none: "None" };

export function MatchPick({
  label,
  value,
  onChange,
  choices = ["all", "any", "none"],
  disabled = false,
}: {
  label: string;
  value: Match;
  onChange: (m: Match) => void;
  choices?: Match[];
  disabled?: boolean;
}) {
  return (
    <SegmentedControl
      aria-label={label}
      isDisabled={disabled}
      selectedKeys={new Set([value])}
      onSelectionChange={(keys) => {
        const next = [...keys][0];
        if (next) {
          onChange(next as Match);
        }
      }}
    >
      {choices.map((m) => (
        <SegmentedControlItem key={m} id={m}>
          {matchLabels[m]}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

/*
 * RuleRow is one condition in its fixed columns, with anything that
 * belongs under it, such as the days an event count covers.
 */
export function RuleRow({
  join,
  field,
  op,
  value,
  below,
  flagged = false,
  onRemove,
}: {
  join: string;
  field: ReactNode;
  op: ReactNode;
  value: ReactNode;
  below?: ReactNode;
  flagged?: boolean;
  onRemove: () => void;
}) {
  return (
    <li
      className={cx(
        "grid grid-cols-[2.75rem_minmax(0,1.15fr)_minmax(0,0.95fr)_minmax(0,1fr)_2rem] items-start gap-2",
        flagged && "rounded-xl ring-1 ring-border-error-default ring-offset-4",
      )}
    >
      <span className="flex h-10 items-center text-body-2-medium text-text-tertiary">{join}</span>
      {field}
      {op}
      <div className="flex min-w-0 flex-col gap-2">{value}</div>
      <span className="flex h-10 items-center">
        <IconButton icon={RiCloseLine} size="small" aria-label="Remove condition" onClick={onRemove} />
      </span>
      {below ? <div className="col-span-3 col-start-2">{below}</div> : null}
    </li>
  );
}

/*
 * RuleCard is one group: how its conditions join, its rows, and a way
 * to add one.
 */
export function RuleCard({
  index,
  match,
  choices,
  only,
  onMatch,
  onAdd,
  onRemove,
  children,
}: {
  index: number;
  match: Match;
  choices?: Match[];
  only: boolean;
  onMatch: (match: Match) => void;
  onAdd: () => void;
  onRemove: () => void;
  children: ReactNode;
}) {
  return (
    <section
      aria-label={`Group ${index + 1}`}
      className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-secondary-default p-4"
    >
      <header className="flex flex-wrap items-center gap-3">
        <span className="text-body-medium text-text-primary">Group {index + 1}</span>
        <span className="text-body-2-regular text-text-secondary">matches</span>
        <MatchPick label={`How group ${index + 1} matches`} value={match} choices={choices} onChange={onMatch} />
        <span className="text-body-2-regular text-text-secondary">of these</span>
        <span className="ms-auto">
          <IconButton
            icon={RiDeleteBinLine}
            size="small"
            aria-label={`Remove group ${index + 1}`}
            disabled={only}
            onClick={onRemove}
          />
        </span>
      </header>
      <ul className="flex flex-col gap-2">{children}</ul>
      <div className="ps-14">
        <LinkButton variant="secondary" leadingIcon={RiAddLine} onClick={onAdd}>
          Add condition
        </LinkButton>
      </div>
    </section>
  );
}

/*
 * CardJoin is the pill between two cards saying how they join.
 */
export function CardJoin({ match }: { match: Match }) {
  return (
    <span className="flex h-8 items-center justify-center">
      <span className="rounded-full bg-background-tertiary-default px-2.5 py-0.5 text-caption-1-medium text-text-secondary">
        {match === "all" ? "and" : "or"}
      </span>
    </span>
  );
}
