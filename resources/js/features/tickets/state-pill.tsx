/*
 * The small marks a ticket wears wherever it is listed: its state as a
 * pill in its group's colour, its type's icon, and its reference beside
 * them, such as T-482.
 */
import { Chip } from "@/components/base/badges/chip";
import type { StateGroup, StateRef, TicketKind } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { groupColor, groupLabel, typeIcon } from "./labels";

export function StatePill({ state, className }: { state: Pick<StateRef, "name" | "group">; className?: string }) {
  return (
    <Chip variant="caption" color={groupColor[state.group]} className={cx("shrink-0", className)}>
      {state.name}
    </Chip>
  );
}

/*
 * GroupPill shows a group by name, for filters and columns.
 */
export function GroupPill({ group }: { group: StateGroup }) {
  return (
    <Chip variant="caption" color={groupColor[group]} className="shrink-0">
      {groupLabel(group)}
    </Chip>
  );
}

export function TypeIcon({ icon, kind, className }: { icon: string; kind: TicketKind; className?: string }) {
  const Icon = typeIcon(icon, kind);

  return <Icon aria-hidden className={cx("size-4 shrink-0 text-foreground-icon-secondary", className)} />;
}

/*
 * TicketMark is the reference, type icon and state pill together, as a
 * conversation row shows them.
 */
export function TicketMark({
  reference,
  type,
  state,
}: {
  reference: string;
  type: { icon: string; kind: TicketKind; name: string };
  state: Pick<StateRef, "name" | "group">;
}) {
  return (
    <span className="flex min-w-0 items-center gap-1.5" title={`${reference} · ${type.name} · ${state.name}`}>
      <TypeIcon icon={type.icon} kind={type.kind} className="size-3.5" />
      <span className="shrink-0 text-caption-1-medium text-text-secondary">{reference}</span>
      <StatePill state={state} />
    </span>
  );
}
